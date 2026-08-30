package audit

import (
	"bytes"
	"encoding/json"
	"strings"
)

const redacted = "[redacted]"

var secretKeys = map[string]bool{
	"secret":        true,
	"secretref":     true,
	"secretfile":    true,
	"token":         true,
	"password":      true,
	"authorization": true,
	"bearer":        true,
	"credential":    true,
	"credentials":   true,
	"apikey":        true,
	"api_key":       true,
	"privatekey":    true,
	"private_key":   true,
	"cookie":        true,
}

func secretPath(path string) bool {
	p := strings.ToLower(path)
	for key := range secretKeys {
		if p == key || strings.HasSuffix(p, "."+key) || strings.HasSuffix(p, "/"+key) {
			return true
		}
	}
	return false
}

func redactJSON(raw []byte) []byte {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return raw
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var tree any
	if err := dec.Decode(&tree); err != nil {
		if containsPrivateKey(string(raw)) {
			return []byte(`"` + redacted + `"`)
		}
		return raw
	}
	out, err := json.Marshal(redactTree(tree))
	if err != nil {
		return raw
	}
	return out
}

func redactTree(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, child := range x {
			if secretKeys[strings.ToLower(k)] {
				out[k] = redacted
				continue
			}
			out[k] = redactTree(child)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, child := range x {
			out[i] = redactTree(child)
		}
		return out
	case string:
		if containsPrivateKey(x) {
			return redacted
		}
		return x
	default:
		return v
	}
}

func redactText(s string) string {
	if containsPrivateKey(s) {
		return redacted
	}
	return s
}

func containsPrivateKey(s string) bool {
	return strings.Contains(s, "BEGIN ") && strings.Contains(s, "PRIVATE")
}
