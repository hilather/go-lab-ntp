package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/hilather/go-lab-ntp/internal/app"
	"github.com/hilather/go-lab-ntp/internal/buildinfo"
	"github.com/hilather/go-lab-ntp/internal/capabilities"
	"github.com/hilather/go-lab-ntp/internal/config"
	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/model"
	"github.com/hilather/go-lab-ntp/internal/observability"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) registerTools() {
	addTool(s, "ntp_version_get", versionDesc, false, true, func(ctx context.Context, actor app.Actor, _ emptyIn) (any, error) {
		_ = actor
		return fromVersion(buildinfo.Current()), nil
	})
	addTool(s, "ntp_capabilities_get", capDesc, false, true, func(ctx context.Context, actor app.Actor, _ emptyIn) (any, error) {
		_ = actor
		return fromCapabilities(), nil
	})
	addTool(s, "ntp_status_get", statusDesc, false, true, func(ctx context.Context, actor app.Actor, _ emptyIn) (any, error) {
		return s.svc.Status(ctx, actor)
	})
	addTool(s, "ntp_schema_get", schemaDesc, false, true, func(ctx context.Context, actor app.Actor, _ emptyIn) (any, error) {
		_ = actor
		b, err := config.SchemaBytes()
		if err != nil {
			return nil, domainerr.Internal("schema unavailable")
		}
		var doc any
		if err := json.Unmarshal(b, &doc); err != nil {
			return nil, domainerr.Internal("internal error")
		}
		return doc, nil
	})
	addTool(s, "ntp_features_list", featuresDesc, false, true, func(ctx context.Context, actor app.Actor, _ emptyIn) (any, error) {
		return s.svc.Features(ctx, actor)
	})
	addTool(s, "ntp_state_get", stateGetDesc, false, true, func(ctx context.Context, actor app.Actor, _ emptyIn) (any, error) {
		v, err := s.svc.GetState(ctx, actor)
		if err != nil {
			return nil, err
		}
		return fromStateView(v)
	})
	addTool(s, "ntp_state_validate", validateDesc, false, true, func(ctx context.Context, actor app.Actor, in validateIn) (any, error) {
		vin, err := in.toValidate()
		if err != nil {
			return nil, asDomain(err)
		}
		p, err := s.svc.Validate(ctx, actor, vin)
		if err != nil {
			return nil, err
		}
		return fromPlan(p), nil
	})
	addTool(s, "ntp_change_plan", planDesc, false, true, func(ctx context.Context, actor app.Actor, in changeIn) (any, error) {
		p, err := s.svc.Plan(ctx, actor, in.toChange())
		if err != nil {
			return nil, err
		}
		return fromPlan(p), nil
	})
	addTool(s, "ntp_change_apply", applyDesc, true, true, func(ctx context.Context, actor app.Actor, in changeIn) (any, error) {
		r, err := s.svc.Apply(ctx, actor, in.toChange())
		if err != nil {
			return nil, err
		}
		return fromApply(r), nil
	})
	addTool(s, "ntp_state_export", exportDesc, false, true, func(ctx context.Context, actor app.Actor, in exportIn) (any, error) {
		format := app.ExportYAML
		switch strings.ToLower(in.Format) {
		case "", "yaml", "yml":
		case "json":
			format = app.ExportJSON
		default:
			return nil, domainerr.ValidationFailed("unknown export format",
				domainerr.FieldViolation{Path: "format", Code: "invalid_value", Message: "format must be yaml or json"})
		}
		exp, err := s.svc.Export(ctx, actor, format)
		if err != nil {
			return nil, err
		}
		return fromExport(exp), nil
	})
	addTool(s, "ntp_state_reset", resetDesc, true, false, func(ctx context.Context, actor app.Actor, in resetIn) (any, error) {
		r, err := s.svc.Reset(ctx, actor, app.ResetIn{Reason: in.Reason})
		if err != nil {
			return nil, err
		}
		return fromApply(r), nil
	})
	addTool(s, "ntp_filters_list", filtersListDesc, false, true, func(ctx context.Context, actor app.Actor, _ emptyIn) (any, error) {
		return s.svc.ListFilters(ctx, actor)
	})
	addTool(s, "ntp_filters_get", filtersGetDesc, false, true, func(ctx context.Context, actor app.Actor, in nameIn) (any, error) {
		if err := requireName(in.Name); err != nil {
			return nil, err
		}
		return s.svc.GetFilter(ctx, actor, in.Name)
	})
	addTool(s, "ntp_filters_put", filtersPutDesc, true, true, func(ctx context.Context, actor app.Actor, in putFilterIn) (any, error) {
		if err := requireName(in.Name); err != nil {
			return nil, err
		}
		f := model.Filter{Name: in.Name}
		if in.Filter != nil {
			f = *in.Filter
			if f.Name == "" {
				f.Name = in.Name
			}
		}
		return s.svc.PutFilter(ctx, actor, app.PutFilterIn{
			ExpectedRevision: model.Revision(in.ExpectedRevision),
			IdempotencyKey:   in.IdempotencyKey,
			Reason:           in.Reason,
			Filter:           f,
		})
	})
	addTool(s, "ntp_filters_delete", filtersDeleteDesc, true, true, func(ctx context.Context, actor app.Actor, in deleteFilterIn) (any, error) {
		if err := requireName(in.Name); err != nil {
			return nil, err
		}
		return s.svc.DeleteFilter(ctx, actor, in.Name, app.DeleteIn{
			ExpectedRevision: model.Revision(in.ExpectedRevision),
			IdempotencyKey:   in.IdempotencyKey,
			Reason:           in.Reason,
		})
	})
	addTool(s, "ntp_views_preview", previewDesc, false, true, func(ctx context.Context, actor app.Actor, in previewIn) (any, error) {
		p, err := s.svc.Preview(ctx, actor, in.IP)
		if err != nil {
			return nil, err
		}
		return fromPreview(p), nil
	})
	addTool(s, "ntp_queries_list", queriesDesc, false, true, func(ctx context.Context, actor app.Actor, in listIn) (any, error) {
		return s.svc.ListQueries(ctx, actor, app.Page{Limit: in.Limit, Cursor: in.Cursor})
	})
	addTool(s, "ntp_audit_list", auditQueryDesc, false, true, func(ctx context.Context, actor app.Actor, in auditQueryIn) (any, error) {
		list, err := s.svc.QueryAudit(ctx, actor, app.AuditQuery{Limit: in.Limit})
		if err != nil {
			return nil, err
		}
		return fromAuditList(list), nil
	})
	addTool(s, "ntp_audit_get", auditGetDesc, false, true, func(ctx context.Context, actor app.Actor, in idIn) (any, error) {
		if in.ID == "" {
			return nil, domainerr.ValidationFailed("id is required",
				domainerr.FieldViolation{Path: "id", Code: "required", Message: "id is required"})
		}
		return s.svc.GetAudit(ctx, actor, in.ID)
	})
}

