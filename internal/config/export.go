package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/model"
	"gopkg.in/yaml.v3"
)

// CanonicalJSON returns compact canonical JSON of a normalized copy of st.
func CanonicalJSON(st *model.State) ([]byte, error) {
	n, _, err := Normalize(st)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(n)
	if err != nil {
		return nil, domainerr.Internal("canonical json: " + err.Error())
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var tree any
	if err := dec.Decode(&tree); err != nil {
		return nil, domainerr.Internal("canonical json tree: " + err.Error())
	}
	convertDurationNumbersToStrings(tree)
	convertByteSizeNumbersToStrings(tree)
	out, err := json.Marshal(tree)
	if err != nil {
		return nil, domainerr.Internal("canonical json encode: " + err.Error())
	}
	return out, nil
}

// CanonicalYAML returns YAML of the canonical JSON tree. Comments are dropped.
func CanonicalYAML(st *model.State) ([]byte, error) {
	n, _, err := Normalize(st)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(n)
	if err != nil {
		return nil, domainerr.Internal("canonical yaml: " + err.Error())
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var tree any
	if err := dec.Decode(&tree); err != nil {
		return nil, domainerr.Internal("canonical yaml tree: " + err.Error())
	}
	convertDurationNumbersToStrings(tree)
	convertByteSizeNumbersToStrings(tree)
	node := toYAMLNode(tree)
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(node); err != nil {
		return nil, domainerr.Internal("canonical yaml encode: " + err.Error())
	}
	_ = enc.Close()
	return buf.Bytes(), nil
}

// DecodeJSON decodes a JSON document with unknown-field rejection.
func DecodeJSON(data []byte) (*model.State, error) {
	return decodeJSON(data)
}

// FormatWireTree rewrites duration and byte-size numbers to canonical strings.
func FormatWireTree(v any) {
	convertDurationNumbersToStrings(v)
	convertByteSizeNumbersToStrings(v)
}

// CoerceWireTree rewrites duration and byte-size strings on an inbound JSON tree.
func CoerceWireTree(v any) []domainerr.FieldViolation {
	vs := convertDurations(v, "")
	vs = append(vs, convertByteSizes(v, "")...)
	return vs
}

// Revision returns "sha256:" plus lowercase hex of SHA-256(canonical JSON).
func Revision(st *model.State) (model.Revision, error) {
	b, err := CanonicalJSON(st)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return model.Revision(model.RevisionPrefix + hex.EncodeToString(sum[:])), nil
}

func toYAMLNode(v any) *yaml.Node {
	switch x := v.(type) {
	case nil:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}
	case bool:
		if x {
			return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"}
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "false"}
	case string:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: x}
	case json.Number:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: x.String()}
	case float64:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: strconv.FormatFloat(x, 'g', -1, 64)}
	case int, int64:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: fmt.Sprint(x)}
	case []any:
		n := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, c := range x {
			n.Content = append(n.Content, toYAMLNode(c))
		}
		return n
	case map[string]any:
		n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k})
			n.Content = append(n.Content, toYAMLNode(x[k]))
		}
		return n
	default:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(fmt.Sprint(x))}
	}
}
