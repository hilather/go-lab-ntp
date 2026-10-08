package mcp

import (
	"fmt"
	"testing"
	"time"
)

func TestManagementLimiterEvicts(t *testing.T) {
	l := newLimiter(32, 64)
	for i := 0; i < 10000; i++ {
		_ = l.allow(fmt.Sprintf("198.51.100.%d:9", i))
	}
	if got := len(l.buckets); got > 1024 {
		t.Fatalf("mcp per-IP buckets grew to %d without eviction", got)
	}
}

// An oldest key of "" must still be deleted, or the map grows past the cap.
func TestEvictOldestEmptyKey(t *testing.T) {
	l := newLimiter(32, 64)
	if err := l.allow(""); err != nil {
		t.Fatal(err)
	}
	l.mu.Lock()
	b := l.buckets[""]
	if b == nil {
		l.mu.Unlock()
		t.Fatal("empty key was not inserted")
	}
	// Older than the keys filled below, but inside the idle window (30s).
	b.last = time.Now().Add(-10 * time.Second)
	l.mu.Unlock()
	for i := 0; i < maxManagementBuckets-1; i++ {
		if err := l.allow(fmt.Sprintf("203.0.113.%d:9", i)); err != nil {
			t.Fatalf("fill %d: %v", i, err)
		}
	}
	if got := len(l.buckets); got != maxManagementBuckets {
		t.Fatalf("filled to %d, want cap %d", got, maxManagementBuckets)
	}
	if err := l.allow("192.0.2.50:9"); err != nil {
		t.Fatal(err)
	}
	if got := len(l.buckets); got != maxManagementBuckets {
		t.Fatalf("map size %d, want cap %d", got, maxManagementBuckets)
	}
	if _, ok := l.buckets[""]; ok {
		t.Fatal("oldest empty key was not evicted")
	}
}
