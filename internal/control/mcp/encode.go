package mcp

import (
	"bytes"
	"encoding/json"

	"github.com/hilather/go-lab-ntp/internal/config"
)

func marshalAPI(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var tree any
	if err := dec.Decode(&tree); err != nil {
		return nil, err
	}
	config.FormatWireTree(tree)
	return json.Marshal(tree)
}

func asStructured(v any) (any, error) {
	raw, err := marshalAPI(v)
	if err != nil {
		return nil, err
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}
