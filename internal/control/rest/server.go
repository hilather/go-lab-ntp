package rest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hilather/go-lab-ntp/internal/app"
	"github.com/hilather/go-lab-ntp/internal/auth"
	"github.com/hilather/go-lab-ntp/internal/capabilities"
	"github.com/hilather/go-lab-ntp/internal/config"
	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/model"
	"github.com/hilather/go-lab-ntp/internal/observability"
)

const (
	DefaultAddr              = config.DefaultMgmtAddress
	DefaultMaxBodyBytes      = 1 << 20
	DefaultRequestTimeout    = 30 * time.Second
	DefaultReadHeaderTimeout = 5 * time.Second
	DefaultReadTimeout       = 30 * time.Second
	DefaultMaxConcurrent     = 256
	headerRequestID          = "X-Request-ID"
	headerIdempotency        = "Idempotency-Key"
	headerIfMatch            = "If-Match"
	headerExpected           = "X-LabNTP-Expected-Revision"
	headerRevision           = "X-LabNTP-Revision"
	headerAllow              = "Allow"
	requestURNPrefix         = "urn:labntp:request:"
)

// rebindDrainTimeout bounds the background drain of the server Rebind
// replaced. Rebind does not wait for it, and a timeout is not an error.
const rebindDrainTimeout = 5 * time.Second

// Config constructs a management HTTP server.
type Config struct {
	Addr              string
	Service           app.Service
	AllowedOrigins    []string
	Live              func() bool
	Ready             func() bool
	MaxBodyBytes      int64
	RequestTimeout    time.Duration
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	MaxConcurrent     int
	RatePerSec        float64
	RateBurst         float64
	PublicMetrics     bool
	Metrics           *observability.Registry
	Logger            *observability.Logger
	Auth              *auth.Verifier
	Sessions          *auth.Store
	CookieSecure      bool
	UI                http.Handler
	UIEnabled         func() bool
	Mounts            map[string]http.Handler
}

// Server is the stdlib net/http management listener.
type Server struct {
	cfg      Config
	svc      app.Service
	routes   []compiledRoute
	handler  http.Handler
	maxBody  atomic.Int64
	timeout  time.Duration
	inflight inflightGate
	rate     *limiter
	metrics  *observability.Registry
	logger   *observability.Logger
	mounts   *http.ServeMux

	limitsMu sync.Mutex
	mu       sync.Mutex
	http     *http.Server
	ln       net.Listener
	closed   atomic.Bool
	addr     string
	// drains are background rebind drains. Each channel is created in the
	// same s.mu section that detaches the old server, and closed when that
	// drain finishes, including Close after a timeout. A slice under mu,
	// not a WaitGroup: Add concurrent with Wait panics once the counter
	// has hit zero, and Rebind can start a drain while Shutdown is waiting.
	drains []chan struct{}
}

// New builds a Server. Routes come from the frozen capability registry.
func New(cfg Config) (*Server, error) {
	if cfg.Service == nil {
		return nil, errors.New("rest: Service is required")
	}
	maxBody := cfg.MaxBodyBytes
	if maxBody <= 0 {
		maxBody = DefaultMaxBodyBytes
	}
	timeout := cfg.RequestTimeout
	if timeout <= 0 {
		timeout = DefaultRequestTimeout
	}
	n := cfg.MaxConcurrent
	if n <= 0 {
		n = DefaultMaxConcurrent
	}
	if cfg.Sessions == nil {
		cfg.Sessions = auth.NewStore(auth.DefaultSessionConfig())
	}
	if cfg.Auth != nil {
		sessions := cfg.Sessions
		cfg.Auth.OnIdentityChange(func() {
			if sessions != nil {
				sessions.Clear()
			}
		})
	}
	s := &Server{
		cfg:     cfg,
		svc:     cfg.Service,
		routes:  compileRoutes(capabilities.All()),
		timeout: timeout,
		rate:    newLimiter(cfg.RatePerSec, cfg.RateBurst),
		metrics: cfg.Metrics,
		logger:  cfg.Logger,
		addr:    cfg.Addr,
	}
	s.maxBody.Store(maxBody)
	s.inflight.setMax(n)
	if appSvc, ok := s.svc.(*app.App); ok {
		appSvc.OnAuthPreflight(preflightAuth)
		appSvc.OnReset(s.reloadAuth)
		appSvc.OnApply(s.reloadAuth)
		appSvc.OnReset(s.syncManagementLimits)
		appSvc.OnApply(s.syncManagementLimits)
	}
	if len(cfg.Mounts) > 0 {
		mux := http.NewServeMux()
		for path, h := range cfg.Mounts {
			mux.Handle(path, h)
		}
		s.mounts = mux
	}
	s.handler = http.HandlerFunc(s.serveHTTP)
	return s, nil
}

