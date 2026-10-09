package audit

import (
	"context"
	"strconv"

	kitaudit "github.com/hilather/go-lab-controlkit/audit"
)

// DefaultMax is the ring size when Max is unset or non-positive.
const DefaultMax = 128

// redacted is the token ring tests compare against.
const redacted = "[redacted]"

type kitRing = kitaudit.Ring[Event]
type kitFanout = kitaudit.Fanout[Event]

// Ring is a bounded in-memory log. Oldest events fall off the front.
type Ring struct {
	k *kitRing
}

// NewRing returns a ring. Non-positive max becomes DefaultMax.
func NewRing(max int) *Ring {
	if max <= 0 {
		max = DefaultMax
	}
	k, err := kitaudit.NewRing(kitaudit.RingOptions[Event]{
		Max:         max,
		SetID:       func(e *Event, id string) { e.ID = id },
		IsDenied:    nil,
		DeniedShare: 0,
		DefaultList: 100,
		MaxList:     100,
		NewID: func(seq uint64) string {
			return "aud-" + strconv.FormatUint(seq, 10)
		},
		GetID: nil,
	})
	if err != nil {
		panic("audit.NewRing: " + err.Error())
	}
	return &Ring{k: k}
}

// Append assigns an ID and stores ev. The stored event is returned.
func (r *Ring) Append(ev Event) Event {
	if r == nil || r.k == nil {
		return ev
	}
	return r.k.Append(ev)
}

// List returns the newest-first page.
func (r *Ring) List(limit int) []Event {
	if r == nil || r.k == nil {
		return nil
	}
	return r.k.List(limit)
}

// Get returns one event by ID.
func (r *Ring) Get(id string) (Event, bool) {
	if r == nil || r.k == nil || id == "" {
		return Event{}, false
	}
	return r.k.Get(id)
}

// Len is the current occupancy.
func (r *Ring) Len() int {
	if r == nil || r.k == nil {
		return 0
	}
	return r.k.Len()
}

// Fanout writes the ring and then the hook. Hook errors are counted by the
// kit and never returned.
type Fanout struct {
	Ring *Ring
	Hook Sink
	kit  *kitFanout
}

// NewFanout returns a fanout with a ring. hook may be nil.
func NewFanout(max int, hook Sink) *Fanout {
	ring := NewRing(max)
	var sink func(context.Context, Event) error
	if hook != nil {
		sink = hook.Emit
	}
	kit, err := kitaudit.NewFanout(ring.k, RedactEvent, sink)
	if err != nil {
		panic("audit.NewFanout: " + err.Error())
	}
	return &Fanout{Ring: ring, Hook: hook, kit: kit}
}

// Record redacts, stores, and best-effort delivers ev. Always succeeds.
func (f *Fanout) Record(ctx context.Context, ev Event) Event {
	if f == nil || f.kit == nil {
		return ev
	}
	return f.kit.Record(ctx, ev)
}

// Emit implements Sink. Hook delivery failure is swallowed.
func (f *Fanout) Emit(ctx context.Context, ev Event) error {
	if f == nil {
		return nil
	}
	f.Record(ctx, ev)
	return nil
}

// List is Ring.List.
func (f *Fanout) List(limit int) []Event {
	if f == nil || f.Ring == nil {
		return nil
	}
	return f.Ring.List(limit)
}

// Get is Ring.Get.
func (f *Fanout) Get(id string) (Event, bool) {
	if f == nil || f.Ring == nil {
		return Event{}, false
	}
	return f.Ring.Get(id)
}

// RedactEvent copies ev with secret material stripped from reason and diff.
func RedactEvent(ev Event) Event {
	r := ntpRedactor()
	out := ev
	out.Reason = r.String(out.Reason)
	if len(out.Diff) == 0 {
		return out
	}
	diff := make([]RedactedEntry, len(out.Diff))
	for i, d := range out.Diff {
		before, _ := r.Value(d.Path, d.Before)
		after, _ := r.Value(d.Path, d.After)
		diff[i] = RedactedEntry{Path: d.Path, Op: d.Op, Before: before, After: after}
	}
	out.Diff = diff
	return out
}

func ntpRedactor() kitaudit.Redactor {
	return kitaudit.Redactor{
		Keys: map[string]bool{
			"secret":        true,
			"secretref":     true,
			"secretfile":    true,
			"token":         true,
			"password":      true,
			"authorization": true,
			"bearer":        true,
			"credential":    true,
			"credentials":   true,
			"apikey":        true,
			"api_key":       true,
			"privatekey":    true,
			"private_key":   true,
			"cookie":        true,
		},
		PEM:          true,
		BearerPrefix: false,
		ColonLines:   false,
	}
}
