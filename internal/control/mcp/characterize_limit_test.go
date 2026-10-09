package mcp

import (
	"fmt"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/domainerr"
)

func TestCharacterizeMCPLimiterCapAndDenyOrder(t *testing.T) {
	disabled := newLimiter(-1, 3)
	for i := 0; i < 80; i++ {
		if err := disabled.allow("203.0.113.1:1"); err != nil {
			t.Fatalf("rate < 0 must disable: %v", err)
		}
	}

	zero := newLimiter(0, 0)
	assertAllowsThenDeny(t, zero, "203.0.113.2:1", 64)

	kept := newLimiter(1, -5)
	assertRateLimited(t, kept.allow("203.0.113.3:1"))

	// A denied key, then 1023 other keys, still has no fresh bucket: the cap
	// holds at least 1024. Probing it refreshes recency, so the exact-cap
	// eviction is a separate limiter.
	low := newLimiter(1, 1)
	if err := low.allow("203.0.113.8:1"); err != nil {
		t.Fatal(err)
	}
	assertRateLimited(t, low.allow("203.0.113.8:2"))
	for i := 0; i < 1023; i++ {
		if err := low.allow(fmt.Sprintf("10.%d.%d.%d:1", i>>16, (i>>8)&0xff, i&0xff)); err != nil {
			t.Fatalf("fill %d: %v", i, err)
		}
	}
	assertRateLimited(t, low.allow("203.0.113.8:3"))

	// After 1024 newer keys the denied key is gone and the next call is a
	// fresh bucket.
	high := newLimiter(1, 1)
	if err := high.allow("203.0.113.9:1"); err != nil {
		t.Fatal(err)
	}
	assertRateLimited(t, high.allow("203.0.113.9:2"))
	for i := 0; i < 1024; i++ {
		if err := high.allow(fmt.Sprintf("11.%d.%d.%d:1", i>>16, (i>>8)&0xff, i&0xff)); err != nil {
			t.Fatalf("fill %d: %v", i, err)
		}
	}
	if err := high.allow("203.0.113.9:3"); err != nil {
		t.Fatalf("evicted key must get a fresh bucket: %v", err)
	}

	// A deny updates recency, so the denied key is not the one a new key
	// evicts. The previous oldest is.
	order := newLimiter(1, 1)
	keys := make([]string, 1024)
	for i := range keys {
		keys[i] = fmt.Sprintf("12.%d.%d.%d", i>>16, (i>>8)&0xff, i&0xff)
		if err := order.allow(keys[i] + ":1"); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}
	assertRateLimited(t, order.allow(keys[0]+":2"))
	if err := order.allow("198.51.100.1:1"); err != nil {
		t.Fatal(err)
	}
	assertRateLimited(t, order.allow(keys[0]+":3"))
	if err := order.allow(keys[1] + ":2"); err != nil {
		t.Fatalf("oldest key must be the eviction victim: %v", err)
	}
}

func assertAllowsThenDeny(t *testing.T, l *limiter, remote string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if err := l.allow(remote); err != nil {
			t.Fatalf("allow %d: %v", i, err)
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
