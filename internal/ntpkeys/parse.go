package ntpkeys

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Key is one symmetric key. Key bytes must not be logged.
type Key struct {
	ID     uint32
	Alg    string
	Secret []byte
}

// Table maps key id to Key.
type Table struct {
	ByID map[uint32]Key
}

// ParseFile reads path. Missing file is the caller's compile error.
func ParseFile(path string) (*Table, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(b)
}

// Parse parses an ntp.keys-ish document.
func Parse(data []byte) (*Table, error) {
	t := &Table{ByID: map[uint32]Key{}}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, err := parseLine(line)
		if err != nil {
			return nil, fmt.Errorf("ntpkeys: line %d: %w", i+1, err)
		}
		if _, ok := t.ByID[k.ID]; ok {
			return nil, fmt.Errorf("ntpkeys: duplicate key id %d", k.ID)
		}
		t.ByID[k.ID] = k
	}
	return t, nil
}

func parseLine(line string) (Key, error) {
	fields := strings.Fields(line)
	if len(fields) != 3 {
		return Key{}, fmt.Errorf("want 'keyid algorithm encoding:material'")
	}
	id64, err := strconv.ParseUint(fields[0], 10, 32)
	if err != nil {
		return Key{}, fmt.Errorf("keyid")
	}
	if id64 < 1 || id64 > 65535 {
		return Key{}, fmt.Errorf("keyid must be 1..65535")
	}
	alg := strings.ToUpper(fields[1])
	switch alg {
	case "MD5", "SHA1", "SHA256":
	default:
		return Key{}, fmt.Errorf("algorithm %q", fields[1])
	}
	enc, mat, ok := strings.Cut(fields[2], ":")
	if !ok {
		return Key{}, fmt.Errorf("material must use hex or ascii encoding")
	}
	var secret []byte
	switch strings.ToLower(enc) {
	case "hex":
		secret, err = hex.DecodeString(mat)
		if err != nil {
			return Key{}, fmt.Errorf("hex material")
		}
	case "ascii":
		secret = []byte(mat)
	default:
		return Key{}, fmt.Errorf("encoding %q", enc)
	}
	if len(secret) == 0 {
		return Key{}, fmt.Errorf("empty key")
	}
	return Key{ID: uint32(id64), Alg: alg, Secret: secret}, nil
}

// Zero overwrites secret copies. Call after the snapshot has hashed them
// into a digest table if needed; LabNTP keeps secrets in the compiled table
// for reply MACs, so this is used on discarded parse buffers only.
func Zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
