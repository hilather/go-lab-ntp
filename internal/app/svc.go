package app

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/hilather/go-lab-ntp/internal/audit"
	"github.com/hilather/go-lab-ntp/internal/buildinfo"
	"github.com/hilather/go-lab-ntp/internal/capabilities"
	"github.com/hilather/go-lab-ntp/internal/compiler"
	"github.com/hilather/go-lab-ntp/internal/config"
	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/ntpview"
	"github.com/hilather/go-lab-ntp/internal/observability"
	"github.com/hilather/go-lab-ntp/internal/querylog"
	"github.com/hilather/go-lab-ntp/internal/snapshot"
)

const (
	defaultIdempotencyMax = 256
	defaultAuditMax       = 128
)

// Options constructs an App.
type Options struct {
	Snapshots      *snapshot.Store
	Now            func() time.Time
	Clock          ntpview.Clock
	BootstrapPath  string
	IdempotencyMax int
	AuditMax       int
	Auditor        audit.Sink
	Metrics        *observability.Registry
	Logger         *observability.Logger
	QueryLog       *querylog.Ring
	// NTPListenOverride is --ntp-listen (empty uses YAML). Flag wins on Reset.
	NTPListenOverride string
	// MgmtListenOverride is --management-listen including off/none/-.
	MgmtListenOverride string
}

// App is the process-local Service implementation.
type App struct {
	mu            sync.Mutex
	snaps         *snapshot.Store
	now           func() time.Time
	clock         ntpview.Clock
	bootstrapPath string
	idemp         *idempCache
	audit         *audit.Fanout
	resetHooks    []func()
	applyHooks    []func()
	metrics       *observability.Registry
	logger        *observability.Logger
	queryLog      *querylog.Ring
	ntpOverride   string
	mgmtOverride  string

	healthMu sync.Mutex
	health   func() observability.Facts

	ntpRebind  func(addr string) error
	httpRebind func(addr string) error
}

var _ Service = (*App)(nil)

// New returns an App. A nil Snapshots becomes an empty snapshot.Store.
func New(opts Options) *App {
	if opts.Snapshots == nil {
		opts.Snapshots = snapshot.NewStore()
	}
	if opts.Now == nil {
		opts.Now = func() time.Time { return time.Now() }
	}
	if opts.Clock == nil {
		opts.Clock = ntpview.SystemClock{}
	}
	idempMax := opts.IdempotencyMax
	if idempMax <= 0 {
		idempMax = defaultIdempotencyMax
	}
	auditMax := opts.AuditMax
	if auditMax <= 0 {
		if snap := opts.Snapshots.Load(); snap != nil && snap.Canonical != nil && snap.Canonical.Spec.Observability.Audit.Ring > 0 {
			auditMax = snap.Canonical.Spec.Observability.Audit.Ring
		} else {
			auditMax = defaultAuditMax
		}
	}
	return &App{
		snaps:         opts.Snapshots,
		now:           opts.Now,
		clock:         opts.Clock,
		bootstrapPath: opts.BootstrapPath,
		idemp:         newIdempCache(idempMax),
		audit:         audit.NewFanout(auditMax, opts.Auditor),
		metrics:       opts.Metrics,
		logger:        opts.Logger,
		queryLog:      opts.QueryLog,
		ntpOverride:   opts.NTPListenOverride,
		mgmtOverride:  opts.MgmtListenOverride,
	}
}

// Boot loads bootstrap YAML, compiles a snapshot, and installs it.
func Boot(ctx context.Context, opts Options) (*App, error) {
	_ = ctx
	if opts.BootstrapPath == "" {
		return nil, domainerr.ValidationFailed("bootstrap path is required",
			domainerr.FieldViolation{Path: "bootstrapPath", Code: "required", Message: "bootstrap path is required"})
	}
	st, err := config.LoadFile(opts.BootstrapPath)
	if err != nil {
		return nil, asDomain(err)
	}
	snap, err := compiler.Compile(st, compiler.CompileOpts{})
	if err != nil {
		return nil, asDomain(err)
	}
	if opts.Snapshots == nil {
		opts.Snapshots = snapshot.NewStore()
	}
	opts.Snapshots.InstallBootstrap(snap)
	if opts.QueryLog == nil {
		opts.QueryLog = querylog.New(snap.QueryLogSize)
	}
	return New(opts), nil
}

