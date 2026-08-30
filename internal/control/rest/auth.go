package rest

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
	"github.com/hilather/go-lab-ntp/internal/observability"
)

func actorOf(p auth.Principal, transport string) app.Actor {
	return app.Actor{
		ID:        p.ID,
		Class:     p.Class,
		Role:      p.Role,
		Scopes:    append([]string(nil), p.Scopes...),
		Transport: transport,
	}
}

func (s *Server) authenticate(r *http.Request, skip bool) (app.Actor, error) {
	if skip {
		return app.Actor{ID: "probe", Class: "startup", Transport: "rest"}, nil
	}
	if s.cfg.Auth == nil {
		s.observeAuthFailure("denied")
		return app.Actor{}, domainerr.Unauthenticated("authentication required")
	}

	hdr := strings.TrimSpace(r.Header.Get("Authorization"))
	if hdr != "" {
		p, err := s.cfg.Auth.Authenticate(auth.Request{
			Authorization: hdr,
			RemoteAddr:    r.RemoteAddr,
		})
		if err != nil {
			s.observeAuthFailure("invalid")
			return app.Actor{}, err
		}
		return actorOf(p, "rest"), nil
	}

	if c, err := r.Cookie(auth.CookieName); err == nil && c.Value != "" && s.cfg.Sessions != nil {
		sess, _, ok := s.cfg.Sessions.Lookup(c.Value)
		if ok {
			return actorOf(auth.PrincipalFromSession(sess), "rest"), nil
		}
	}

	p, err := s.cfg.Auth.Authenticate(auth.Request{RemoteAddr: r.RemoteAddr})
	if err != nil {
		s.observeAuthFailure("invalid")
		return app.Actor{}, err
	}
	return actorOf(p, "rest"), nil
}

func (s *Server) observeAuthFailure(reason string) {
	if s == nil {
		return
	}
	_ = observability.AuthFailureReason(reason)
	if s.metrics != nil {
		s.metrics.Inc(observability.MetricAuthFailuresTotal, nil, 1)
	}
	if s.logger != nil {
		s.logger.Log(observability.Record{
			Event:     observability.EventAuthFailure,
			Component: "rest",
			Result:    reason,
			ErrorCode: string(domainerr.CodeUnauthenticated),
		})
	}
}

func (s *Server) authorize(r *http.Request, actor app.Actor, cap capabilities.Capability) error {
	if s.cfg.Auth == nil {
		return domainerr.Unauthenticated("authentication required")
	}
	if err := auth.AuthorizeScopes(actor.Scopes, cap.RequiredScopes); err != nil {
		return err
	}
	if !auth.UnsafeMethod(r.Method) {
		return nil
	}
	if strings.TrimSpace(r.Header.Get("Authorization")) != "" {
		return nil
	}
	c, err := r.Cookie(auth.CookieName)
	if err != nil || c.Value == "" {
		return nil
	}
	if s.cfg.Sessions == nil || !s.cfg.Sessions.ValidCSRF(c.Value, r.Header.Get(auth.CSRFHeader)) {
		return domainerr.Forbidden("CSRF token is missing or invalid")
	}
	return nil
}

func (s *Server) cookieSecure(r *http.Request) bool {
	return auth.CookieSecure(r, s.cfg.CookieSecure)
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
	l.evictIdleLocked(now)
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

func (l *limiter) evictIdleLocked(now time.Time) {
	if l == nil || len(l.buckets) == 0 {
		return
	}
	idleFor := 30 * time.Second
	if l.rate > 0 {
		refill := time.Duration(float64(time.Second) * (l.burst / l.rate) * 4)
		if refill > idleFor {
			idleFor = refill
		}
	}
	for k, b := range l.buckets {
		if now.Sub(b.last) > idleFor {
			delete(l.buckets, k)
		}
	}
}
