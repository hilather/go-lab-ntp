package audit

import "context"

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
