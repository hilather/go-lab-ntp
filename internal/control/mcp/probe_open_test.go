package mcp

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
)

// probeGolden is the SDK accept/reject matrix observed on
// github.com/modelcontextprotocol/go-sdk v1.7.0. It records today's
// behavior. It does not assert a future mcpstrict policy.
const probeGolden = `ntp_audit_get	a-root-unknown	reject-schema	additional
ntp_audit_get	d-duplicate	accept-isError	audit event y not found
ntp_audit_get	e-case	reject-schema	additional
ntp_audit_get	f-empty	reject-schema	required
ntp_audit_get	f-omit	reject-schema	required
ntp_audit_list	a-root-unknown	reject-schema	additional
ntp_audit_list	d-duplicate	reject-schema	type
ntp_audit_list	e-case	reject-schema	additional
ntp_audit_list	f-empty	accept-ok
ntp_audit_list	f-omit	accept-ok
ntp_capabilities_get	a-root-unknown	reject-schema	additional
ntp_capabilities_get	d-duplicate	reject-schema	additional
ntp_capabilities_get	e-case	reject-schema	additional
ntp_capabilities_get	f-empty	accept-ok
ntp_capabilities_get	f-omit	accept-ok
ntp_change_apply	a-root-unknown	reject-schema	additional
ntp_change_apply	b-operations.0	reject-schema	additional
ntp_change_apply	b-operations.0.admission	reject-schema	additional
ntp_change_apply	b-operations.0.filter	reject-schema	additional
ntp_change_apply	b-operations.0.filter.match	reject-schema	additional
ntp_change_apply	b-operations.0.filter.view	reject-schema	additional
ntp_change_apply	b-operations.0.filters.0	reject-schema	additional
ntp_change_apply	b-operations.0.filters.0.match	reject-schema	additional
ntp_change_apply	b-operations.0.filters.0.view	reject-schema	additional
ntp_change_apply	b-operations.0.managementHTTP	reject-schema	additional
ntp_change_apply	b-operations.0.queryLog	reject-schema	additional
ntp_change_apply	b-operations.0.restrict	reject-schema	additional
ntp_change_apply	d-duplicate	accept-isError	active revision does not match expectedRevision
ntp_change_apply	e-case	reject-schema	additional
ntp_change_apply	f-empty	accept-isError	expectedRevision is required
ntp_change_apply	f-omit	accept-isError	expectedRevision is required
ntp_change_plan	a-root-unknown	reject-schema	additional
ntp_change_plan	b-operations.0	reject-schema	additional
ntp_change_plan	b-operations.0.admission	reject-schema	additional
ntp_change_plan	b-operations.0.filter	reject-schema	additional
ntp_change_plan	b-operations.0.filter.match	reject-schema	additional
ntp_change_plan	b-operations.0.filter.view	reject-schema	additional
ntp_change_plan	b-operations.0.filters.0	reject-schema	additional
ntp_change_plan	b-operations.0.filters.0.match	reject-schema	additional
ntp_change_plan	b-operations.0.filters.0.view	reject-schema	additional
ntp_change_plan	b-operations.0.managementHTTP	reject-schema	additional
ntp_change_plan	b-operations.0.queryLog	reject-schema	additional
ntp_change_plan	b-operations.0.restrict	reject-schema	additional
ntp_change_plan	d-duplicate	accept-isError	active revision does not match expectedRevision
ntp_change_plan	e-case	reject-schema	additional
ntp_change_plan	f-empty	accept-isError	expectedRevision is required
ntp_change_plan	f-omit	accept-isError	expectedRevision is required
ntp_features_list	a-root-unknown	reject-schema	additional
ntp_features_list	d-duplicate	reject-schema	additional
ntp_features_list	e-case	reject-schema	additional
ntp_features_list	f-empty	accept-ok
ntp_features_list	f-omit	accept-ok
ntp_filters_delete	a-root-unknown	reject-schema	additional
ntp_filters_delete	d-duplicate	reject-schema	required
ntp_filters_delete	e-case	reject-schema	additional
ntp_filters_delete	f-empty	reject-schema	required
ntp_filters_delete	f-omit	reject-schema	required
ntp_filters_get	a-root-unknown	reject-schema	additional
ntp_filters_get	d-duplicate	accept-isError	filter y not found
ntp_filters_get	e-case	reject-schema	additional
ntp_filters_get	f-empty	reject-schema	required
ntp_filters_get	f-omit	reject-schema	required
ntp_filters_list	a-root-unknown	reject-schema	additional
ntp_filters_list	d-duplicate	reject-schema	additional
ntp_filters_list	e-case	reject-schema	additional
ntp_filters_list	f-empty	accept-ok
ntp_filters_list	f-omit	accept-ok
ntp_filters_put	a-root-unknown	reject-schema	additional
ntp_filters_put	b-filter	reject-schema	additional
ntp_filters_put	b-filter.match	reject-schema	additional
ntp_filters_put	b-filter.view	reject-schema	additional
ntp_filters_put	d-duplicate	reject-schema	required
ntp_filters_put	e-case	reject-schema	additional
ntp_filters_put	f-empty	reject-schema	required
ntp_filters_put	f-omit	reject-schema	required
ntp_queries_list	a-root-unknown	reject-schema	additional
ntp_queries_list	d-duplicate	accept-ok
ntp_queries_list	e-case	reject-schema	additional
ntp_queries_list	f-empty	accept-ok
ntp_queries_list	f-omit	accept-ok
ntp_schema_get	a-root-unknown	reject-schema	additional
ntp_schema_get	d-duplicate	reject-schema	additional
ntp_schema_get	e-case	reject-schema	additional
ntp_schema_get	f-empty	accept-ok
ntp_schema_get	f-omit	accept-ok
ntp_state_export	a-root-unknown	reject-schema	additional
ntp_state_export	d-duplicate	accept-isError	unknown export format
ntp_state_export	e-case	reject-schema	additional
ntp_state_export	f-empty	accept-ok
ntp_state_export	f-omit	accept-ok
ntp_state_get	a-root-unknown	reject-schema	additional
ntp_state_get	d-duplicate	reject-schema	additional
ntp_state_get	e-case	reject-schema	additional
ntp_state_get	f-empty	accept-ok
ntp_state_get	f-omit	accept-ok
ntp_state_reset	a-root-unknown	reject-schema	additional
ntp_state_reset	d-duplicate	accept-isError	management auth cannot be loaded
ntp_state_reset	e-case	reject-schema	additional
ntp_state_reset	f-empty	accept-isError	management auth cannot be loaded
ntp_state_reset	f-omit	accept-isError	management auth cannot be loaded
ntp_state_validate	a-root-unknown	reject-schema	additional
ntp_state_validate	b-operations.0	reject-schema	additional
ntp_state_validate	b-operations.0.admission	reject-schema	additional
ntp_state_validate	b-operations.0.filter	reject-schema	additional
ntp_state_validate	b-operations.0.filter.match	reject-schema	additional
ntp_state_validate	b-operations.0.filter.view	reject-schema	additional
ntp_state_validate	b-operations.0.filters.0	reject-schema	additional
ntp_state_validate	b-operations.0.filters.0.match	reject-schema	additional
ntp_state_validate	b-operations.0.filters.0.view	reject-schema	additional
ntp_state_validate	b-operations.0.managementHTTP	reject-schema	additional
ntp_state_validate	b-operations.0.queryLog	reject-schema	additional
ntp_state_validate	b-operations.0.restrict	reject-schema	additional
ntp_state_validate	c-state	reject-schema	type
ntp_state_validate	c-state-bytes	accept-isError	JSON decode failed
ntp_state_validate	d-duplicate	reject-schema	type
ntp_state_validate	e-case	reject-schema	additional
ntp_state_validate	f-empty	accept-ok
ntp_state_validate	f-omit	accept-ok
ntp_status_get	a-root-unknown	reject-schema	additional
ntp_status_get	d-duplicate	reject-schema	additional
ntp_status_get	e-case	reject-schema	additional
ntp_status_get	f-empty	accept-ok
ntp_status_get	f-omit	accept-ok
ntp_version_get	a-root-unknown	reject-schema	additional
ntp_version_get	d-duplicate	reject-schema	additional
ntp_version_get	e-case	reject-schema	additional
ntp_version_get	f-empty	accept-ok
ntp_version_get	f-omit	accept-ok
ntp_views_preview	a-root-unknown	reject-schema	additional
ntp_views_preview	d-duplicate	accept-isError	unparseable ip
ntp_views_preview	e-case	reject-schema	additional
ntp_views_preview	f-empty	reject-schema	required
ntp_views_preview	f-omit	reject-schema	required
`

