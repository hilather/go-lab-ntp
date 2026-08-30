// Package testutil holds injectable clocks and packet helpers for tests.
package testutil

import (
	"sync"
	"time"
)

// Clock is a source of time. Now must keep a monotonic reading when sourced
// from time.Now (D22).
type Clock interface {
	Now() time.Time
}

// SystemClock is the process clock.
type SystemClock struct{}

// Now returns time.Now without .UTC().
func (SystemClock) Now() time.Time { return time.Now() }

// FakeClock is a test clock. Advance a single time.Time with Add so
// consecutive Now values are comparable with Sub.
type FakeClock struct {
	mu sync.Mutex
	t  time.Time
}

// NewFakeClock returns a FakeClock at t.
func NewFakeClock(t time.Time) *FakeClock {
	return &FakeClock{t: t}
}

// Now returns the current fake instant.
func (f *FakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.t
}

// Advance moves the fake clock by d.
func (f *FakeClock) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.t = f.t.Add(d)
}

// Set replaces the fake instant.
func (f *FakeClock) Set(t time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.t = t
}
