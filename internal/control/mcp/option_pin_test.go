package mcp

import (
	"testing"
	"time"
)

// A zero construction rate uses DefaultRate (32/s), not a 1/s stand-in.
func TestOptionPinMCPDefaultRate(t *testing.T) {
	lim := newLimiter(0, 0)
	remote := "203.0.113.70:1"
	for i := 0; i < 64; i++ {
		if err := lim.allow(remote); err != nil {
			t.Fatalf("burst %d: %v", i, err)
		}
	}
	assertRateLimited(t, lim.allow(remote))
	time.Sleep(time.Second)
	got := 0
	for i := 0; i < 10; i++ {
		if lim.allow(remote) != nil {
			break
		}
		got++
	}
	if got < 8 {
		t.Fatalf("refilled %d tokens in 1s", got)
	}
}

// Now nil is time.Now, so a rate of 1/s refills across a 2s gap.
func TestOptionPinMCPClockRefills(t *testing.T) {
	lim := newLimiter(1, 1)
	remote := "203.0.113.71:1"
	if err := lim.allow(remote); err != nil {
		t.Fatal(err)
	}
	assertRateLimited(t, lim.allow(remote))
	time.Sleep(2 * time.Second)
	if err := lim.allow(remote); err != nil {
		t.Fatalf("refill: %v", err)
	}
}
