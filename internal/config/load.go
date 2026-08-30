package config

import (
	"os"

	"github.com/hilather/go-lab-ntp/internal/model"
)

// Load decodes, normalizes, and validates a YAML or JSON document.
func Load(data []byte) (*model.State, error) {
	st, _, err := LoadWithWarnings(data)
	return st, err
}

// LoadWithWarnings is Load plus non-fatal warnings (universal allowlist).
func LoadWithWarnings(data []byte) (*model.State, []Warning, error) {
	st, err := Decode(data)
	if err != nil {
		return nil, nil, err
	}
	n, warns, err := Normalize(st)
	if err != nil {
		return nil, nil, err
	}
	if err := Validate(n); err != nil {
		return nil, warns, err
	}
	return n, warns, nil
}

// LoadFile reads path and calls Load.
func LoadFile(path string) (*model.State, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Load(b)
}

// LoadFileWithWarnings reads path and calls LoadWithWarnings.
func LoadFileWithWarnings(path string) (*model.State, []Warning, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	return LoadWithWarnings(b)
}
