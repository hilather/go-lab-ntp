package observability

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Listener is the optional scrape HTTP server for spec.observability.metrics.listen.
type Listener struct {
	mu     sync.Mutex
	srv    *http.Server
	ln     net.Listener
	closed bool
}

// Handler serves OpenMetrics from reg. A nil registry writes only # EOF.
func Handler(reg *Registry) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", OpenMetricsContentType)
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		if err := reg.WriteOpenMetrics(w); err != nil {
			return
		}
	})
}

// Listen binds addr and serves /metrics (and /). Empty addr returns (nil, nil).
func Listen(addr string, reg *Registry) (*Listener, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, nil
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	h := Handler(reg)
	mux.Handle("/metrics", h)
	mux.Handle("/", h)
	s := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
	}
	l := &Listener{srv: s, ln: ln}
	go func() { _ = s.Serve(ln) }()
	return l, nil
}

// Addr is the bound address, or "" if disabled / not started.
func (l *Listener) Addr() string {
	if l == nil {
		return ""
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.ln == nil {
		return ""
	}
	return l.ln.Addr().String()
}

// Shutdown stops the scrape listener.
func (l *Listener) Shutdown(ctx context.Context) error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil
	}
	l.closed = true
	srv := l.srv
	l.mu.Unlock()
	if srv != nil {
		return srv.Shutdown(ctx)
	}
	return nil
}
