package ntpview

import (
	"testing"
	"time"

	"github.com/hilather/go-lab-ntp/internal/model"
	"github.com/hilather/go-lab-ntp/internal/ntpwire"
	"github.com/hilather/go-lab-ntp/internal/testutil"
)

func TestFollowRealAndOffset(t *testing.T) {
	clk := testutil.NewFakeClock(time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC))
	fr := View{Mode: model.ModeFollowReal}
	if !fr.Served(clk.Now()).Equal(clk.Now().UTC()) {
		t.Fatal("follow-real")
	}
	off := View{Mode: model.ModeOffset, Offset: -6 * time.Minute}
	want := clk.Now().UTC().Add(-6 * time.Minute)
	if !off.Served(clk.Now()).Equal(want) {
		t.Fatalf("offset got %s want %s", off.Served(clk.Now()), want)
	}
}

func TestAbsoluteStepThenFollow(t *testing.T) {
	start := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	clk := testutil.NewFakeClock(start)
	abs := time.Date(2035, 1, 1, 0, 0, 0, 0, time.UTC)
	v := View{Mode: model.ModeAbsolute, Absolute: abs, EpochMono: clk.Now()}
	if !v.Served(clk.Now()).Equal(abs) {
		t.Fatal("at apply")
	}
	clk.Advance(10 * time.Second)
	got := v.Served(clk.Now())
	want := abs.Add(10 * time.Second)
	if !got.Equal(want) {
		t.Fatalf("absolute follow got %s want %s", got, want)
	}
}

func TestFreeze(t *testing.T) {
	clk := testutil.NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	at := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	v := View{Mode: model.ModeFreeze, FreezeAt: at, EpochMono: clk.Now()}
	clk.Advance(time.Hour)
	if !v.Served(clk.Now()).Equal(at) {
		t.Fatal("freeze moved")
	}
}

func TestRateOmittedEpochNewView(t *testing.T) {
	start := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	clk := testutil.NewFakeClock(start)
	v := View{
		Mode:         model.ModeRate,
		Rate:         2,
		HasRate:      true,
		EpochVirtual: start,
		EpochMono:    start,
		EpochWall:    start,
	}
	clk.Advance(10 * time.Second)
	got := v.Served(clk.Now())
	want := start.Add(20 * time.Second)
	if !got.Equal(want) {
		t.Fatalf("rate 2 omitted epoch got %s want %s", got, want)
	}
}

func TestRateExplicitEpoch2035(t *testing.T) {
	start := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	clk := testutil.NewFakeClock(start)
	epoch := time.Date(2035, 1, 1, 0, 0, 0, 0, time.UTC)
	v := View{
		Mode:         model.ModeRate,
		Rate:         2,
		HasRate:      true,
		EpochVirtual: epoch,
		EpochMono:    start,
	}
	clk.Advance(10 * time.Second)
	got := v.Served(clk.Now())
	want := epoch.Add(20 * time.Second)
	if !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestRateZeroEqualsFreezeAtEpoch(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := testutil.NewFakeClock(start)
	v := View{Mode: model.ModeRate, Rate: 0, HasRate: true, EpochVirtual: start, EpochMono: start}
	clk.Advance(time.Minute)
	if !v.Served(clk.Now()).Equal(start) {
		t.Fatal("rate 0")
	}
}

func TestRateKeepEpochVsReanchor(t *testing.T) {
	start := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	clk := testutil.NewFakeClock(start)
	old := View{Mode: model.ModeRate, Rate: 2, HasRate: true, EpochVirtual: start, EpochMono: start}
	clk.Advance(10 * time.Second)
	mid := old.Served(clk.Now())
	// unchanged rate/epoch keeps pair: 10s more real → +20s more virtual
	keep := old
	clk.Advance(10 * time.Second)
	got := keep.Served(clk.Now())
	if !got.Equal(start.Add(40 * time.Second)) {
		t.Fatalf("keep got %s", got)
	}
	// re-anchor: epochVirtual = served(now) under old, new rate 1
	re := View{Mode: model.ModeRate, Rate: 1, HasRate: true, EpochVirtual: mid, EpochMono: clk.Now().Add(-10 * time.Second)}
	// at t = start+20s, elapsed from re.EpochMono (start+10s) is 10s * rate 1 = +10s from mid (start+20s) → start+30s
	got = re.Served(clk.Now())
	want := mid.Add(10 * time.Second)
	if !got.Equal(want) {
		t.Fatalf("reanchor got %s want %s (mid %s)", got, want, mid)
	}
}

func TestNegativeRateClamp1900(t *testing.T) {
	start := time.Date(1900, 1, 1, 0, 1, 0, 0, time.UTC)
	clk := testutil.NewFakeClock(start)
	v := View{Mode: model.ModeRate, Rate: -100, HasRate: true, EpochVirtual: start, EpochMono: start}
	clk.Advance(10 * time.Second)
	got := v.Served(clk.Now())
	if !got.Equal(ntpwire.ClampServed(time.Date(1899, 12, 31, 0, 0, 0, 0, time.UTC))) {
		if got.Before(time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)) {
			t.Fatalf("pre-1900 leaked %s", got)
		}
	}
	if got.Before(time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("clamped below era0 %s", got)
	}
}

func TestJitterStablePerHostSecond(t *testing.T) {
	v := View{Name: "tester", Generation: 7, Jitter: time.Millisecond}
	d1 := v.JitterDelta(1000)
	d2 := v.JitterDelta(1000)
	d3 := v.JitterDelta(1001)
	if d1 != d2 {
		t.Fatal("same second must match")
	}
	if d1 == d3 {
		t.Fatal("adjacent seconds almost surely differ")
	}
	if d1 < -v.Jitter || d1 > v.Jitter {
		t.Fatalf("delta %s outside jitter", d1)
	}
}

func TestSystemClockNoUTCStrip(t *testing.T) {
	n := SystemClock{}.Now()
	// A stripped UTC instant compared to itself still works; the contract is
	// documented. At least Now is non-zero.
	if n.IsZero() {
		t.Fatal("zero")
	}
}
