package audit

import "testing"

// BearerPrefix false leaves a "bearer " reason unchanged.
func TestOptionPinBearerPrefixOff(t *testing.T) {
	ev := RedactEvent(Event{Reason: "bearer abc"})
	if ev.Reason != "bearer abc" {
		t.Fatalf("reason %q", ev.Reason)
	}
}

// ColonLines false leaves a "password:" line unchanged.
func TestOptionPinColonLinesOff(t *testing.T) {
	ev := RedactEvent(Event{Reason: "password: hunter2"})
	if ev.Reason != "password: hunter2" {
		t.Fatalf("reason %q", ev.Reason)
	}
}
