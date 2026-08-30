package ntpwire

import (
	"encoding/binary"
	"fmt"
)

// Parse reads a 48-byte NTP header. Extra trailing bytes are ignored here;
// callers enforce MAC / strip policy.
func Parse(b []byte) (Packet, error) {
	if len(b) < PacketSize {
		return Packet{}, fmt.Errorf("ntpwire: short packet (%d)", len(b))
	}
	var p Packet
	b0 := b[0]
	p.LI = b0 >> 6
	p.VN = (b0 >> 3) & 0x7
	p.Mode = b0 & 0x7
	p.Stratum = b[1]
	p.Poll = int8(b[2])
	p.Precision = int8(b[3])
	p.RootDelay = int32(binary.BigEndian.Uint32(b[4:8]))
	p.RootDisp = binary.BigEndian.Uint32(b[8:12])
	copy(p.RefID[:], b[12:16])
	p.RefTime = parseTS(b[16:24])
	p.OrgTime = parseTS(b[24:32])
	p.RecTime = parseTS(b[32:40])
	p.XmtTime = parseTS(b[40:48])
	return p, nil
}

func parseTS(b []byte) Timestamp {
	return Timestamp{
		Seconds:  binary.BigEndian.Uint32(b[0:4]),
		Fraction: binary.BigEndian.Uint32(b[4:8]),
	}
}

// Append encodes p as 48 bytes onto dst.
func Append(dst []byte, p Packet) []byte {
	var hdr [PacketSize]byte
	hdr[0] = (p.LI << 6) | (p.VN << 3) | p.Mode
	hdr[1] = p.Stratum
	hdr[2] = byte(p.Poll)
	hdr[3] = byte(p.Precision)
	binary.BigEndian.PutUint32(hdr[4:8], uint32(p.RootDelay))
	binary.BigEndian.PutUint32(hdr[8:12], p.RootDisp)
	copy(hdr[12:16], p.RefID[:])
	putTS(hdr[16:24], p.RefTime)
	putTS(hdr[24:32], p.OrgTime)
	putTS(hdr[32:40], p.RecTime)
	putTS(hdr[40:48], p.XmtTime)
	return append(dst, hdr[:]...)
}

func putTS(b []byte, ts Timestamp) {
	binary.BigEndian.PutUint32(b[0:4], ts.Seconds)
	binary.BigEndian.PutUint32(b[4:8], ts.Fraction)
}

// Encode returns a new 48-byte header.
func Encode(p Packet) []byte {
	return Append(nil, p)
}

// KoD returns a kiss-of-death reply for req. Timestamps other than originate
// are zero. RefID is the kiss code (RATE).
func KoD(req Packet, code [4]byte) Packet {
	return Packet{
		LI:      LIUnsync,
		VN:      req.VN,
		Mode:    ModeServer,
		Stratum: 0,
		RefID:   code,
		OrgTime: req.XmtTime,
	}
}