func addTool[In any](s *Server, name, desc string, mutating, idempotent bool, h func(context.Context, app.Actor, In) (any, error)) {
	caps := capabilities.LookupTool(name)
	title := name
	if len(caps) > 0 && caps[0].Title != "" {
		title = caps[0].Title
		if desc == "" {
			desc = caps[0].Description
		}
	}
	readOnly := !mutating
	ann := &sdk.ToolAnnotations{
		Title:           title,
		ReadOnlyHint:    readOnly,
		IdempotentHint:  idempotent,
		DestructiveHint: boolPtr(mutating && !idempotent),
		OpenWorldHint:   boolPtr(false),
	}
	sdk.AddTool(s.sdk, &sdk.Tool{
		Name:        name,
		Title:       title,
		Description: desc,
		Annotations: ann,
		InputSchema: inferToolInput[In](),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in In) (*sdk.CallToolResult, any, error) {
		if err := ctx.Err(); err != nil {
			return toolErrorResult(canceledError(err)), nil, nil
		}
		actor := s.actorFrom(ctx)
		if err := s.authorizeTool(actor, name); err != nil {
			return toolErrorResult(err), nil, nil
		}
		if s.metrics != nil {
			s.metrics.Inc(observability.MetricMCPCallsTotal, map[string]string{"capability": name}, 1)
		}
		out, err := h(ctx, actor, in)
		if err != nil {
			return toolErrorResult(err), nil, nil
		}
		structured, err := asStructured(out)
		if err != nil {
			return nil, nil, rpcError(domainerr.Internal("internal error"))
		}
		return nil, structured, nil
	})
}

func boolPtr(v bool) *bool { return &v }

func canceledError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return domainerr.Timeout("request deadline exceeded")
	}
	return domainerr.Internal("request canceled")
}

const (
	versionDesc       = "Read-only. Build and protocol versions (MCP " + ProtocolVersion + ")."
	capDesc           = "Read-only. Capability list and protocol metadata."
	statusDesc        = "Read-only. Listeners, revisions, hostTime, and ready."
	schemaDesc        = "Read-only. Published v1alpha1 config JSON Schema."
	featuresDesc      = "Read-only. Frozen live vs reset-only catalog."
	stateGetDesc      = "Read-only. Redacted spec plus revision metadata."
	validateDesc      = "Read-only dry-run. Validate a candidate document and/or operations without writing."
	planDesc          = "Read-only dry-run. Plan operations against the active snapshot."
	applyDesc         = "State-changing. Apply operations with expectedRevision. High-impact."
	exportDesc        = "Read-only. Canonical desired-state export plus drift material."
	resetDesc         = "State-changing, high-impact. Reread the bootstrap mount, wipe the query log, and swap. Never writes the file."
	filtersListDesc   = "Read-only. List first-match filters in document order."
	filtersGetDesc    = "Read-only. Get one filter by name."
	filtersPutDesc    = "State-changing. Upsert a filter by name."
	filtersDeleteDesc = "State-changing. Remove a filter by name."
	previewDesc       = "Read-only. Compute served virtual time for an IP without sending NTP."
	queriesDesc       = "Read-only. Last-N NTP query ring."
	auditQueryDesc    = "Read-only. Query recent in-memory audit events."
	auditGetDesc      = "Read-only. Get one audit event by id."
)
