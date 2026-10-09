package mcp

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/hilather/go-lab-controlkit/ratelimit"

	"github.com/hilather/go-lab-ntp/internal/app"
	"github.com/hilather/go-lab-ntp/internal/auth"
	"github.com/hilather/go-lab-ntp/internal/capabilities"
	"github.com/hilather/go-lab-ntp/internal/config"
	"github.com/hilather/go-lab-ntp/internal/domainerr"
)

func actorOf(p auth.Principal) app.Actor {
	return app.Actor{
		ID:        p.ID,
		Class:     p.Class,
		Role:      p.Role,
		Scopes:    append([]string(nil), p.Scopes...),
		Transport: "mcp",
	}
}

func (s *Server) authenticate(r *http.Request) (app.Actor, error) {
	if s.cfg.Auth == nil {
		return app.Actor{}, domainerr.Unauthenticated("authentication required")
	}
	h := strings.TrimSpace(r.Header.Get(headerAuthorization))
	if h != "" && strings.HasPrefix(strings.ToLower(h), "basic ") {
		return app.Actor{}, domainerr.Unauthenticated("MCP accepts bearer tokens only")
	}
	p, err := s.cfg.Auth.Authenticate(auth.Request{
		Authorization: h,
		RemoteAddr:    r.RemoteAddr,
	})
	if err != nil {
		return app.Actor{}, err
	}
	return actorOf(p), nil
}

func (s *Server) authorizeResource(actor app.Actor, uri string) error {
	if s.cfg.Auth == nil {
		return domainerr.Unauthenticated("authentication required")
	}
	cap, ok := capabilities.LookupResource(uri)
	if !ok {
		return domainerr.NotFound("not found")
	}
	return auth.AuthorizeScopes(actor.Scopes, cap.RequiredScopes)
}

func (s *Server) authorizeTool(actor app.Actor, name string) error {
	if s.cfg.Auth == nil {
		return domainerr.Unauthenticated("authentication required")
	}
	caps := capabilities.LookupTool(name)
	if len(caps) == 0 {
		return nil
	}
	return auth.AuthorizeScopes(actor.Scopes, caps[0].RequiredScopes)
}

type limiter struct {
	keyed *ratelimit.Keyed
}

// maxManagementBuckets matches ntpserver.DefaultMaxInflight and the regression ceiling.
const maxManagementBuckets = 1024

func newLimiter(rate, burst float64) *limiter {
	keyed, err := ratelimit.NewKeyed(rate, burst, ratelimit.Ctor{
		DefaultRate:  float64(config.DefaultRequestsPerSecond),
		DefaultBurst: float64(config.DefaultBurst),
		NegativeRate: ratelimit.NegativeRateDisabled,
		ZeroRate:     ratelimit.ZeroRateUseDefault,
		Burst:        ratelimit.BurstDefaultOnZero,
	}, ratelimit.Options{
		IdleFloor:        30 * time.Second,
		IdleRefillFactor: 4,
		MaxKeys:          maxManagementBuckets,
		Now:              nil,
	})
	if err != nil {
		panic("ratelimit.NewKeyed: " + err.Error())
	}
	return &limiter{keyed: keyed}
}

func (l *limiter) allow(remote string) error {
	if l == nil || l.keyed == nil {
		return nil
	}
	key := remote
	if host, _, err := net.SplitHostPort(remote); err == nil {
		key = host
	}
	if l.keyed.Allow(key) {
		return nil
	}
	return domainerr.RateLimited("too many management requests")
}
