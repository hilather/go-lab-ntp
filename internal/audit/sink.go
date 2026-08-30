package audit

import (
	"context"
	"sync/atomic"
)

// Sink is an optional external audit destination.
type Sink interface {
	Emit(ctx context.Context, ev Event) error
}

var _ Sink = (*Fanout)(nil)

// SinkFunc adapts a function to Sink.
type SinkFunc func(ctx context.Context, ev Event) error

// Emit calls f.
func (f SinkFunc) Emit(ctx context.Context, ev Event) error {
	return f(ctx, ev)
}

// Fanout writes the ring and then the hook. Hook errors are counted and
// never returned.
type Fanout struct {
	Ring     *Ring
	Hook     Sink
	HookErrs atomic.Uint64
}

// NewFanout returns a fanout with a ring. hook may be nil.
func NewFanout(max int, hook Sink) *Fanout {
	return &Fanout{Ring: NewRing(max), Hook: hook}
}

// Record redacts, stores, and best-effort delivers ev. Always succeeds.
func (f *Fanout) Record(ctx context.Context, ev Event) Event {
	if f == nil {
		return ev
	}
	ev = RedactEvent(ev)
	if f.Ring != nil {
		ev = f.Ring.Append(ev)
	}
	if f.Hook != nil {
		if err := f.Hook.Emit(ctx, ev); err != nil {
			f.HookErrs.Add(1)
		}
	}
	return ev
}

// Emit implements Sink. Hook delivery failure is swallowed.
func (f *Fanout) Emit(ctx context.Context, ev Event) error {
	f.Record(ctx, ev)
	return nil
}

// List is Ring.List.
func (f *Fanout) List(limit int) []Event {
	if f == nil {
		return nil
	}
	return f.Ring.List(limit)
}

// Get is Ring.Get.
func (f *Fanout) Get(id string) (Event, bool) {
	if f == nil {
		return Event{}, false
	}
	return f.Ring.Get(id)
}
