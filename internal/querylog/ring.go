// Package querylog is the last-N NTP query ring. Reset wipes it. Not persisted.
package querylog

import (
	"sync"
	"time"
)

// Entry is one answered (or KoD) query.
type Entry struct {
	ClientIP   string
	Filter     string
	ServedTime time.Time
	Leap       string
	Mode       string
	VN         uint8
	WhenHost   time.Time
}

// Ring is a bounded mutex-protected ring. TryInsert drops the sample if the
// lock is not acquired within 100µs.
type Ring struct {
	mu      sync.Mutex
	items   []Entry
	next    int
	size    int
	dropped int
}

// New returns a ring of cap n (clamped 1..4096).
func New(n int) *Ring {
	if n < 1 {
		n = 1
	}
	if n > 4096 {
		n = 4096
	}
	return &Ring{items: make([]Entry, n)}
}

// TryInsert records e or drops it after 100µs.
func (r *Ring) TryInsert(e Entry) bool {
	if r == nil {
		return false
	}
	deadline := time.Now().Add(100 * time.Microsecond)
	for {
		if r.mu.TryLock() {
			r.items[r.next%len(r.items)] = e
			r.next++
			if r.size < len(r.items) {
				r.size++
			}
			r.mu.Unlock()
			return true
		}
		if !time.Now().Before(deadline) {
			if r.mu.TryLock() {
				r.dropped++
				r.mu.Unlock()
			}
			return false
		}
	}
}

// Dropped is the number of skipped samples.
func (r *Ring) Dropped() int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.dropped
}

// List returns newest-first copies.
func (r *Ring) List() []Entry {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Entry, 0, r.size)
	for i := 0; i < r.size; i++ {
		idx := (r.next - 1 - i) % len(r.items)
		if idx < 0 {
			idx += len(r.items)
		}
		out = append(out, r.items[idx])
	}
	return out
}

// Reset wipes the ring.
func (r *Ring) Reset() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next = 0
	r.size = 0
	clear(r.items)
}

// Cap is the configured ring size.
func (r *Ring) Cap() int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.items)
}

// Resize changes capacity, keeping the newest samples. Clamped 1..4096.
func (r *Ring) Resize(n int) {
	if r == nil {
		return
	}
	if n < 1 {
		n = 1
	}
	if n > 4096 {
		n = 4096
	}
	items := r.List()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items = make([]Entry, n)
	r.next = 0
	r.size = 0
	keep := items
	if len(keep) > n {
		keep = keep[:n]
	}
	for i := len(keep) - 1; i >= 0; i-- {
		r.items[r.next%len(r.items)] = keep[i]
		r.next++
		if r.size < len(r.items) {
			r.size++
		}
	}
}