// Snapshots is the live config pointer the NTP server re-reads per packet.
func (s *App) Snapshots() *snapshot.Store {
	if s == nil {
		return nil
	}
	return s.snaps
}

// Active is the live snapshot, or nil.
func (s *App) Active() *snapshot.Snapshot {
	if s == nil || s.snaps == nil {
		return nil
	}
	return s.snaps.Load()
}

// QueryLog is the process query ring.
func (s *App) QueryLog() *querylog.Ring {
	if s == nil {
		return nil
	}
	return s.queryLog
}

// SetQueryLog replaces the ring pointer (serve wires the NTP ring).
func (s *App) SetQueryLog(log *querylog.Ring) {
	if s == nil {
		return
	}
	s.queryLog = log
}

// Close is a no-op placeholder for Boot callers.
func (s *App) Close() {}

// SetHealth installs live listener facts for Status.Ready / Evaluate.
func (s *App) SetHealth(fn func() observability.Facts) {
	if s == nil {
		return
	}
	s.healthMu.Lock()
	s.health = fn
	s.healthMu.Unlock()
}

// SetNTPRebind installs the D8 NTP bind-new-first hook.
func (s *App) SetNTPRebind(fn func(addr string) error) {
	if s == nil {
		return
	}
	s.ntpRebind = fn
}

// SetHTTPRebind installs the D8 management HTTP bind-new-first hook.
// Empty addr means unbind (management off).
func (s *App) SetHTTPRebind(fn func(addr string) error) {
	if s == nil {
		return
	}
	s.httpRebind = fn
}

// OnReset registers a hook fired after a successful Reset (outside the mutex).
func (s *App) OnReset(fn func()) {
	if s == nil || fn == nil {
		return
	}
	s.mu.Lock()
	s.resetHooks = append(s.resetHooks, fn)
	s.mu.Unlock()
}

// OnApply registers a hook fired after a successful Apply (outside the mutex).
func (s *App) OnApply(fn func()) {
	if s == nil || fn == nil {
		return
	}
	s.mu.Lock()
	s.applyHooks = append(s.applyHooks, fn)
	s.mu.Unlock()
}

// HealthFacts is the input to observability.Evaluate.
func (s *App) HealthFacts() observability.Facts {
	if s == nil {
		return observability.Facts{}
	}
	s.healthMu.Lock()
	fn := s.health
	s.healthMu.Unlock()
	snapUp := s.Active() != nil
	if fn != nil {
		f := fn()
		f.SnapshotUp = snapUp
		return f
	}
	return observability.Facts{SnapshotUp: snapUp, NTPBound: snapUp, MgmtOff: true}
}

func (s *App) requireCtx(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func (s *App) active() (*snapshot.Snapshot, error) {
	if s == nil || s.snaps == nil {
		return nil, domainerr.Internal("no snapshot store")
	}
	snap := s.snaps.Load()
	if snap == nil {
		return nil, domainerr.Internal("no active snapshot")
	}
	return snap, nil
}

func (s *App) Version(ctx context.Context, actor Actor) (*buildinfo.Info, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	info := buildinfo.Current()
	return &info, nil
}

func (s *App) Capabilities(ctx context.Context, actor Actor) (*CapabilityView, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	src := capabilities.DiscoveryList()
	out := make([]CapabilityInfo, 0, len(src))
	for _, d := range src {
		out = append(out, CapabilityInfo{
			Name: d.Name, Version: d.Version, Description: d.Description,
			Mutating: d.Mutating, Idempotent: d.Idempotent,
		})
	}
	return &CapabilityView{Capabilities: out}, nil
}

func (s *App) Features(ctx context.Context, actor Actor) (*FeatureList, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	return &FeatureList{Items: capabilities.Features()}, nil
}

func (s *App) ConfigSchema(ctx context.Context, actor Actor) ([]byte, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	b, err := config.SchemaBytes()
	if err != nil {
		return nil, domainerr.Internal("schema unavailable")
	}
	return b, nil
}

func asDomain(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := domainerr.As(err); ok {
		return err
	}
	return domainerr.Internal(err.Error())
}

func managementOff(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "off", "none", "-":
		return true
	default:
		return false
	}
}

func effectiveNTP(override string, yamlAddr string) string {
	if override != "" {
		return override
	}
	return yamlAddr
}

func effectiveMgmt(override string, yamlAddr string) string {
	if override != "" {
		if managementOff(override) {
			return ""
		}
		return override
	}
	return yamlAddr
}
