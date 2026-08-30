package mcp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/hilather/go-lab-ntp/internal/app"
	"github.com/hilather/go-lab-ntp/internal/auth"
	"github.com/hilather/go-lab-ntp/internal/buildinfo"
	"github.com/hilather/go-lab-ntp/internal/config"
	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/observability"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// ProtocolVersion is the only MCP revision first GA speaks (ADR 0006).
	ProtocolVersion = "2026-07-28"

	// SDKModule is the official Go SDK module path.
	SDKModule = "github.com/modelcontextprotocol/go-sdk"

	// SDKVersion is the pinned official SDK tag.
	SDKVersion = "v1.7.0"

	// DefaultPath is the Streamable HTTP mount on the management listener.
	DefaultPath = config.DefaultMCPPath

	DefaultMaxBodyBytes   = 1 << 20
	DefaultRequestTimeout = 30 * time.Second
	DefaultMaxConcurrent  = 256

	headerProtocolVersion = "Mcp-Protocol-Version"
	headerRequestID       = "X-Request-ID"
	headerOrigin          = "Origin"
	headerAuthorization   = "Authorization"
)

// Config constructs the MCP adapter.
type Config struct {
	Service            app.Service
	AllowedOrigins     []string
	AllowLegacyClients bool
	RatePerSec         float64
	RateBurst          float64
	MaxBodyBytes       int64
	RequestTimeout     time.Duration
	MaxConcurrent      int
	Auth               *auth.Verifier
	FixedActor         *app.Actor
	Metrics            *observability.Registry
}

// Server is the official-SDK adapter. Third-party MCP types do not escape it.
type Server struct {
	cfg      Config
	svc      app.Service
	sdk      *sdk.Server
	http     *sdk.StreamableHTTPHandler
	maxBody  int64
	timeout  time.Duration
	inflight chan struct{}
	rate     *limiter
	closed   atomic.Bool
	metrics  *observability.Registry
}

type ctxKey int

const (
	ctxActor ctxKey = iota
	ctxRequestID
)

// New builds a Server. Tools and resources come from the frozen registry.
func New(cfg Config) (*Server, error) {
	if cfg.Service == nil {
		return nil, errors.New("mcp: Service is required")
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

	info := buildinfo.Current()
	impl := &sdk.Implementation{
		Name:    "labntp",
		Title:   "LabNTP",
		Version: info.Version,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	s := &Server{
		cfg:      cfg,
		svc:      cfg.Service,
		maxBody:  maxBody,
		timeout:  timeout,
		inflight: make(chan struct{}, n),
		rate:     newLimiter(cfg.RatePerSec, cfg.RateBurst),
		metrics:  cfg.Metrics,
	}
	sdkOpts := &sdk.ServerOptions{
		Instructions: "LabNTP control plane. Use typed ntp_* tools; do not assume connection state. Protocol " + ProtocolVersion + ".",
		Logger:       logger,
		Capabilities: &sdk.ServerCapabilities{
			Logging:   nil,
			Tools:     &sdk.ToolCapabilities{ListChanged: false},
			Resources: &sdk.ResourceCapabilities{ListChanged: false, Subscribe: false},
		},
		SchemaCache: sdk.NewSchemaCache(),
	}
	s.sdk = sdk.NewServer(impl, sdkOpts)
	if !cfg.AllowLegacyClients {
		s.sdk.AddReceivingMiddleware(pinProtocolMiddleware)
	}
	if appSvc, ok := s.svc.(*app.App); ok {
		appSvc.OnReset(s.reloadAuth)
		appSvc.OnApply(s.reloadAuth)
	}
	s.registerTools()
	s.registerResources()

	s.http = sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server {
		return s.sdk
	}, &sdk.StreamableHTTPOptions{
		// 2026-07-28 Streamable HTTP is accepted only when Stateless is true.
		Stateless:                    true,
		Logger:                       logger,
		MaxRequestBodyBytes:          maxBody,
		PropagateRequestCancellation: true,
		DisableLocalhostProtection:   true,
	})
	return s, nil
}

// Handler returns the Streamable HTTP adapter. Mount it at /mcp.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(s.serveHTTP)
}

// Close marks the adapter stopped.
func (s *Server) Close() {
	s.closed.Store(true)
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	reqID := requestID(r)
	w.Header().Set(headerRequestID, reqID)

	if s.closed.Load() {
		writeRPC(w, http.StatusServiceUnavailable, domainerr.Internal("server closed"))
		return
	}

	if err := checkOrigin(r.Header.Get(headerOrigin), s.cfg.AllowedOrigins); err != nil {
		writeRPC(w, http.StatusForbidden, err)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeRPC(w, http.StatusMethodNotAllowed, domainerr.MethodNotAllowed("method not allowed"))
		return
	}

	select {
	case s.inflight <- struct{}{}:
		defer func() { <-s.inflight }()
	default:
		writeRPC(w, http.StatusTooManyRequests, domainerr.RateLimited("too many concurrent management requests"))
		return
	}
	if err := s.rate.allow(r.RemoteAddr); err != nil {
		writeRPC(w, http.StatusTooManyRequests, err)
		return
	}

	ctx := r.Context()
	var cancel context.CancelFunc
	if s.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}
	ctx = context.WithValue(ctx, ctxRequestID, reqID)
	r = r.WithContext(ctx)

	defer func() {
		if rec := recover(); rec != nil {
			writeRPC(w, http.StatusInternalServerError, domainerr.Internal("internal error"))
		}
	}()

	if !s.cfg.AllowLegacyClients {
		if err := validateProtocolVersion(r); err != nil {
			writeRPC(w, http.StatusBadRequest, err)
			return
		}
	}

	actor, err := s.authenticate(r)
	if err != nil {
		status := http.StatusUnauthorized
		if de, ok := domainerr.As(err); ok && de.Code == domainerr.CodeForbidden {
			status = http.StatusForbidden
		}
		writeRPC(w, status, err)
		return
	}
	r = r.WithContext(withActor(r.Context(), actor))

	s.http.ServeHTTP(w, r)
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

func withActor(ctx context.Context, a app.Actor) context.Context {
	return context.WithValue(ctx, ctxActor, a)
}

func (s *Server) actorFrom(ctx context.Context) app.Actor {
	a, _ := ctx.Value(ctxActor).(app.Actor)
	if a.ID != "" || a.Class != "" {
		if a.Transport == "" {
			a.Transport = "mcp"
		}
		return a
	}
	if s != nil && s.cfg.FixedActor != nil {
		out := *s.cfg.FixedActor
		if out.Transport == "" {
			out.Transport = "mcp"
		}
		return out
	}
	if a.Transport == "" {
		a.Transport = "mcp"
	}
	return a
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
	s.cfg.Auth.Replace(next)
}
