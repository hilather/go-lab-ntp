package rest

import (
	"testing"
	"time"
)

// Idle floor is 30s. Burst 1 at 0.2/s refills one token in 5s, so the allow
// after the gap cannot succeed on wall time. The factor term is 20s. A 25s
// idle stays inside the 30s floor and outside that factor. A 1s floor evicts
// the bucket and the next allow is fresh.
func TestOptionPinRESTIdleFloor(t *testing.T) {
	lim := newLimiter(0.2, 1)
	remote := "203.0.113.40:1"
	if err := lim.allow(remote); err != nil {
		t.Fatal(err)
	}
	assertRateLimited(t, lim.allow(remote))
	lim.evictIdleLocked(time.Now().Add(25 * time.Second))
	assertRateLimited(t, lim.allow(remote))
}
