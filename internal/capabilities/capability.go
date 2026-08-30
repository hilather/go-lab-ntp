package capabilities

// SchemaRef names an OpenAPI/JSON Schema component.
type SchemaRef struct {
	Name string `json:"name,omitempty"`
}

// RESTBinding is one HTTP method+path. Path templates are frozen spellings.
type RESTBinding struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

// MCPBinding is the tool and optional resource surface for one capability.
type MCPBinding struct {
	Tools     []string `json:"tools,omitempty"`
	Resources []string `json:"resources,omitempty"`
}

// Capability is one frozen row of the REST↔MCP table.
type Capability struct {
	ID               ID            `json:"id"`
	Title            string        `json:"title"`
	Version          string        `json:"version"`
	Description      string        `json:"description"`
	InputSchema      *SchemaRef    `json:"inputSchema,omitempty"`
	OutputSchema     *SchemaRef    `json:"outputSchema,omitempty"`
	RequiredScopes   []string      `json:"requiredScopes,omitempty"`
	Mutating         bool          `json:"mutating"`
	Idempotent       bool          `json:"idempotent"`
	RESTOnly         bool          `json:"restOnly,omitempty"`
	DifferentBinding bool          `json:"differentBinding,omitempty"`
	REST             []RESTBinding `json:"rest"`
	MCP              *MCPBinding   `json:"mcp"`
	ServiceMethods   []string      `json:"serviceMethods,omitempty"`
}

// RESTRef is Method plus Path, used as a lookup key ("GET /v1/state").
func (b RESTBinding) RESTRef() string {
	if b.Method == "" {
		return b.Path
	}
	return b.Method + " " + b.Path
}
