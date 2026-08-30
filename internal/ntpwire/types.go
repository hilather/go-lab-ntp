package ntpwire

import "time"

const (
	PacketSize = 48
	MaxUDPSize = 576

	LINone   = 0
	LIInsert = 1
	LIDelete = 2
	LIUnsync = 3

	ModeClient    = 3
	ModeServer    = 4
	ModeBroadcast = 5

	ntpUnixDelta = 2208988800
)

// Packet is the 48-byte NTP header. No padding; encode via binary.BigEndian.
type Packet struct {
	LI        uint8
	VN        uint8
	Mode      uint8
	Stratum   uint8
	Poll      int8
	Precision int8
	RootDelay int32
	RootDisp  uint32
	RefID     [4]byte
	RefTime   Timestamp
	OrgTime   Timestamp
	RecTime   Timestamp
	XmtTime   Timestamp
}

// Timestamp is NTP 32.32 seconds since 1900-01-01 UTC, era-truncated.
type Timestamp struct {
	Seconds  uint32
	Fraction uint32
}

// Zero reports an all-zero timestamp (RFC 4330 duplicate/bogus xmit).
func (ts Timestamp) Zero() bool {
	return ts.Seconds == 0 && ts.Fraction == 0
}

var (
	ntpEra0Start = time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	ntpEra0End   = time.Date(2036, 2, 7, 6, 28, 16, 0, time.UTC)
	ntpEra1End   = time.Date(2172, 3, 15, 12, 56, 32, 0, time.UTC)
)

// KissRATE is the KoD refid "RATE".
var KissRATE = [4]byte{'R', 'A', 'T', 'E'}

// KissDENY is reserved; v1 does not emit it.
var KissDENY = [4]byte{'D', 'E', 'N', 'Y'}

// KissRSTR is reserved; v1 does not emit it.
var KissRSTR = [4]byte{'R', 'S', 'T', 'R'}
