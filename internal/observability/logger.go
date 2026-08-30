package observability

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Level is a structured log severity matching spec.observability.logLevel.
type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Record is one structured event. Client IPs, Authorization, Cookie, and
// key material are never fields on this type.
type Record struct {
	Timestamp  time.Time
	Level      Level
	Event      string
	Component  string
	RequestID  string
	Capability string
	Result     string
	ErrorCode  string
	DurationMS float64
}

// Logger writes slog JSON events.
type Logger struct {
	mu      sync.Mutex
	handler slog.Handler
	min     slog.Level
	reg     *Registry
	now     func() time.Time
}

// ParseLevel maps YAML logLevel to a slog level. Unknown values are info.
func ParseLevel(s string) Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return LevelDebug
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

func slogLevel(l Level) slog.Level {
	switch l {
	case LevelDebug:
		return slog.LevelDebug
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// NewLogger writes JSON lines to w on the calling goroutine. w may be nil (discard).
func NewLogger(w io.Writer, min Level) *Logger {
	if min == "" {
		min = LevelInfo
	}
	lvl := slogLevel(min)
	var h slog.Handler
	if w != nil {
		h = slog.NewJSONHandler(w, &slog.HandlerOptions{Level: lvl})
	}
	return &Logger{handler: h, min: lvl, now: time.Now}
}

// SetRegistry attaches optional drop counters.
func (l *Logger) SetRegistry(r *Registry) {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.reg = r
	l.mu.Unlock()
}

// Log emits one structured event.
func (l *Logger) Log(rec Record) {
	if l == nil || l.handler == nil {
		return
	}
	if rec.Timestamp.IsZero() {
		if l.now != nil {
			rec.Timestamp = l.now()
		} else {
			rec.Timestamp = time.Now()
		}
	}
	if rec.Level == "" {
		rec.Level = LevelInfo
	}
	lvl := slogLevel(rec.Level)
	if !l.handler.Enabled(context.Background(), lvl) {
		return
	}
	sr := slog.NewRecord(rec.Timestamp, lvl, rec.Event, 0)
	sr.AddAttrs(
		slog.String("event", rec.Event),
		slog.String("component", rec.Component),
	)
	if rec.RequestID != "" {
		sr.AddAttrs(slog.String("request_id", rec.RequestID))
	}
	if rec.Capability != "" {
		sr.AddAttrs(slog.String("capability", rec.Capability))
	}
	if rec.Result != "" {
		sr.AddAttrs(slog.String("result", rec.Result))
	}
	if rec.ErrorCode != "" {
		sr.AddAttrs(slog.String("error_code", rec.ErrorCode))
	}
	if rec.DurationMS != 0 {
		sr.AddAttrs(slog.Float64("duration_ms", rec.DurationMS))
	}
	l.mu.Lock()
	h := l.handler
	l.mu.Unlock()
	_ = h.Handle(context.Background(), sr)
}
