package ntpserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hilather/go-lab-ntp/internal/ntpview"
	"github.com/hilather/go-lab-ntp/internal/ntpwire"
	"github.com/hilather/go-lab-ntp/internal/observability"
	"github.com/hilather/go-lab-ntp/internal/querylog"
	"github.com/hilather/go-lab-ntp/internal/snapshot"
)

const (
	DefaultMaxInflight  = 1024
	DefaultShutdownWait = 5 * time.Second
	LimitedRatePerSec   = 0.5
	limitedBurst        = 1
)

// Config is the UDP listener configuration.
type Config struct {
	Addr        string
	Store       *snapshot.Store
	Clock       ntpview.Clock
	Log         *querylog.Ring
	MaxInflight int
	MaxUDPSize  int
	Metrics     *observability.Registry
}

// Server is a dual-stack unicast NTP listener.
type Server struct {
	cfg Config

	ctx    context.Context
	cancel context.CancelFunc

	mu       sync.Mutex
	udp      net.PacketConn
	bindAddr string
	started  bool
	stopped  bool

	inflight chan struct{}
	global   *queryLimiter
	perIP    *queryLimiter
	limited  *queryLimiter

	wg sync.WaitGroup

	// Metrics
	Oversize  atomic.Int64
	Short     atomic.Int64
	Version   atomic.Int64
	Mode      atomic.Int64
	ZeroXmit  atomic.Int64
	Allowlist atomic.Int64
	Admission atomic.Int64
	Ignore    atomic.Int64
	Unmatched atomic.Int64
	KoD       atomic.Int64
	Serve     atomic.Int64

	filterMu    sync.Mutex
	filterNames map[string]struct{}
}

// New validates cfg. Start binds and serves.
func New(cfg Config) (*Server, error) {
	if cfg.Store == nil {
		return nil, errors.New("ntpserver: Store is required")
	}
	if cfg.Addr == "" {
		return nil, errors.New("ntpserver: Addr is required")
	}
	if cfg.Clock == nil {
		cfg.Clock = ntpview.SystemClock{}
	}
	if cfg.MaxInflight <= 0 {
		cfg.MaxInflight = DefaultMaxInflight
	}
	if cfg.MaxUDPSize <= 0 {
		cfg.MaxUDPSize = ntpwire.MaxUDPSize
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		cfg:         cfg,
		ctx:         ctx,
		cancel:      cancel,
		bindAddr:    cfg.Addr,
		inflight:    make(chan struct{}, cfg.MaxInflight),
		global:      newQueryLimiter(256, 512, nil),
		perIP:       newQueryLimiter(32, 64, nil),
		limited:     newQueryLimiter(LimitedRatePerSec, limitedBurst, nil),
		filterNames: map[string]struct{}{},
	}, nil
}

// BindAddr is the requested listen address (string identity for Rebind).
func (s *Server) BindAddr() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bindAddr
}

// Inflight is the number of in-flight packet handlers.
func (s *Server) Inflight() int {
	if s == nil {
		return 0
	}
	return len(s.inflight)
}

// Bound reports whether a PacketConn is currently serving.
func (s *Server) Bound() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.udp != nil && s.started && !s.stopped
}

func (s *Server) syncAdmission(snap *snapshot.Snapshot) {
	if s == nil || snap == nil {
		return
	}
	s.global.setRate(float64(snap.MaxPacketsPerSec), float64(snap.MaxPacketsPerSec*2))
	s.perIP.setRate(float64(snap.MaxPacketsPerIP), float64(snap.MaxPacketsPerIP*2))
}

func (s *Server) observe(decision, version string) {
	if s == nil || s.cfg.Metrics == nil {
		return
	}
	s.cfg.Metrics.Inc(observability.MetricPacketsTotal, map[string]string{
		"decision": decision,
		"version":  version,
	}, 1)
}