// Handler returns the management mux. Safe for httptest.NewServer / ServeHTTP.
func (s *Server) Handler() http.Handler {
	return s.handler
}

// ListenAndServe binds Addr and serves until Shutdown.
func (s *Server) ListenAndServe() error {
	addr := s.cfg.Addr
	if addr == "" {
		addr = DefaultAddr
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return s.Serve(ln)
}

// Serve serves on ln until Shutdown.
func (s *Server) Serve(ln net.Listener) error {
	s.mu.Lock()
	if s.closed.Load() {
		s.mu.Unlock()
		_ = ln.Close()
		return nil
	}
	if s.http != nil {
		s.mu.Unlock()
		_ = ln.Close()
		return errors.New("rest: server already started")
	}
	hs := s.newHTTPServer()
	s.http = hs
	s.ln = ln
	s.addr = ln.Addr().String()
	alreadyClosed := s.closed.Load()
	s.mu.Unlock()
	if alreadyClosed {
		_ = ln.Close()
		return nil
	}
	return serveResult(hs.Serve(ln))
}

func (s *Server) newHTTPServer() *http.Server {
	rh := s.cfg.ReadHeaderTimeout
	if rh <= 0 {
		rh = DefaultReadHeaderTimeout
	}
	rt := s.cfg.ReadTimeout
	if rt <= 0 {
		rt = DefaultReadTimeout
	}
	return &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: rh,
		ReadTimeout:       rt,
		WriteTimeout:      s.cfg.WriteTimeout,
		MaxHeaderBytes:    1 << 16,
	}
}

