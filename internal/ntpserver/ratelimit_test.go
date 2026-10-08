package ntpserver

import (
	"fmt"
	"testing"
	"time"
)

// Spoofed UDP source addresses must not grow the per-IP bucket map without
// a bound. The same limiter serves per-IP admission and restrict: limited.
func TestPerIPLimiterEvicts(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	l := newQueryLimiter(32, 64, func() time.Time { return now })
	for i := 0; i < 10000; i++ {
		l.allow(fmt.Sprintf("198.51.100.%d.%d", i/256, i%256))
	}
	if got := len(l.buckets); got > 1024 {
		t.Fatalf("per-IP buckets grew to %d without eviction", got)
	}
}