func (s *Server) observeFilter(name string) {
	if s == nil || s.cfg.Metrics == nil {
		return
	}
	label := name
	if label == "" {
		label = "other"
	}
	s.filterMu.Lock()
	if _, ok := s.filterNames[label]; !ok {
		if len(s.filterNames) >= 128 {
			label = "other"
		} else {
			s.filterNames[label] = struct{}{}
		}
	}
	s.filterMu.Unlock()
	s.cfg.Metrics.Inc(observability.MetricFilterHitsTotal, map[string]string{"filter": label}, 1)
}

func (s *Server) observeQuerylogDrop() {
	if s == nil || s.cfg.Metrics == nil {
		return
	}
	s.cfg.Metrics.Inc(observability.MetricQuerylogDroppedTotal, nil, 1)
}

// Start binds and serves in the background.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return errors.New("ntpserver: already started")
	}
	if s.stopped {
		return errors.New("ntpserver: start after shutdown")
	}
	pc, err := net.ListenPacket("udp", s.cfg.Addr)
	if err != nil {
		return fmt.Errorf("ntpserver: udp listen: %w", err)
	}
	s.udp = pc
	s.bindAddr = s.cfg.Addr
	s.started = true
	s.wg.Add(1)
	go s.serveUDP()
	return nil
}

// Rebind binds addr first, then drains/closes the old conn. Unchanged addr
// (string identity of the requested listen) does not rebind.
func (s *Server) Rebind(addr string) error {
	s.mu.Lock()
	if !s.started || s.stopped {
		s.mu.Unlock()
		return errors.New("ntpserver: not running")
	}
	if addr == s.bindAddr && s.udp != nil {
		s.mu.Unlock()
		return nil
	}
	old := s.udp
	s.mu.Unlock()

	pc, err := net.ListenPacket("udp", addr)
	if err != nil {
		return fmt.Errorf("ntpserver: rebind: %w", err)
	}
	s.mu.Lock()
	s.udp = pc
	s.bindAddr = addr
	s.cfg.Addr = addr
	s.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	return nil
}

// Shutdown stops the read loop and waits up to ctx.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return nil
	}
	s.stopped = true
	s.cancel()
	udp := s.udp
	s.mu.Unlock()
	if udp != nil {
		_ = udp.Close()
	}
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Addr is the bound UDP address, or nil.
func (s *Server) Addr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.udp == nil {
		return nil
	}
	return s.udp.LocalAddr()
}

// PacketConn returns the current PacketConn (tests: rebind identity).
func (s *Server) PacketConn() net.PacketConn {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.udp
}

func (s *Server) conn() net.PacketConn {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.udp
}

func (s *Server) serveUDP() {
	defer s.wg.Done()
	buf := make([]byte, s.cfg.MaxUDPSize+1)
	for {
		if s.ctx.Err() != nil {
			return
		}
		pc := s.conn()
		if pc == nil {
			return
		}
		n, addr, err := pc.ReadFrom(buf)
		if err != nil {
			if s.ctx.Err() != nil || isClosed(err) {
				if s.stoppedOrCanceled() {
					return
				}
				continue
			}
			continue
		}
		tRecv := s.cfg.Clock.Now()
		pkt := make([]byte, n)
		copy(pkt, buf[:n])
		if !s.acquireInflight() {
			s.Admission.Add(1)
			continue
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer s.releaseInflight()
			s.handle(pkt, addr, tRecv)
		}()
	}
}

func (s *Server) stoppedOrCanceled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopped || s.ctx.Err() != nil
}

func (s *Server) acquireInflight() bool {
	select {
	case s.inflight <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s *Server) releaseInflight() {
	select {
	case <-s.inflight:
	default:
	}
}

func isClosed(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, net.ErrClosed) || errors.Is(err, context.Canceled)
}

func peerFromAddr(addr net.Addr) netip.Addr {
	if addr == nil {
		return netip.Addr{}
	}
	if a, ok := addr.(interface{ AddrPort() netip.AddrPort }); ok {
		return a.AddrPort().Addr().Unmap()
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return netip.Addr{}
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return ip.Unmap()
}
