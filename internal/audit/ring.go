package audit

import (
	"strconv"
	"sync"
)

// DefaultMax is the ring size when Max is unset or non-positive.
const DefaultMax = 128

// Ring is a bounded in-memory log. Oldest events fall off the front.
type Ring struct {
	mu     sync.Mutex
	max    int
	nextID uint64
	events []Event
}

// NewRing returns a ring. Non-positive max becomes DefaultMax.
func NewRing(max int) *Ring {
	if max <= 0 {
		max = DefaultMax
	}
	return &Ring{max: max, events: make([]Event, 0, max)}
}

// Append assigns an ID and stores ev. The stored event is returned.
func (r *Ring) Append(ev Event) Event {
	if r == nil {
		return ev
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	ev.ID = "aud-" + strconv.FormatUint(r.nextID, 10)
	r.events = append(r.events, ev)
	if len(r.events) > r.max {
		r.events = append([]Event(nil), r.events[len(r.events)-r.max:]...)
	}
	return ev
}

// List returns the newest-first page.
func (r *Ring) List(limit int) []Event {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	n := len(r.events)
	if limit > n {
		limit = n
	}
	out := make([]Event, limit)
	for i := 0; i < limit; i++ {
		out[i] = r.events[n-1-i]
	}
	return out
}

// Get returns one event by ID.
func (r *Ring) Get(id string) (Event, bool) {
	if r == nil || id == "" {
		return Event{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.events) - 1; i >= 0; i-- {
		if r.events[i].ID == id {
			return r.events[i], true
		}
	}
	return Event{}, false
}

// Len is the current occupancy.
func (r *Ring) Len() int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.events)
}
