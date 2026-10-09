package mcp

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hilather/go-lab-ntp/internal/config"
	"github.com/hilather/go-lab-ntp/internal/domainerr"
)

func TestCharacterizeMCPLimiterCapAndDenyOrder(t *testing.T) {
	src, err := os.ReadFile("auth.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(src), "func (l *limiter) setRate") {
		t.Fatal("mcp limiter has setRate")
	}
	typ := reflect.TypeOf(newLimiter(1, 1))
	for i := 0; i < typ.NumMethod(); i++ {
		if strings.EqualFold(typ.Method(i).Name, "setRate") {
			t.Fatalf("exported %s", typ.Method(i).Name)
		}
	}

	disabled := newLimiter(-1, 3)
	if !disabled.disabled {
		t.Fatal("rate < 0 must disable")
	}
	if err := disabled.allow("203.0.113.1:1"); err != nil {
		t.Fatal(err)
	}
	zero := newLimiter(0, 0)
	if zero.rate != float64(config.DefaultRequestsPerSecond) || zero.burst != float64(config.DefaultBurst) {
		t.Fatalf("zero ctor rate %v burst %v", zero.rate, zero.burst)
	}
	kept := newLimiter(1, -5)
	if kept.burst != -5 || kept.rate != 1 {
		t.Fatalf("negative burst kept: %+v", kept)
	}

	// A denied key refreshes last before the token check, so it stays newest.
	// Rate 1 and a 1ms-old bucket avoid the idle-cutoff duration overflow
	// that a near-zero rate produces (4*burst/rate does not fit in int64).
	deny := newLimiter(1, 1)
	old := time.Now().Add(-time.Millisecond)
	deny.buckets["203.0.113.8"] = &bucket{tokens: 0, last: old}
	err = deny.allow("203.0.113.8:9")
	de, ok := domainerr.As(err)
	if !ok || de.Code != domainerr.CodeRateLimited || de.Message != "too many management requests" {
		t.Fatalf("deny: %v", err)
	}
	if !deny.buckets["203.0.113.8"].last.After(old) {
		t.Fatal("deny must set last to now before the token check")
	}

	// Burst 1000 keeps the idle cutoff above one hour, so the cap eviction
	// runs instead of the idle sweep deleting every seeded bucket.
	l := newLimiter(1, 1000)
	base := time.Now().Add(-time.Hour)
	oldest := "10.0.0.0"
	victim := "10.1.2.3"
	l.buckets[oldest] = &bucket{tokens: 1, last: base}
	l.buckets[victim] = &bucket{tokens: 0, last: time.Now()}
	for i := 1; i < maxManagementBuckets-1; i++ {
		key := fmt.Sprintf("10.2.%d.%d", i>>8, i&0xff)
		l.buckets[key] = &bucket{tokens: 1, last: base.Add(time.Duration(i) * time.Millisecond)}
	}
	if len(l.buckets) != maxManagementBuckets {
		t.Fatalf("cap setup %d", len(l.buckets))
	}
	if err := l.allow(victim + ":9"); err == nil {
		t.Fatal("victim must be denied")
	}
	if _, ok := l.buckets[victim]; !ok {
		t.Fatal("denied key must remain")
	}
	if err := l.allow("11.0.0.1:1"); err != nil {
		t.Fatal(err)
	}
	if _, ok := l.buckets[oldest]; ok {
		t.Fatal("oldest last must be evicted at the cap")
	}
	if _, ok := l.buckets[victim]; !ok {
		t.Fatal("denied key stays most recent and must not be the eviction victim")
	}
	if _, ok := l.buckets["11.0.0.1"]; !ok {
		t.Fatal("new key missing")
	}
	if len(l.buckets) != maxManagementBuckets {
		t.Fatalf("len %d", len(l.buckets))
	}
}
