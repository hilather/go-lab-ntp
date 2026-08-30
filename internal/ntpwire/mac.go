package ntpwire

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash"
	"strings"
)

// DigestLen returns the MAC digest length for alg (MD5/SHA1/SHA256).
func DigestLen(alg string) (int, error) {
	switch strings.ToUpper(alg) {
	case "MD5":
		return md5.Size, nil
	case "SHA1":
		return sha1.Size, nil
	case "SHA256":
		return sha256.Size, nil
	default:
		return 0, fmt.Errorf("ntpwire: unknown MAC algorithm %q", alg)
	}
}

func newHash(alg string) (hash.Hash, error) {
	switch strings.ToUpper(alg) {
	case "MD5":
		return md5.New(), nil
	case "SHA1":
		return sha1.New(), nil
	case "SHA256":
		return sha256.New(), nil
	default:
		return nil, fmt.Errorf("ntpwire: unknown MAC algorithm %q", alg)
	}
}

// MAC computes ntpd concatenation ALG(key || header[0:48]), not HMAC.
func MAC(alg string, key, header []byte) ([]byte, error) {
	if len(header) < PacketSize {
		return nil, fmt.Errorf("ntpwire: MAC header short")
	}
	h, err := newHash(alg)
	if err != nil {
		return nil, err
	}
	_, _ = h.Write(key)
	_, _ = h.Write(header[:PacketSize])
	return h.Sum(nil), nil
}

// TrailerSize is 4 + digestLen.
func TrailerSize(digestLen int) int {
	return 4 + digestLen
}

// AppendMAC appends keyid_be32 || digest.
func AppendMAC(pkt []byte, keyID uint32, digest []byte) []byte {
	var id [4]byte
	binary.BigEndian.PutUint32(id[:], keyID)
	pkt = append(pkt, id[:]...)
	return append(pkt, digest...)
}

// SplitMAC splits a packet into header and optional MAC trailer.
// ok is false when the trailer length is not 4+16/20/32.
func SplitMAC(pkt []byte) (header []byte, keyID uint32, digest []byte, ok bool) {
	if len(pkt) < PacketSize {
		return nil, 0, nil, false
	}
	if len(pkt) == PacketSize {
		return pkt[:PacketSize], 0, nil, true
	}
	rest := pkt[PacketSize:]
	if len(rest) < 4 {
		return pkt[:PacketSize], 0, nil, false
	}
	dlen := len(rest) - 4
	if dlen != md5.Size && dlen != sha1.Size && dlen != sha256.Size {
		return pkt[:PacketSize], 0, nil, false
	}
	keyID = binary.BigEndian.Uint32(rest[:4])
	return pkt[:PacketSize], keyID, rest[4:], true
}

// VerifyMAC reports whether pkt's trailer matches ALG(key || header).
func VerifyMAC(alg string, key, pkt []byte) bool {
	header, _, digest, ok := SplitMAC(pkt)
	if !ok || len(digest) == 0 {
		return false
	}
	want, err := MAC(alg, key, header)
	if err != nil || len(want) != len(digest) {
		return false
	}
	var diff byte
	for i := range want {
		diff |= want[i] ^ digest[i]
	}
	return diff == 0
}
