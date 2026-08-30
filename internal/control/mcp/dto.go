package mcp

import (
	"encoding/json"

	"github.com/hilather/go-lab-ntp/internal/app"
	"github.com/hilather/go-lab-ntp/internal/buildinfo"
	"github.com/hilather/go-lab-ntp/internal/capabilities"
	"github.com/hilather/go-lab-ntp/internal/config"
	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/model"
)

type emptyIn struct{}

type idIn struct {
	ID string `json:"id"`
}

type nameIn struct {
	Name string `json:"name"`
}

type exportIn struct {
	Format string `json:"format,omitempty"`
}

type changeIn struct {
	ExpectedRevision string            `json:"expectedRevision,omitempty"`
	IdempotencyKey   string            `json:"idempotencyKey,omitempty"`
	Reason           string            `json:"reason,omitempty"`
	Force            bool              `json:"force,omitempty"`
	Operations       []model.Operation `json:"operations,omitempty"`
}

type validateIn struct {
	State      json.RawMessage   `json:"state,omitempty"`
	Operations []model.Operation `json:"operations,omitempty"`
}

type resetIn struct {
	Reason string `json:"reason,omitempty"`
}

type previewIn struct {
	IP string `json:"ip"`
}

type putFilterIn struct {
	Name             string        `json:"name"`
	ExpectedRevision string        `json:"expectedRevision,omitempty"`
	IdempotencyKey   string        `json:"idempotencyKey,omitempty"`
	Reason           string        `json:"reason,omitempty"`
	Filter           *model.Filter `json:"filter"`
}

type deleteFilterIn struct {
	Name             string `json:"name"`
	ExpectedRevision string `json:"expectedRevision,omitempty"`
	IdempotencyKey   string `json:"idempotencyKey,omitempty"`
	Reason           string `json:"reason,omitempty"`
}

type listIn struct {
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type auditQueryIn struct {
	Limit int `json:"limit,omitempty"`
}

func (in changeIn) toChange() app.ChangeIn {
	return app.ChangeIn{
		ExpectedRevision: model.Revision(in.ExpectedRevision),
		IdempotencyKey:   in.IdempotencyKey,
		Reason:           in.Reason,
		Force:            in.Force,
		Operations:       in.Operations,
	}
}

func (in validateIn) toValidate() (app.ValidateIn, error) {
	st, err := decodeCandidateState(in.State)
	if err != nil {
		return app.ValidateIn{}, err
	}
	return app.ValidateIn{State: st, Operations: in.Operations}, nil
}

func decodeCandidateState(raw json.RawMessage) (*model.State, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	return config.DecodeJSON(raw)
}

func fromVersion(info buildinfo.Info) map[string]any {
	return map[string]any{
		"version":   info.Version,
		"commit":    info.Commit,
		"buildTime": info.BuildTime,
		"protocols": map[string]any{
			"configAPI": info.Protocols.ConfigAPI,
			"rest":      info.Protocols.REST,
			"mcp":       info.Protocols.MCP,
		},
	}
}

func fromCapabilities() map[string]any {
	src := capabilities.DiscoveryList()
	items := make([]map[string]any, 0, len(src))
	for _, d := range src {
		items = append(items, map[string]any{
			"name": d.Name, "version": d.Version, "description": d.Description,
			"mutating": d.Mutating, "idempotent": d.Idempotent,
		})
	}
	return map[string]any{"capabilities": items}
}

func fromPlan(p *app.Plan) any {
	if p == nil {
		return map[string]any{"diff": []any{}}
	}
	return p
}

func fromApply(r *app.ApplyResult) any {
	return r
}

func fromStateView(v *app.StateView) (any, error) {
	return v, nil
}

func fromExport(exp *app.Export) any {
	return map[string]any{
		"format":            string(exp.Format),
		"revision":          string(exp.Revision),
		"bootstrapRevision": string(exp.BootstrapRevision),
		"drifted":           exp.Drifted,
		"body":              string(exp.Body),
		"humanDiff":         exp.HumanDiff,
	}
}

func fromPreview(p *app.Preview) any {
	return p
}

func fromAuditList(list *app.AuditList) any {
	if list == nil {
		return map[string]any{"events": []any{}}
	}
	return list
}

func requireName(name string) error {
	if name == "" {
		return domainerr.ValidationFailed("name is required",
			domainerr.FieldViolation{Path: "name", Code: "required", Message: "name is required"})
	}
	return nil
}