// serveResult treats http.ErrServerClosed as a normal stop.
func serveResult(err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Rebind moves the management listener to addr, or unbinds when addr is empty.
// A non-nil error means net.Listen failed and the previous listener is untouched.
// The old listener is closed before Rebind returns. The old server drains in
// the background for up to rebindDrainTimeout; that drain is not an error.
func (s *Server) Rebind(addr string) error {
	if s == nil {
		return errors.New("rest: nil server")
	}
	s.mu.Lock()
	cur := s.addr
	bound := s.ln != nil && s.http != nil && !s.closed.Load()
	s.mu.Unlock()
	if addr != "" && addr == cur && bound {
		return nil
	}
	if addr == "" {
		s.mu.Lock()
		oldHS, oldLn := s.http, s.ln
		s.http = nil
		s.ln = nil
		s.addr = ""
		s.closed.Store(true)
		var done chan struct{}
		if oldHS != nil || oldLn != nil {
			done = make(chan struct{})
			s.drains = append(s.drains, done)
		}
		s.mu.Unlock()
		s.drainAsync(oldHS, oldLn, done)
		return nil
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	hs := s.newHTTPServer()
	s.mu.Lock()
	oldHS, oldLn := s.http, s.ln
	s.http = hs
	s.ln = ln
	s.addr = ln.Addr().String()
	s.closed.Store(false)
	var done chan struct{}
	if oldHS != nil || oldLn != nil {
		done = make(chan struct{})
		s.drains = append(s.drains, done)
	}
	s.mu.Unlock()
	s.drainAsync(oldHS, oldLn, done)
	go func() { _ = serveResult(hs.Serve(ln)) }()
	return nil
}

// drainAsync closes ln before it returns and drains hs in the background
// for up to rebindDrainTimeout. done was created under s.mu in the same
// critical section that detached hs and ln, so a Shutdown that runs after
// that section sees the drain. done is nil only when both are nil. A nil
// server with a live listener closes that listener and finishes done
// before returning.
//
// http.Server.Shutdown sets inShutdown and closes tracked listeners before
// its RegisterOnShutdown hooks run, so waiting for the hook and then closing
// ln refuses new connections without Serve returning "use of closed network
// connection". A timed-out drain calls Close. That still finishes done.
//
// net/http runs every onShutdown hook again on each later Shutdown of the
// same *http.Server. sync.Once keeps close(ready) from running twice.
func (s *Server) drainAsync(hs *http.Server, ln net.Listener, done chan struct{}) {
	if hs == nil && ln == nil {
		return
	}
	if hs == nil {
		_ = ln.Close()
		s.finishDrain(done)
		return
	}
	var once sync.Once
	ready := make(chan struct{})
	hs.RegisterOnShutdown(func() {
		once.Do(func() { close(ready) })
	})
	go func() {
		defer s.finishDrain(done)
		ctx, cancel := context.WithTimeout(context.Background(), rebindDrainTimeout)
		defer cancel()
		if err := hs.Shutdown(ctx); err != nil {
			_ = hs.Close()
		}
	}()
	<-ready
	if ln != nil {
		_ = ln.Close()
	}
}

// finishDrain drops done from the in-flight set, then closes it so a
// Shutdown that already copied the slice still wakes. Exactly once.
func (s *Server) finishDrain(done chan struct{}) {
	if done == nil {
		return
	}
	s.mu.Lock()
	for i, ch := range s.drains {
		if ch == done {
			s.drains = append(s.drains[:i], s.drains[i+1:]...)
			break
		}
	}
	s.mu.Unlock()
	close(done)
}

// Bound reports whether a listener is accepting.
func (s *Server) Bound() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ln != nil && s.http != nil && !s.closed.Load()
}

// Shutdown closes the listener and waits for in-flight requests.
// It also waits for background rebind drains already started, bounded by ctx.
// If ctx ends first, Shutdown returns ctx.Err() and does not wait out the drain.
// An error from the current server takes precedence over the drain wait.
func (s *Server) Shutdown(ctx context.Context) error {
	s.closed.Store(true)
	s.mu.Lock()
	hs := s.http
	ln := s.ln
	s.mu.Unlock()
	var err error
	if hs != nil {
		err = hs.Shutdown(ctx)
	} else if ln != nil {
		err = ln.Close()
	}
	if werr := s.waitDrains(ctx); err == nil {
		err = werr
	}
	return err
}

// waitDrains waits for background drains already started when it is called.
// It snapshots the set under s.mu and does not hold that lock while waiting.
// A drain that finishes before the snapshot is already gone. If ctx ends
// first, waitDrains returns ctx.Err().
func (s *Server) waitDrains(ctx context.Context) error {
	s.mu.Lock()
	pending := append([]chan struct{}(nil), s.drains...)
	s.mu.Unlock()
	for _, done := range pending {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-done:
		}
	}
	return nil
}

