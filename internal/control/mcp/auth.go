package mcp

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

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
	disabled bool
	rate     float64
	burst    float64
	mu       sync.Mutex
	buckets  map[string]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

func newLimiter(rate, burst float64) *limiter {
	if rate < 0 {
		return &limiter{disabled: true}
	}
	if rate == 0 {
		rate = float64(config.DefaultRequestsPerSecond)
	}
	if burst == 0 {
		burst = float64(config.DefaultBurst)
	}
	return &limiter{rate: rate, burst: burst, buckets: map[string]*bucket{}}
}

func (l *limiter) allow(remote string) error {
	if l == nil || l.disabled {
		return nil
	}
	key := remote
	if host, _, err := net.SplitHostPort(remote); err == nil {
		key = host
	}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	b := l.buckets[key]
	if b == nil {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}
	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * l.rate
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.last = now
	if b.tokens < 1 {
		return domainerr.RateLimited("too many management requests")
	}
	b.tokens--
	return nil
}
