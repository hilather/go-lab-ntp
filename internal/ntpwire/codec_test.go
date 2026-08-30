package ntpwire

import (
	"bytes"
	"testing"
	"time"
)

func TestUnixZeroNTPSeconds(t *testing.T) {
	ts := FromTime(time.Unix(0, 0).UTC())
	if ts.Seconds != 2208988800 {
		t.Fatalf("unix 0 NTP seconds = %d, want 2208988800", ts.Seconds)
	}
	got := ts.Time(0)
	if !got.Equal(time.Unix(0, 0).UTC()) {
		t.Fatalf("Time(0) = %s", got)
	}
}

func TestRoundTripEra0(t *testing.T) {
	cases := []time.Time{
		time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Unix(0, 0).UTC(),
		time.Date(2035, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2036, 2, 7, 6, 28, 15, 0, time.UTC),
	}
	for _, in := range cases {
		ts := FromTime(in)
		out := ts.Time(Era(in))
		d := out.Sub(in)
		if d < 0 {
			d = -d
		}
		if d > time.Nanosecond {
			t.Fatalf("%s round-trip %s delta %s", in, out, d)
		}
	}
}

func TestEra1Boundary(t *testing.T) {
	in := ntpEra1End.Add(-time.Second)
	if Clamped(in) {
		t.Fatal("era1 end -1s must not clamp")
	}
	if Era(in) != 1 {
		t.Fatalf("era = %d", Era(in))
	}
	ts := FromTime(in)
	out := ts.Time(1)
	d := out.Sub(in)
	if d < 0 {
		d = -d
	}
	if d > time.Second { // 1s precision enough for this lock
		t.Fatalf("era1 round-trip %s -> %s", in, out)
	}
}

func TestClampPre1900(t *testing.T) {
	pre := time.Date(1899, 12, 31, 0, 0, 0, 0, time.UTC)
	if !Clamped(pre) {
		t.Fatal("pre-1900 must clamp")
	}
	got := ClampServed(pre)
	if !got.Equal(ntpEra0Start) {
		t.Fatalf("clamp = %s", got)
	}
	ts := FromTime(pre)
	if ts.Time(0).Before(ntpEra0Start) {
		t.Fatal("encoded pre-1900 escaped the floor")
	}
}

func TestParseEncodeRoundTrip(t *testing.T) {
	in := Packet{
		LI: LIInsert, VN: 4, Mode: ModeClient,
		Stratum: 2, Poll: 6, Precision: -20,
		RootDelay: 1, RootDisp: 2,
		RefID:   [4]byte{'L', 'O', 'C', 'L'},
		XmtTime: FromTime(time.Unix(0, 0).UTC()),
	}
	b := Encode(in)
	if len(b) != PacketSize {
		t.Fatalf("len=%d", len(b))
	}
	out, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	if out.LI != in.LI || out.VN != in.VN || out.Mode != in.Mode || out.XmtTime != in.XmtTime {
		t.Fatalf("%+v vs %+v", out, in)
	}
}

func TestParseShort(t *testing.T) {
	if _, err := Parse(make([]byte, 47)); err == nil {
		t.Fatal("expected short")
	}
}

func TestKoDRate(t *testing.T) {
	req := Packet{VN: 4, Mode: ModeClient, XmtTime: FromTime(time.Unix(1, 0).UTC())}
	k := KoD(req, KissRATE)
	if k.Stratum != 0 || k.Mode != ModeServer || k.LI != LIUnsync || k.RefID != KissRATE {
		t.Fatalf("%+v", k)
	}
	if k.OrgTime != req.XmtTime || !k.XmtTime.Zero() {
		t.Fatalf("org/xmt %+v", k)
	}
	b := Encode(k)
	if !bytes.Equal(b[12:16], []byte("RATE")) {
		t.Fatalf("refid %q", b[12:16])
	}
}

func TestHeaderOctet(t *testing.T) {
	b := Encode(Packet{LI: 2, VN: 3, Mode: 3})
	if b[0] != (2<<6)|(3<<3)|3 {
		t.Fatalf("octet0=%#x", b[0])
	}
}
