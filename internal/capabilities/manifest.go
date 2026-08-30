package capabilities

import (
	"bytes"
	"encoding/json"
)

// ManifestRelPath is the generated machine-readable table, relative to the module root.
const ManifestRelPath = "api/capabilities/v1.json"

// ManifestAPIVersion identifies the manifest document shape.
const ManifestAPIVersion = "labntp.dev/capabilities/v1"

// ManifestGeneratedBy is embedded so verify-generated can treat the file as generated.
const ManifestGeneratedBy = "scripts/generate; DO NOT EDIT."

// Manifest is the generated capability table.
type Manifest struct {
	APIVersion   string       `json:"apiVersion"`
	GeneratedBy  string       `json:"generatedBy"`
	Capabilities []Capability `json:"capabilities"`
	Features     []Feature    `json:"features"`
}

// RenderManifest returns pretty-printed JSON for api/capabilities/v1.json.
func RenderManifest() ([]byte, error) {
	doc := Manifest{
		APIVersion:   ManifestAPIVersion,
		GeneratedBy:  ManifestGeneratedBy,
		Capabilities: All(),
		Features:     Features(),
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return nil, err
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}
