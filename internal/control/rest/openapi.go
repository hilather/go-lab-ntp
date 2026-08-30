package rest

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/hilather/go-lab-ntp/internal/capabilities"
)

// OpenAPIRelPath is the generated OpenAPI document, relative to the module root.
const OpenAPIRelPath = "api/openapi/v1.json"

// OpenAPIVersion is the OpenAPI specification version of the generated file.
const OpenAPIVersion = "3.1.0"

// RenderOpenAPI builds OpenAPI 3.1 JSON from the capability registry.
func RenderOpenAPI() ([]byte, error) {
	doc := map[string]any{
		"openapi": OpenAPIVersion,
		"info": map[string]any{
			"title":          "LabNTP Management API",
			"version":        capabilities.VersionTag,
			"description":    "REST adapter for the shared LabNTP capability registry. Generated from internal/capabilities. Do not edit by hand.",
			"x-generated-by": "internal/control/rest.RenderOpenAPI; DO NOT EDIT.",
		},
		"servers": []any{
			map[string]any{"url": "/", "description": "Management listener (default address " + DefaultAddr + ")"},
		},
		"paths": openAPIPaths(),
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"bearerAuth": map[string]any{"type": "http", "scheme": "bearer"},
			},
		},
		"security": []any{map[string]any{"bearerAuth": []any{}}},
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

func openAPIPaths() map[string]any {
	paths := map[string]any{}
	for _, c := range capabilities.All() {
		for _, b := range c.REST {
			item, _ := paths[b.Path].(map[string]any)
			if item == nil {
				item = map[string]any{}
			}
			item[strings.ToLower(b.Method)] = map[string]any{
				"operationId": string(c.ID),
				"summary":     c.Title,
				"description": c.Description,
				"tags":        []any{string(c.ID)},
			}
			paths[b.Path] = item
		}
	}
	return paths
}
