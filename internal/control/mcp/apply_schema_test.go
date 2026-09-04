package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// smokeReplaceFiltersArgs is the mcp-integration-lab / issue #2 payload:
// tester-first smoke-offset then catch-all default, offset as integer ns,
// ViewSpec zero-default wire fields omitted.
func smokeReplaceFiltersArgs(expectedRevision string) map[string]any {
	return map[string]any{
		"expectedRevision": expectedRevision,
		"reason":           "issue-2-repro",
		"operations": []any{
			map[string]any{
				"op": "replaceFilters",
				"filters": []any{
					map[string]any{
						"name":    "smoke-offset",
						"enabled": true,
						"match":   map[string]any{"cidrs": []any{"10.99.42.20/32"}},
						"view": map[string]any{
							"mode":    "offset",
							"offset":  -360000000000,
							"leap":    "none",
							"stratum": 2,
							"refid":   "LOCL",
						},
					},
					map[string]any{
						"name":    "default",
						"enabled": true,
						"match":   map[string]any{"cidrs": []any{"0.0.0.0/0", "::/0"}},
						"view": map[string]any{
							"mode":    "follow-real",
							"leap":    "none",
							"stratum": 2,
							"refid":   "LOCL",
						},
					},
				},
			},
		},
	}
}

func TestChangeApplyOmitsZeroDefaultViewFields(t *testing.T) {
	s, svc := newTestServer(t)
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)
	ctx := t.Context()

	rev := string(svc.Active().Revision)

	res, err := cs.CallTool(ctx, &sdk.CallToolParams{
		Name:      "ntp_change_apply",
		Arguments: smokeReplaceFiltersArgs(rev),
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("ntp_change_apply isError: %s", toolText(res))
	}
	if applied, _ := structuredMap(res)["Applied"].(bool); !applied {
		t.Fatalf("Applied=%v structured=%v", applied, res.StructuredContent)
	}

	prev, err := cs.CallTool(ctx, &sdk.CallToolParams{
		Name:      "ntp_views_preview",
		Arguments: map[string]any{"ip": "10.99.42.20"},
	})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if prev.IsError {
		t.Fatalf("ntp_views_preview isError: %s", toolText(prev))
	}
	m := structuredMap(prev)
	if m["filter"] != "smoke-offset" || m["mode"] != "offset" {
		t.Fatalf("preview 10.99.42.20 = %+v", m)
	}

	other, err := cs.CallTool(ctx, &sdk.CallToolParams{
		Name:      "ntp_views_preview",
		Arguments: map[string]any{"ip": "10.99.42.21"},
	})
	if err != nil {
		t.Fatalf("preview other: %v", err)
	}
	om := structuredMap(other)
	if om["filter"] != "default" || om["mode"] != "follow-real" {
		t.Fatalf("preview 10.99.42.21 = %+v", om)
	}
}

func TestChangeApplyViewSchemaOmitsZeroDefaults(t *testing.T) {
	s, _ := newTestServer(t)
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)
	wantOptional := []string{"precision", "rootDelay", "rootDispersion", "jitter", "offset", "leap", "refid"}
	found := map[string]bool{}
	for tool, err := range cs.Tools(t.Context(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		switch tool.Name {
		case "ntp_change_apply", "ntp_change_plan", "ntp_state_validate":
			req := viewRequiredAt(t, tool.InputSchema, "properties", "operations", "items", "properties", "filters", "items", "properties", "view")
			assertViewRequired(t, tool.Name+" filters[].view", req, wantOptional)
			single := viewRequiredAt(t, tool.InputSchema, "properties", "operations", "items", "properties", "filter", "properties", "view")
			assertViewRequired(t, tool.Name+" filter.view", single, wantOptional)
			found[tool.Name] = true
		case "ntp_filters_put":
			req := viewRequiredAt(t, tool.InputSchema, "properties", "filter", "properties", "view")
			assertViewRequired(t, tool.Name+" filter.view", req, wantOptional)
			found[tool.Name] = true
		}
	}
	if !found["ntp_change_apply"] || !found["ntp_filters_put"] || !found["ntp_change_plan"] || !found["ntp_state_validate"] {
		t.Fatalf("missing tools: %v", found)
	}
}

func TestChangeApplyRejectsOmittedViewMode(t *testing.T) {
	s, svc := newTestServer(t)
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)
	args := smokeReplaceFiltersArgs(string(svc.Active().Revision))
	ops := args["operations"].([]any)
	op := ops[0].(map[string]any)
	filters := op["filters"].([]any)
	view := filters[0].(map[string]any)["view"].(map[string]any)
	delete(view, "mode")
	res, err := cs.CallTool(t.Context(), &sdk.CallToolParams{Name: "ntp_change_apply", Arguments: args})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatal("want schema isError when view.mode is omitted")
	}
	if !strings.Contains(toolText(res), "mode") {
		t.Fatalf("want missing mode, got %s", toolText(res))
	}
}

func assertViewRequired(t *testing.T, where string, required []string, optional []string) {
	t.Helper()
	have := map[string]bool{}
	for _, n := range required {
		have[n] = true
	}
	if !have["mode"] || !have["stratum"] {
		t.Fatalf("%s required=%v want mode and stratum", where, required)
	}
	for _, n := range optional {
		if have[n] {
			t.Fatalf("%s still requires %q: %v", where, n, required)
		}
	}
}

func viewRequiredAt(t *testing.T, schema any, path ...string) []string {
	t.Helper()
	cur := asMap(schema)
	if cur == nil {
		t.Fatalf("schema is %T", schema)
	}
	for _, key := range path {
		next, ok := cur[key]
		if !ok {
			t.Fatalf("missing %q under %v", key, path)
		}
		if key == "view" {
			m := asMap(next)
			raw, ok := m["required"]
			if !ok {
				return nil
			}
			return asStringSlice(raw)
		}
		cur = asMap(next)
		if cur == nil {
			t.Fatalf("%q is %T", key, next)
		}
	}
	t.Fatal("path did not end at view")
	return nil
}

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	return m
}

func asStringSlice(v any) []string {
	switch x := v.(type) {
	case []string:
		return x
	case []any:
		out := make([]string, 0, len(x))
		for _, e := range x {
			s, _ := e.(string)
			out = append(out, s)
		}
		return out
	default:
		return nil
	}
}

func toolText(res *sdk.CallToolResult) string {
	if res == nil {
		return ""
	}
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*sdk.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func structuredMap(res *sdk.CallToolResult) map[string]any {
	if res == nil {
		return nil
	}
	if m, ok := res.StructuredContent.(map[string]any); ok {
		return m
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		return nil
	}
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	return m
}
