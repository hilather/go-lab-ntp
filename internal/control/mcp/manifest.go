package mcp

import (
	"bytes"
	"encoding/json"
	"runtime/debug"

	"github.com/hilather/go-lab-ntp/internal/capabilities"
)

// ManifestRelPath is the generated MCP capability manifest, relative to the module root.
const ManifestRelPath = "api/mcp/v1.json"

// ManifestAPIVersion identifies the manifest document shape.
const ManifestAPIVersion = "labntp.dev/mcp/v1"

// ManifestGeneratedBy is embedded so verify-generated can treat the file as generated.
const ManifestGeneratedBy = "internal/control/mcp.RenderManifest; DO NOT EDIT."

// Manifest is the generated MCP surface: protocol pin, tools, and resources.
type Manifest struct {
	APIVersion     string   `json:"apiVersion"`
	GeneratedBy    string   `json:"generatedBy"`
	Protocol       string   `json:"protocol"`
	SDK            string   `json:"sdk"`
	SDKVersion     string   `json:"sdkVersion"`
	Tools          []string `json:"tools"`
	Resources      []string `json:"resources"`
	MutatingTools  []string `json:"mutatingTools"`
	FeatureIDs     []string `json:"featureIds"`
	HealthNotTools []string `json:"healthNotTools"`
}

// RenderManifest returns pretty-printed JSON for api/mcp/v1.json.
func RenderManifest() ([]byte, error) {
	var mutating []string
	seen := map[string]bool{}
	for _, c := range capabilities.All() {
		if c.MCP == nil || !c.Mutating {
			continue
		}
		for _, t := range c.MCP.Tools {
			if seen[t] {
				continue
			}
			seen[t] = true
			mutating = append(mutating, t)
		}
	}
	doc := Manifest{
		APIVersion:     ManifestAPIVersion,
		GeneratedBy:    ManifestGeneratedBy,
		Protocol:       ProtocolVersion,
		SDK:            SDKModule,
		SDKVersion:     resolvedSDKVersion(),
		Tools:          capabilities.Tools(),
		Resources:      capabilities.Resources(),
		MutatingTools:  mutating,
		FeatureIDs:     capabilities.FeatureIDs(),
		HealthNotTools: []string{string(capabilities.HealthLive), string(capabilities.HealthReady)},
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

func resolvedSDKVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, d := range info.Deps {
			if d.Path == SDKModule {
				return d.Version
			}
		}
	}
	return SDKVersion
}