func TestMCPSchemaGapProbe(t *testing.T) {
	s, _ := newTestServer(t)
	ts := startHTTP(t, s)
	schemas := probeToolSchemas(t, ts)
	var lines []string
	for _, name := range schemaNames(schemas) {
		root := schemas[name]
		minimal := minimalObject(root)
		lines = append(lines, probeCase(t, ts, name, "a-root-unknown", withUnknown(minimal, "")))
		for _, path := range nestedObjectPaths(root) {
			lines = append(lines, probeCase(t, ts, name, "b-"+path, withUnknown(minimal, path)))
		}
		for _, path := range openFieldPaths(root) {
			lines = append(lines, probeCase(t, ts, name, "c-"+path, withUnknown(minimal, path)))
		}
		// state is json.RawMessage in Go. The published schema types it as a
		// byte array, so it is not an open object. Record that injection anyway.
		if name == "ntp_state_validate" {
			lines = append(lines, probeCase(t, ts, name, "c-state", withUnknown(minimal, "state")))
			// A byte array of {"zzUnknown":true} reaches decodeCandidateState
			// as JSON array text ([123,34,...]). DecodeJSON fails as a type
			// mismatch ("JSON decode failed"). It never sees the object or
			// the unknown key. Do not turn this case into an unknown-field check.
			doc := []byte(`{"zzUnknown":true}`)
			nums := make([]any, len(doc))
			for i, b := range doc {
				nums[i] = int(b)
			}
			bytesArgs := cloneMap(minimal)
			bytesArgs["state"] = nums
			lines = append(lines, probeCase(t, ts, name, "c-state-bytes", bytesArgs))
		}
		lines = append(lines, probeRaw(t, ts, name, "d-duplicate", duplicateBody(name, root)))
		lines = append(lines, probeCase(t, ts, name, "e-case", caseVariant(minimal, root)))
		lines = append(lines, probeRaw(t, ts, name, "f-omit", callBody(name, nil, false)))
		lines = append(lines, probeCase(t, ts, name, "f-empty", map[string]any{}))
	}
	sort.Strings(lines)
	got := strings.Join(lines, "\n") + "\n"
	if got != probeGolden {
		t.Fatalf("matrix changed:\n%s", got)
	}
}

