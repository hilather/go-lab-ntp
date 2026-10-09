package rest

import (
	"testing"
	"time"

	"github.com/hilather/go-lab-ntp/internal/config"
	"github.com/hilather/go-lab-ntp/internal/domainerr"
)

func TestCharacterizeRESTLimiterCtorAndSetRate(t *testing.T) {
	if config.DefaultRequestsPerSecond != 32 || config.DefaultBurst != 64 {
		t.Fatalf("defaults rps=%d burst=%d", config.DefaultRequestsPerSecond, config.DefaultBurst)
	}

	disabled := newLimiter(-1, 1)
	if !disabled.disabled {
		t.Fatal("rate < 0 must disable")
	}
	if err := disabled.allow("203.0.113.1:1"); err != nil {
		t.Fatal(err)
	}
	disabled.setRate(9, 9)
	if !disabled.disabled || disabled.rate != 0 || disabled.burst != 0 {
		t.Fatalf("disabled setRate changed the limiter: %+v", disabled)
	}

	zero := newLimiter(0, 0)
	if zero.disabled || zero.rate != 32 || zero.burst != 64 {
		t.Fatalf("zero ctor %+v", zero)
	}
	negBurst := newLimiter(1, -5)
	if negBurst.disabled || negBurst.rate != 1 || negBurst.burst != -5 {
		t.Fatalf("negative burst is kept at construction: %+v", negBurst)
	}

	live := newLimiter(5, 7)
	live.setRate(0, 0)
	if live.rate != 32 || live.burst != 64 {
		t.Fatalf("setRate zero %+v", live)
	}
	live.setRate(-1, -2)
	if live.rate != 32 || live.burst != 64 || live.disabled {
		t.Fatalf("setRate negative %+v", live)
	}
	live.setRate(3, 9)
	if live.rate != 3 || live.burst != 9 {
		t.Fatalf("setRate positive %+v", live)
	}

	// Idle cutoff is max(30s, 4*burst/rate). burst 10 at 1/s is 40s.
	// After setRate(10, 10) it falls to 30s, so a 35s-idle bucket is evicted.
	idle := newLimiter(1, 10)
	if err := idle.allow("203.0.113.5:9"); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	idle.mu.Lock()
	idle.buckets["203.0.113.5"].last = now.Add(-35 * time.Second)
	idle.mu.Unlock()
	idle.evictIdleLocked(now)
	if _, ok := idle.buckets["203.0.113.5"]; !ok {
		t.Fatal("35s is inside the 40s cutoff")
	}
	idle.setRate(10, 10)
	idle.evictIdleLocked(now)
	if _, ok := idle.buckets["203.0.113.5"]; ok {
		t.Fatal("35s is outside the 30s cutoff after setRate")
	}

	limited := newLimiter(1, 1)
	if err := limited.allow("203.0.113.9:1"); err != nil {
		t.Fatal(err)
	}
	err := limited.allow("203.0.113.9:2")
	de, ok := domainerr.As(err)
	if !ok || de.Code != domainerr.CodeRateLimited || de.Message != "too many management requests" {
		t.Fatalf("deny: %v", err)
	}
}
