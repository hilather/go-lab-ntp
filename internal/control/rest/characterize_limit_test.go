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
	for i := 0; i < 80; i++ {
		if err := disabled.allow("203.0.113.1:1"); err != nil {
			t.Fatalf("rate < 0 must disable: %v", err)
		}
	}
	disabled.setRate(9, 9)
	for i := 0; i < 80; i++ {
		if err := disabled.allow("203.0.113.1:1"); err != nil {
			t.Fatalf("disabled setRate changed the limiter: %v", err)
		}
	}

	zero := newLimiter(0, 0)
	if zero.rate != 32 {
		t.Fatalf("newLimiter(0, 0) rate %v", zero.rate)
	}
	assertAllowsThenDeny(t, zero, "203.0.113.2:1", 64)

	negBurst := newLimiter(1, -5)
	assertRateLimited(t, negBurst.allow("203.0.113.3:1"))

	live := newLimiter(5, 7)
	assertAllowsThenDeny(t, live, "203.0.113.4:1", 7)
	live.setRate(0, 0)
	if live.rate != 32 {
		t.Fatalf("setRate(0, 0) rate %v", live.rate)
	}
	assertAllowsThenDeny(t, live, "203.0.113.5:1", 64)
	live.setRate(-1, -2)
	if live.rate != 32 {
		t.Fatalf("setRate(-1, -2) rate %v", live.rate)
	}
	assertAllowsThenDeny(t, live, "203.0.113.6:1", 64)
	live.setRate(3, 9)
	assertAllowsThenDeny(t, live, "203.0.113.7:1", 9)

	// Idle cutoff is max(30s, 4*burst/rate). burst 10 at 1/s is 40s, so a
	// 35s-idle exhausted bucket stays denied. After setRate(10, 10) the
	// cutoff falls to 30s and the same age is a fresh bucket.
	idle := newLimiter(1, 10)
	remote := "203.0.113.8:9"
	for i := 0; i < 10; i++ {
		if err := idle.allow(remote); err != nil {
			t.Fatalf("fill %d: %v", i, err)
		}
	}
	assertRateLimited(t, idle.allow(remote))
	idle.evictIdleLocked(time.Now().Add(35 * time.Second))
	assertRateLimited(t, idle.allow(remote))
	idle.setRate(10, 10)
	idle.evictIdleLocked(time.Now().Add(35 * time.Second))
	if err := idle.allow(remote); err != nil {
		t.Fatal("35s is outside the 30s cutoff after setRate")
	}
}

func assertAllowsThenDeny(t *testing.T, l *limiter, remote string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if err := l.allow(remote); err != nil {
			t.Fatalf("%s allow %d: %v", remote, i, err)
		}
	}
	assertRateLimited(t, l.allow(remote))
}

func assertRateLimited(t *testing.T, err error) {
	t.Helper()
	de, ok := domainerr.As(err)
	if !ok || de.Code != domainerr.CodeRateLimited || de.Message != "too many management requests" {
		t.Fatalf("deny: %v", err)
	}
}
