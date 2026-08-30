package ntpwire

import "time"

// ClampServed clamps t to [1900-01-01T00:00:00Z, 2172-03-15T12:56:32Z).
func ClampServed(t time.Time) time.Time {
	u := t.UTC()
	if u.Before(ntpEra0Start) {
		return ntpEra0Start
	}
	if !u.Before(ntpEra1End) {
		return ntpEra1End.Add(-time.Nanosecond)
	}
	return u
}

// Clamped reports whether t is outside the encodable NTP range.
func Clamped(t time.Time) bool {
	u := t.UTC()
	return u.Before(ntpEra0Start) || !u.Before(ntpEra1End)
}

// Era returns 0 for times before 2036-02-07T06:28:16Z and 1 otherwise,
// after ClampServed.
func Era(t time.Time) int {
	u := ClampServed(t)
	if u.Before(ntpEra0End) {
		return 0
	}
	return 1
}

// FromTime encodes t as an NTP timestamp. t is clamped (D25) then era-truncated.
func FromTime(t time.Time) Timestamp {
	t = ClampServed(t)
	sec := t.Unix() + ntpUnixDelta
	frac := uint64(t.Nanosecond()) * (1 << 32) / 1_000_000_000
	return Timestamp{Seconds: uint32(uint64(sec)), Fraction: uint32(frac)}
}

// Time decodes ts using era 0 (1900–2036) or 1 (next 2^32 seconds).
func (ts Timestamp) Time(era int) time.Time {
	sec := int64(era)*int64(1<<32) + int64(ts.Seconds) - ntpUnixDelta
	nsec := int64(ts.Fraction) * 1_000_000_000 >> 32
	return time.Unix(sec, nsec).UTC()
}

// ShortFromDuration encodes a duration as signed 16.16 seconds, saturating.
func ShortFromDuration(d time.Duration) int32 {
	sec := d.Seconds() * 65536
	if sec > float64(^uint32(0)>>1) {
		return int32(^uint32(0) >> 1)
	}
	if sec < -float64(^uint32(0)>>1)-1 {
		return -int32(^uint32(0)>>1) - 1
	}
	return int32(sec)
}

// UShortFromDuration encodes a duration as unsigned 16.16 seconds, saturating.
func UShortFromDuration(d time.Duration) uint32 {
	if d <= 0 {
		return 0
	}
	sec := d.Seconds() * 65536
	if sec > float64(^uint32(0)) {
		return ^uint32(0)
	}
	return uint32(sec)
}