// Addr returns the bound address after Serve, or the configured listen address.
// After Rebind(""), Bound is false and Addr is empty. Shutdown leaves the
// listener in place, so Addr still returns the address that was bound.
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln != nil {
		return s.ln.Addr().String()
	}
	if s.addr != "" {
		return s.addr
	}
	if s.closed.Load() {
		return ""
	}
	if s.cfg.Addr != "" {
		return s.cfg.Addr
	}
	return DefaultAddr
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	sw := &statusWriter{ResponseWriter: w, code: http.StatusOK}
	w = sw
	reqID := requestID(r)
	w.Header().Set(headerRequestID, reqID)
	r.Header.Set(headerRequestID, reqID)
	instance := requestURNPrefix + reqID
	capID := ""
	defer func() {
		s.observeHTTP(capID, sw.status(), start, reqID)
	}()

	if err := checkOrigin(r.Header.Get("Origin"), s.cfg.AllowedOrigins); err != nil {
		s.writeProblem(w, r, instance, err)
		return
	}
	if r.Method == http.MethodOptions {
		s.writeProblem(w, r, instance, domainerr.Forbidden("CORS is disabled"))
		return
	}

	if !s.inflight.acquire() {
		s.writeProblem(w, r, instance, domainerr.RateLimited("too many concurrent management requests"))
		return
	}
	defer s.inflight.release()

	ctx := r.Context()
	var cancel context.CancelFunc
	if s.timeout > 0 && !s.isMountedPath(r) {
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}
	r = r.WithContext(ctx)

	defer func() {
		if rec := recover(); rec != nil {
			s.writeProblem(w, r, instance, domainerr.Internal("internal error"))
		}
	}()

	if s.dispatchMount(w, r, instance) {
		return
	}

	rt, params, pathOK, methodOK := matchRoute(s.routes, r.Method, r.URL.Path)
	if pathOK {
		capID = string(rt.cap.ID)
		if !methodOK {
			w.Header().Set(headerAllow, allowedMethods(s.routes, r.URL.Path))
			s.writeProblem(w, r, instance, domainerr.MethodNotAllowed("method not allowed"))
			return
		}
		if !isHealthCap(rt.cap) {
			if err := s.rate.allow(r.RemoteAddr); err != nil {
				s.writeProblem(w, r, instance, err)
				return
			}
		}
		actor, err := s.authenticate(r, isHealthCap(rt.cap))
		if err != nil {
			s.writeProblem(w, r, instance, err)
			return
		}
		if err := s.authorize(r, actor, rt.cap); err != nil {
			s.writeProblem(w, r, instance, err)
			return
		}
		s.dispatch(w, r, instance, actor, rt, params)
		return
	}

	if s.tryUI(w, r, instance) {
		return
	}
	s.writeProblem(w, r, instance, domainerr.NotFound("not found"))
}

// preflightAuth refuses a reset whose spec.auth the live verifier cannot load.
// RequireListen's plain error is validation_failed; asDomain would hide it
// as internal_error.
func preflightAuth(spec model.AuthSpec) error {
	next, err := auth.FromSpec(spec)
	if err != nil {
		return err
	}
	if err := next.RequireListen(); err != nil {
		return domainerr.ValidationFailed("management auth cannot be loaded",
			domainerr.FieldViolation{
				Path:    "spec.auth.tokens",
				Code:    "invalid_value",
				Message: err.Error(),
			})
	}
	return nil
}

func (s *Server) reloadAuth() {
	if s.cfg.Auth == nil {
		return
	}
	appSvc, ok := s.svc.(*app.App)
	if !ok {
		return
	}
	snap := appSvc.Active()
	if snap == nil || snap.Canonical == nil {
		return
	}
	next, err := auth.FromSpec(snap.Canonical.Spec.Auth)
	if err != nil {
		return
	}
	if err := next.RequireListen(); err != nil {
		return
	}
	changed := !s.cfg.Auth.Equivalent(next)
	s.cfg.Auth.Replace(next)
	if changed && s.cfg.Sessions != nil {
		s.cfg.Sessions.Clear()
	}
}

func isHealthCap(cap capabilities.Capability) bool {
	return cap.ID == capabilities.HealthLive || cap.ID == capabilities.HealthReady
}

func (s *Server) isMountedPath(r *http.Request) bool {
	if s == nil || s.mounts == nil || r == nil {
		return false
	}
	_, pattern := s.mounts.Handler(r)
	return pattern != ""
}

