package ntpview

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"net"
	"time"

	"github.com/hilather/go-lab-ntp/internal/model"
	"github.com/hilather/go-lab-ntp/internal/ntpwire"
)

// View is a compiled virtual clock. Immutable after compile.
type View struct {
	Name       string
	Generation uint64
	Mode       string

	Offset   time.Duration
	Absolute time.Time
	FreezeAt time.Time
	Rate     float64
	HasRate  bool

	EpochVirtual time.Time
	EpochMono    time.Time
	EpochWall    time.Time

	Leap           string
	Stratum        int
	RefID          string
	Precision      int
	RootDelay      time.Duration
	RootDispersion time.Duration
	Jitter         time.Duration
	MinPoll        *int
	MaxPoll        *int
}

// Served returns virtual time at real instant t (from Clock.Now).
func (v View) Served(t time.Time) time.Time {
	return ntpwire.ClampServed(v.servedUnclamped(t))
}

func (v View) servedUnclamped(t time.Time) time.Time {
	switch v.Mode {
	case model.ModeFollowReal:
		return t.UTC()
	case model.ModeOffset:
		return t.UTC().Add(v.Offset)
	case model.ModeAbsolute:
		return v.Absolute.Add(t.Sub(v.EpochMono))
	case model.ModeFreeze:
		return v.FreezeAt
	case model.ModeRate:
		elapsed := t.Sub(v.EpochMono)
		delta := saturatingDuration(elapsed.Seconds() * v.Rate)
		return v.EpochVirtual.Add(delta)
	default:
		return t.UTC()
	}
}

// ServedClamped reports whether Served had to clamp t.
func (v View) ServedClamped(t time.Time) bool {
	return ntpwire.Clamped(v.servedUnclamped(t))
}

// JitterDelta is the stable wander for this filter at host unix second.
func (v View) JitterDelta(hostUnixSecond int64) time.Duration {
	if v.Jitter == 0 {
		return 0
	}
	var gen [8]byte
	binary.LittleEndian.PutUint64(gen[:], v.Generation)
	var sec [8]byte
	binary.LittleEndian.PutUint64(sec[:], uint64(hostUnixSecond))
	h := sha256.New()
	_, _ = h.Write([]byte(v.Name))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write(gen[:])
	_, _ = h.Write(sec[:])
	sum := h.Sum(nil)
	n := binary.LittleEndian.Uint64(sum[:8])
	u := float64(n) / math.Ldexp(1, 64)
	return time.Duration((2*u - 1) * float64(v.Jitter))
}

// RefTime is the view's last "sync" timestamp at transmit virtual time.
func (v View) RefTime(tXmitVirt time.Time) time.Time {
	switch v.Mode {
	case model.ModeFreeze:
		return v.FreezeAt
	case model.ModeRate:
		return v.EpochVirtual
	default:
		return tXmitVirt
	}
}

// ClampPoll echoes client poll, optionally clamped into [minpoll, maxpoll].
func (v View) ClampPoll(client int8) int8 {
	p := int(client)
	if v.MinPoll != nil && p < *v.MinPoll {
		p = *v.MinPoll
	}
	if v.MaxPoll != nil && p > *v.MaxPoll {
		p = *v.MaxPoll
	}
	if p < -128 {
		p = -128
	}
	if p > 127 {
		p = 127
	}
	return int8(p)
}

// LeapLI maps YAML leap to LI bits.
func LeapLI(leap string) uint8 {
	switch leap {
	case model.LeapInsert:
		return ntpwire.LIInsert
	case model.LeapDelete:
		return ntpwire.LIDelete
	case model.LeapUnsync:
		return ntpwire.LIUnsync
	default:
		return ntpwire.LINone
	}
}

// EncodeRefID packs a 1–4 ASCII or IPv4 dotted-quad refid.
func EncodeRefID(s string) [4]byte {
	var out [4]byte
	if s == "" {
		copy(out[:], []byte("LOCL"))
		return out
	}
	if ip := net.ParseIP(s); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			copy(out[:], v4)
			return out
		}
	}
	n := len(s)
	if n > 4 {
		n = 4
	}
	copy(out[:], s[:n])
	return out
}

func saturatingDuration(seconds float64) time.Duration {
	if math.IsNaN(seconds) {
		return 0
	}
	maxSec := float64(math.MaxInt64) / float64(time.Second)
	minSec := float64(math.MinInt64) / float64(time.Second)
	if seconds >= maxSec {
		return time.Duration(math.MaxInt64)
	}
	if seconds <= minSec {
		return time.Duration(math.MinInt64)
	}
	return time.Duration(seconds * float64(time.Second))
}
