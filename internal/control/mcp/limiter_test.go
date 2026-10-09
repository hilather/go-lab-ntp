package mcp

import (
	"fmt"
	"testing"
)

func TestManagementLimiterEvicts(t *testing.T) {
	l := newLimiter(32, 64)
	for i := 0; i < 10000; i++ {
		_ = l.allow(fmt.Sprintf("198.51.100.%d:9", i))
	}
	if got := l.keyed.Len(); got > 1024 {
		t.Fatalf("mcp per-IP buckets grew to %d without eviction", got)
	}
}

// An oldest key of "" must still be deleted, or the map grows past the cap.
func TestEvictOldestEmptyKey(t *testing.T) {
	l := newLimiter(32, 64)
	if err := l.allow(""); err != nil {
		t.Fatal(err)
	}
	if !l.keyed.Contains("") {
		t.Fatal("empty key was not inserted")
	}
	for i := 0; i < maxManagementBuckets-1; i++ {
		if err := l.allow(fmt.Sprintf("203.0.113.%d:9", i)); err != nil {
			t.Fatalf("fill %d: %v", i, err)
		}
	}
	if got := l.keyed.Len(); got != maxManagementBuckets {
		t.Fatalf("filled to %d, want cap %d", got, maxManagementBuckets)
	}
	if err := l.allow("192.0.2.50:9"); err != nil {
		t.Fatal(err)
	}
	if got := l.keyed.Len(); got != maxManagementBuckets {
		t.Fatalf("map size %d, want cap %d", got, maxManagementBuckets)
	}
	if l.keyed.Contains("") {
		t.Fatal("oldest empty key was not evicted")
	}
}
