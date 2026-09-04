package mcp

import (
	"fmt"
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/hilather/go-lab-ntp/internal/model"
)

// optionalViewJSON are ViewSpec wire fields whose Go zero is a designed
// JSON/typed omit (REST apply and MCP unmarshal). YAML document decode still
// materializes precision -20. jsonschema-go marks non-omitempty fields
// required; the MCP adapter must not invent a stricter gate than validateView.
var optionalViewJSON = map[string]struct{}{
	"precision":      {},
	"rootDelay":      {},
	"rootDispersion": {},
	"jitter":         {},
	"offset":         {},
	"leap":           {},
	"refid":          {},
}

func inferToolInput[In any]() *jsonschema.Schema {
	view, err := jsonschema.For[model.ViewSpec](nil)
	if err != nil {
		panic(fmt.Errorf("mcp: infer ViewSpec schema: %w", err))
	}
	// CloneSchemas shares Required; copy before mutate, then insert.
	kept := make([]string, 0, len(view.Required))
	for _, name := range view.Required {
		if _, drop := optionalViewJSON[name]; !drop {
			kept = append(kept, name)
		}
	}
	view.Required = kept
	s, err := jsonschema.For[In](&jsonschema.ForOptions{
		TypeSchemas: map[reflect.Type]*jsonschema.Schema{
			reflect.TypeOf(model.ViewSpec{}): view,
		},
	})
	if err != nil {
		panic(fmt.Errorf("mcp: infer tool input schema: %w", err))
	}
	return s
}
