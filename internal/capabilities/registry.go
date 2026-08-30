package capabilities

import "fmt"

type indexes struct {
	all    []Capability
	byID   map[ID]Capability
	byREST map[string]Capability
	byTool map[string][]Capability
	byRes  map[string]Capability
}

func newIndexes() indexes {
	all := catalog()
	idx := indexes{
		all:    all,
		byID:   make(map[ID]Capability, len(all)),
		byREST: make(map[string]Capability),
		byTool: make(map[string][]Capability),
		byRes:  make(map[string]Capability),
	}
	for _, c := range all {
		if _, ok := idx.byID[c.ID]; ok {
			panic("capabilities: duplicate id " + string(c.ID))
		}
		idx.byID[c.ID] = c
		for _, b := range c.REST {
			key := b.RESTRef()
			if _, ok := idx.byREST[key]; ok {
				panic("capabilities: duplicate REST binding " + key)
			}
			idx.byREST[key] = c
		}
		if c.MCP == nil {
			continue
		}
		for _, t := range c.MCP.Tools {
			idx.byTool[t] = append(idx.byTool[t], c)
		}
		for _, r := range c.MCP.Resources {
			if _, ok := idx.byRes[r]; ok {
				continue
			}
			idx.byRes[r] = c
		}
	}
	return idx
}

var registry = newIndexes()

func init() {
	if err := ValidateCatalog(); err != nil {
		panic(err)
	}
}

func cloneCapability(c Capability) Capability {
	c.RequiredScopes = append([]string(nil), c.RequiredScopes...)
	c.REST = append([]RESTBinding(nil), c.REST...)
	c.ServiceMethods = append([]string(nil), c.ServiceMethods...)
	if c.MCP != nil {
		mcp := *c.MCP
		mcp.Tools = append([]string(nil), c.MCP.Tools...)
		mcp.Resources = append([]string(nil), c.MCP.Resources...)
		c.MCP = &mcp
	}
	if c.InputSchema != nil {
		s := *c.InputSchema
		c.InputSchema = &s
	}
	if c.OutputSchema != nil {
		s := *c.OutputSchema
		c.OutputSchema = &s
	}
	return c
}

// All returns a copy of the frozen table in documented order.
func All() []Capability {
	out := make([]Capability, len(registry.all))
	for i, c := range registry.all {
		out[i] = cloneCapability(c)
	}
	return out
}

// Lookup returns the capability with id.
func Lookup(id ID) (Capability, bool) {
	c, ok := registry.byID[id]
	if !ok {
		return Capability{}, false
	}
	return cloneCapability(c), true
}

// LookupREST returns the capability bound to method+path.
func LookupREST(method, path string) (Capability, bool) {
	c, ok := registry.byREST[RESTBinding{Method: method, Path: path}.RESTRef()]
	if !ok {
		return Capability{}, false
	}
	return cloneCapability(c), true
}

// LookupTool returns capabilities that expose tool.
func LookupTool(name string) []Capability {
	src := registry.byTool[name]
	out := make([]Capability, len(src))
	for i, c := range src {
		out[i] = cloneCapability(c)
	}
	return out
}

// LookupResource returns the capability bound to a labntp:// URI template.
func LookupResource(uri string) (Capability, bool) {
	c, ok := registry.byRes[uri]
	if !ok {
		return Capability{}, false
	}
	return cloneCapability(c), true
}

// Tools returns unique MCP tool names in first-seen catalog order.
func Tools() []string {
	var out []string
	seen := make(map[string]bool)
	for _, c := range registry.all {
		if c.MCP == nil {
			continue
		}
		for _, t := range c.MCP.Tools {
			if seen[t] {
				continue
			}
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

// Resources returns unique MCP resource URI templates in catalog order.
func Resources() []string {
	var out []string
	seen := make(map[string]bool)
	for _, c := range registry.all {
		if c.MCP == nil {
			continue
		}
		for _, r := range c.MCP.Resources {
			if seen[r] {
				continue
			}
			seen[r] = true
			out = append(out, r)
		}
	}
	return out
}

// Discovery describes one name an agent should see on GET /v1/capabilities.
type Discovery struct {
	Name        string
	Version     string
	Description string
	Mutating    bool
	Idempotent  bool
}

// DiscoveryList is the agent-facing name list derived from the registry.
func DiscoveryList() []Discovery {
	var out []Discovery
	seen := make(map[string]bool)
	for _, c := range registry.all {
		if c.RESTOnly || c.MCP == nil || len(c.MCP.Tools) == 0 {
			name := string(c.ID)
			if seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, Discovery{
				Name:        name,
				Version:     c.Version,
				Description: c.Description,
				Mutating:    c.Mutating,
				Idempotent:  c.Idempotent,
			})
			continue
		}
		for _, t := range c.MCP.Tools {
			if seen[t] {
				continue
			}
			seen[t] = true
			out = append(out, Discovery{
				Name:        t,
				Version:     c.Version,
				Description: c.Description,
				Mutating:    c.Mutating,
				Idempotent:  c.Idempotent,
			})
		}
	}
	return out
}

// SessionCapability reports whether id is a UI session row.
func SessionCapability(id ID) bool {
	switch id {
	case SessionCreate, SessionDelete, SessionGet:
		return true
	default:
		return false
	}
}

// ValidateCatalog reports structural defects. init panics if this fails.
func ValidateCatalog() error {
	all := All()
	if len(all) != TableRowCount {
		return fmt.Errorf("catalog has %d rows, want %d", len(all), TableRowCount)
	}
	for _, c := range all {
		if c.ID == "" || c.Title == "" || c.Version == "" {
			return fmt.Errorf("capability %q missing id/title/version", c.ID)
		}
		if len(c.REST) == 0 {
			return fmt.Errorf("%s: no REST binding", c.ID)
		}
		if c.RESTOnly {
			if c.MCP != nil && (len(c.MCP.Tools) > 0 || len(c.MCP.Resources) > 0) {
				return fmt.Errorf("%s: REST-only row must not declare MCP tools/resources", c.ID)
			}
			continue
		}
		if c.DifferentBinding {
			if c.MCP == nil || (len(c.MCP.Tools) == 0 && len(c.MCP.Resources) == 0) {
				return fmt.Errorf("%s: different-binding row needs MCP tools or resources", c.ID)
			}
			continue
		}
		if c.MCP == nil || len(c.MCP.Tools) == 0 {
			return fmt.Errorf("%s: missing MCP tool", c.ID)
		}
		for _, t := range c.MCP.Tools {
			if len(t) >= 7 && t[:7] == "labntp_" {
				return fmt.Errorf("%s: tool %s must use ntp_ prefix, not labntp_", c.ID, t)
			}
		}
	}
	return nil
}