func schemaNames(schemas map[string]schemaNode) []string {
	names := make([]string, 0, len(schemas))
	for name := range schemas {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// schemaTypes accepts a JSON Schema type that is either a string or an
// array of strings (nullable properties publish ["object","null"]).
type schemaTypes []string

func (s *schemaTypes) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}
	if len(b) > 0 && b[0] == '"' {
		var one string
		if err := json.Unmarshal(b, &one); err != nil {
			return err
		}
		*s = schemaTypes{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return err
	}
	*s = many
	return nil
}

func (s schemaTypes) has(want string) bool {
	for _, t := range s {
		if t == want {
			return true
		}
	}
	return false
}

func (s schemaTypes) primary() string {
	for _, t := range s {
		if t != "null" {
			return t
		}
	}
	if len(s) > 0 {
		return s[0]
	}
	return ""
}

type schemaNode struct {
	Type                 schemaTypes           `json:"type"`
	Ref                  string                `json:"$ref"`
	Properties           map[string]schemaNode `json:"properties"`
	Required             []string              `json:"required"`
	Items                *schemaNode           `json:"items"`
	AdditionalProperties any                   `json:"additionalProperties"`
	Defs                 map[string]schemaNode `json:"$defs"`
	AllOf                []schemaNode          `json:"allOf"`
	AnyOf                []schemaNode          `json:"anyOf"`
}

func (n schemaNode) resolve(root schemaNode) schemaNode {
	if n.Ref != "" {
		const prefix = "#/$defs/"
		if strings.HasPrefix(n.Ref, prefix) {
			if def, ok := root.Defs[strings.TrimPrefix(n.Ref, prefix)]; ok {
				if len(def.Defs) == 0 {
					def.Defs = root.Defs
				}
				return def.resolve(root)
			}
		}
	}
	if len(n.Properties) == 0 && len(n.AllOf) > 0 {
		merged := n
		for _, part := range n.AllOf {
			part = part.resolve(root)
			if len(part.Type) > 0 {
				merged.Type = part.Type
			}
			if merged.Properties == nil {
				merged.Properties = map[string]schemaNode{}
			}
			for k, v := range part.Properties {
				merged.Properties[k] = v
			}
			merged.Required = append(merged.Required, part.Required...)
			if part.AdditionalProperties != nil {
				merged.AdditionalProperties = part.AdditionalProperties
			}
			if part.Items != nil {
				merged.Items = part.Items
			}
		}
		return merged
	}
	return n
}

func probeToolSchemas(t *testing.T, ts *httptest.Server) map[string]schemaNode {
	t.Helper()
	status, body := postMCP(t, ts.URL, callBody("", nil, false, "tools/list"), "tools/list", "")
	payload := rpcPayload(t, status, body)
	var listed struct {
		Result struct {
			Tools []struct {
				Name        string          `json:"name"`
				InputSchema json.RawMessage `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(payload, &listed); err != nil {
		t.Fatalf("tools/list %s: %v", payload, err)
	}
	if listed.Error != nil {
		t.Fatalf("tools/list error %+v", listed.Error)
	}
	out := map[string]schemaNode{}
	for _, tool := range listed.Result.Tools {
		var node schemaNode
		if err := json.Unmarshal(tool.InputSchema, &node); err != nil {
			t.Fatalf("%s schema: %v\n%s", tool.Name, err, tool.InputSchema)
		}
		out[tool.Name] = node.resolve(node)
	}
	if len(out) != 19 {
		t.Fatalf("tools %d", len(out))
	}
	return out
}

func minimalObject(root schemaNode) map[string]any {
	return fillObject(root.resolve(root), root)
}

func fillObject(node, root schemaNode) map[string]any {
	node = node.resolve(root)
	out := map[string]any{}
	for _, name := range node.Required {
		prop, ok := node.Properties[name]
		if !ok {
			out[name] = "x"
			continue
		}
		out[name] = fillValue(prop, root)
	}
	return out
}

func fillValue(node, root schemaNode) any {
	node = node.resolve(root)
	kind := node.Type.primary()
	if kind == "array" || (node.Items != nil && kind == "") {
		if node.Items != nil {
			item := node.Items.resolve(root)
			if item.Type.has("object") || len(item.Properties) > 0 {
				return []any{fillObject(item, root)}
			}
			return []any{fillValue(item, root)}
		}
		return []any{}
	}
	if kind == "object" || len(node.Properties) > 0 {
		return fillObject(node, root)
	}
	switch kind {
	case "integer", "number":
		return 0
	case "boolean":
		return false
	default:
		return "x"
	}
}

func nestedObjectPaths(root schemaNode) []string {
	var paths []string
	walkObjects(root.resolve(root), root, "", false, &paths)
	sort.Strings(paths)
	return paths
}

func openFieldPaths(root schemaNode) []string {
	var paths []string
	walkObjects(root.resolve(root), root, "", true, &paths)
	sort.Strings(paths)
	return paths
}

func walkObjects(node, root schemaNode, path string, open bool, out *[]string) {
	walkObjectsDepth(node, root, path, open, out, 0)
}

func walkObjectsDepth(node, root schemaNode, path string, open bool, out *[]string, depth int) {
	if depth > 6 {
		return
	}
	node = node.resolve(root)
	if path != "" && isObjectSchema(node) && open == isOpenSchema(node) {
		*out = append(*out, path)
	}
	for name, prop := range node.Properties {
		next := name
		if path != "" {
			next = path + "." + name
		}
		prop = prop.resolve(root)
		if prop.Type.has("array") && prop.Items != nil {
			item := prop.Items.resolve(root)
			itemPath := next + ".0"
			walkObjectsDepth(item, root, itemPath, open, out, depth+1)
			continue
		}
		walkObjectsDepth(prop, root, next, open, out, depth+1)
	}
}

func isObjectSchema(node schemaNode) bool {
	return node.Type.has("object") || len(node.Properties) > 0
}

func isOpenSchema(node schemaNode) bool {
	// additionalProperties applies to objects. A scalar or array is not an
	// open object field. Omitted additionalProperties on an object defaults
	// to open; jsonschema-go emits false for structs, which stays closed.
	if !node.Type.has("object") && len(node.Properties) == 0 {
		return false
	}
	switch extra := node.AdditionalProperties.(type) {
	case bool:
		return extra
	case nil:
		return true
	default:
		return true
	}
}

func withUnknown(minimal map[string]any, path string) map[string]any {
	clone := cloneMap(minimal)
	if path == "" {
		clone["zzUnknown"] = true
		return clone
	}
	setPath(clone, strings.Split(path, "."), map[string]any{"zzUnknown": true})
	return clone
}

func setPath(node map[string]any, parts []string, unknown map[string]any) {
	if len(parts) == 0 {
		return
	}
	key := parts[0]
	if len(parts) == 1 {
		child, _ := node[key].(map[string]any)
		if child == nil {
			child = map[string]any{}
		}
		for k, v := range unknown {
			child[k] = v
		}
		node[key] = child
		return
	}
	if parts[1] == "0" {
		arr, _ := node[key].([]any)
		var elem map[string]any
		if len(arr) > 0 {
			elem, _ = arr[0].(map[string]any)
		}
		if elem == nil {
			elem = map[string]any{}
		}
		if len(parts) == 2 {
			elem["zzUnknown"] = true
		} else {
			setPath(elem, parts[2:], unknown)
		}
		node[key] = []any{elem}
		return
	}
	child, _ := node[key].(map[string]any)
	if child == nil {
		child = map[string]any{}
	}
	setPath(child, parts[1:], unknown)
	node[key] = child
}

func cloneMap(in map[string]any) map[string]any {
	b, err := json.Marshal(in)
	if err != nil {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func caseVariant(minimal map[string]any, root schemaNode) map[string]any {
	out := cloneMap(minimal)
	name := firstProperty(root)
	if name == "" {
		out["ZzUnknown"] = true
		return out
	}
	out[flipCase(name)] = "x"
	return out
}

func firstProperty(root schemaNode) string {
	root = root.resolve(root)
	names := make([]string, 0, len(root.Properties))
	for prop := range root.Properties {
		names = append(names, prop)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

func flipCase(name string) string {
	if name == "" {
		return "Z"
	}
	r := name[0]
	if r >= 'a' && r <= 'z' {
		return string(r-32) + name[1:]
	}
	if r >= 'A' && r <= 'Z' {
		return string(r+32) + name[1:]
	}
	return "Z" + name
}

func duplicateBody(name string, root schemaNode) string {
	key := firstProperty(root)
	if key == "" {
		key = "zz"
	}
	args := `{"` + key + `":"x","` + key + `":"y"}`
	return `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":` + jsonString(name) + `,"arguments":` + args + `,"_meta":` + metaJSON() + `}}`
}

func callBody(name string, args map[string]any, withArgs bool, method ...string) string {
	m := "tools/call"
	if len(method) > 0 {
		m = method[0]
	}
	params := map[string]any{"_meta": metaMap()}
	if m == "tools/call" {
		params["name"] = name
		if withArgs || args != nil {
			if args == nil {
				args = map[string]any{}
			}
			params["arguments"] = args
		}
	}
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  m,
		"params":  params,
	})
	if err != nil {
		panic(err)
	}
	return string(body)
}

func metaMap() map[string]any {
	return map[string]any{
		"io.modelcontextprotocol/protocolVersion":    ProtocolVersion,
		"io.modelcontextprotocol/clientInfo":         map[string]any{"name": "probe", "version": "dev"},
		"io.modelcontextprotocol/clientCapabilities": map[string]any{},
	}
}

func metaJSON() string {
	b, _ := json.Marshal(metaMap())
	return string(b)
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func probeCase(t *testing.T, ts *httptest.Server, tool, kind string, args map[string]any) string {
	t.Helper()
	status, body := postMCP(t, ts.URL, callBody(tool, args, true), "tools/call", tool)
	return tool + "\t" + kind + "\t" + classifyRPC(status, rpcPayload(t, status, body))
}

func probeRaw(t *testing.T, ts *httptest.Server, tool, kind, body string) string {
	t.Helper()
	status, raw := postMCP(t, ts.URL, body, "tools/call", tool)
	return tool + "\t" + kind + "\t" + classifyRPC(status, rpcPayload(t, status, raw))
}

func classifyRPC(status int, payload []byte) string {
	var doc struct {
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		Result *struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(payload, &doc); err != nil {
		return "unparsed"
	}
	if doc.Error != nil {
		kind := "reject-other"
		if doc.Error.Code == -32602 {
			kind = "reject-schema"
		}
		return kind + "\t" + itoa(doc.Error.Code) + "\t" + oneLine(doc.Error.Message, 160)
	}
	if doc.Result != nil && doc.Result.IsError {
		msg := ""
		if len(doc.Result.Content) > 0 {
			msg = doc.Result.Content[0].Text
		}
		// SDK v1.7.0 packs input-schema failure into the tool result
		// (server.go toolForErr: SetError("validating \"arguments\"")).
		// The handler does not run. That is a schema reject, not an accept.
		if strings.Contains(msg, `validating "arguments"`) {
			return schemaReason(msg)
		}
		return "accept-isError\t" + oneLine(msg, 120)
	}
	if doc.Result != nil {
		return "accept-ok"
	}
	return "unparsed-status-" + itoa(status)
}

func schemaReason(msg string) string {
	switch {
	case strings.Contains(msg, "unexpected additional properties"):
		return "reject-schema\tadditional"
	case strings.Contains(msg, "missing properties"):
		return "reject-schema\trequired"
	case strings.Contains(msg, "has type"):
		return "reject-schema\ttype"
	default:
		return "reject-schema\tother\t" + oneLine(msg, 120)
	}
}

func oneLine(msg string, n int) string {
	if i := strings.IndexAny(msg, "\r\n"); i >= 0 {
		msg = msg[:i]
	}
	msg = strings.Join(strings.Fields(msg), " ")
	if len(msg) > n {
		msg = msg[:n]
	}
	return msg
}

func itoa(n int) string {
	return strings.TrimPrefix(strings.ReplaceAll(jsonNumber(n), ",", ""), "+")
}

func jsonNumber(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func postMCP(t *testing.T, url, body, method, tool string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set("Mcp-Protocol-Version", ProtocolVersion)
	req.Header.Set("Mcp-Method", method)
	if tool != "" {
		req.Header.Set("Mcp-Name", tool)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, string(b)
}

func rpcPayload(t *testing.T, status int, body string) []byte {
	t.Helper()
	trim := strings.TrimSpace(body)
	if strings.HasPrefix(trim, "{") {
		return []byte(trim)
	}
	var data strings.Builder
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimRight(line, "\r")
		if rest, ok := strings.CutPrefix(line, "data:"); ok {
			data.WriteString(strings.TrimPrefix(rest, " "))
		}
	}
	if data.Len() == 0 {
		t.Fatalf("status %d body %s", status, body)
	}
	return []byte(data.String())
}