func (s *Server) dispatchMount(w http.ResponseWriter, r *http.Request, instance string) bool {
	if s.mounts == nil {
		return false
	}
	h, pattern := s.mounts.Handler(r)
	if pattern == "" {
		return false
	}
	if err := s.rate.allow(r.RemoteAddr); err != nil {
		s.writeProblem(w, r, instance, err)
		return true
	}
	// Same live ceiling REST JSON decode uses. The mounted MCP handler
	// still has its own startup MaxRequestBodyBytes, so a raise above
	// that ceiling does not take effect until restart.
	if r.Body != nil {
		r.Body = http.MaxBytesReader(w, r.Body, s.maxBody.Load())
	}
	h.ServeHTTP(w, r)
	return true
}

func (s *Server) isLive() bool {
	if s.cfg.Live != nil {
		return s.cfg.Live()
	}
	return true
}

func (s *Server) isReady(ctx context.Context) bool {
	if s.cfg.Ready != nil {
		return s.cfg.Ready()
	}
	st, err := s.svc.Status(ctx, app.Actor{ID: "probe", Class: "startup", Transport: "rest"})
	if err != nil {
		return false
	}
	return st.Ready
}

func (s *Server) observeHTTP(capID string, status int, start time.Time, reqID string) {
	if s.metrics != nil {
		s.metrics.Inc(observability.MetricHTTPRequestsTotal, map[string]string{
			"capability": capID,
			"code_class": observability.CodeClass(status),
		}, 1)
	}
	if s.logger != nil {
		s.logger.Log(observability.Record{
			Event:      observability.EventHTTPRequest,
			Component:  "rest",
			RequestID:  reqID,
			Capability: capID,
			Result:     observability.CodeClass(status),
			DurationMS: float64(time.Since(start).Milliseconds()),
		})
	}
}

func requestID(r *http.Request) string {
	if id := r.Header.Get(headerRequestID); id != "" {
		return id
	}
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "req-fallback"
	}
	return hex.EncodeToString(b[:])
}

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(status int) {
	w.code = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) status() int {
	if w.code == 0 {
		return http.StatusOK
	}
	return w.code
}

// inflightMax is the current concurrency cap.
func (s *Server) inflightMax() int {
	if s == nil {
		return 0
	}
	return s.inflight.getMax()
}

// syncManagementLimits publishes the active snapshot's HTTP limits. The
// mutex is taken before Active so overlapping apply and reset hooks cannot
// store an older snapshot after a newer one.
func (s *Server) syncManagementLimits() {
	if s == nil {
		return
	}
	s.limitsMu.Lock()
	defer s.limitsMu.Unlock()
	appSvc, ok := s.svc.(*app.App)
	if !ok {
		return
	}
	snap := appSvc.Active()
	if snap == nil || snap.Canonical == nil {
		return
	}
	m := snap.Canonical.Spec.Management
	s.ApplyLimits(m.BodyLimit, m.RequestsPerSecond, m.Burst, m.MaxConcurrent)
}

// ApplyLimits updates live management HTTP admission knobs.
func (s *Server) ApplyLimits(bodyLimit int64, rps, burst, maxConcurrent int) {
	if s == nil {
		return
	}
	if bodyLimit <= 0 {
		bodyLimit = DefaultMaxBodyBytes
	}
	s.maxBody.Store(bodyLimit)
	s.rate.setRate(float64(rps), float64(burst))
	n := maxConcurrent
	if n <= 0 {
		n = DefaultMaxConcurrent
	}
	s.inflight.setMax(n)
}

// inflightGate counts in-flight management requests. acquire does not wait.
type inflightGate struct {
	mu  sync.Mutex
	cur int
	max int
}

func (g *inflightGate) acquire() bool {
	if g == nil {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.max > 0 && g.cur >= g.max {
		return false
	}
	g.cur++
	return true
}

func (g *inflightGate) release() {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.cur > 0 {
		g.cur--
	}
}

func (g *inflightGate) setMax(n int) {
	if g == nil {
		return
	}
	g.mu.Lock()
	g.max = n
	g.mu.Unlock()
}

func (g *inflightGate) getMax() int {
	if g == nil {
		return 0
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.max
}
