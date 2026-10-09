package rest

import (
	"testing"
	"time"
)

// Idle floor is 30s. At 100/s and burst 1 the refill term is 40ms, so a
// bucket idle for 2s stays. A shorter floor evicts it and the next allow
// is a fresh bucket.
func TestOptionPinRESTIdleFloor(t *testing.T) {
	lim := newLimiter(100, 1)
	remote := "203.0.113.40:1"
	if err := lim.allow(remote); err != nil {
		t.Fatal(err)
	}
	assertRateLimited(t, lim.allow(remote))
	lim.evictIdleLocked(time.Now().Add(2 * time.Second))
	assertRateLimited(t, lim.allow(remote))
}
