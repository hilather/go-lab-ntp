package ntpview

import "time"

// Clock is the sole real-time input. Now must keep a monotonic reading when
// sourced from time.Now (D22). Do not call .UTC() here.
type Clock interface {
	Now() time.Time
}

// SystemClock is the process clock.
type SystemClock struct{}

// Now returns time.Now without .UTC().
func (SystemClock) Now() time.Time { return time.Now() }
