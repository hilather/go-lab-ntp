package testutil

import (
	"testing"
	"time"
)

func TestFakeClockAdvance(t *testing.T) {
	start := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	c := NewFakeClock(start)
	if !c.Now().Equal(start) {
		t.Fatal("now")
	}
	c.Advance(10 * time.Second)
	if c.Now().Sub(start) != 10*time.Second {
		t.Fatal("advance")
	}
	c.Set(start)
	if !c.Now().Equal(start) {
		t.Fatal("set")
	}
}
