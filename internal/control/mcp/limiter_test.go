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
	if got := len(l.buckets); got > 1024 {
		t.Fatalf("mcp per-IP buckets grew to %d without eviction", got)
	}
}
