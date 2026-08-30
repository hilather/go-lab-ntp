package config

import (
	"os"
	"path/filepath"
)

// SchemaRelPath is the published v1alpha1 JSON Schema, relative to the module root.
const SchemaRelPath = "api/jsonschema/labntp.dev.v1alpha1.json"

// SchemaBytes returns the published v1alpha1 JSON Schema.
func SchemaBytes() ([]byte, error) {
	root, err := findModuleRoot()
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(root, filepath.FromSlash(SchemaRelPath)))
}

func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
