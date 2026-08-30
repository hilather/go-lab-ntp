package mcp

import (
	"context"

	"github.com/hilather/go-lab-ntp/internal/app"
	"github.com/hilather/go-lab-ntp/internal/config"
	"github.com/hilather/go-lab-ntp/internal/domainerr"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) registerResources() {
	h := s.readResource
	s.sdk.AddResource(&sdk.Resource{
		URI: "labntp://capabilities", Name: "capabilities",
		Description: "Capability list and protocol metadata.",
		MIMEType:    "application/json",
	}, h)
	s.sdk.AddResource(&sdk.Resource{
		URI: "labntp://status", Name: "status",
		Description: "Listeners, revisions, hostTime, and ready.",
		MIMEType:    "application/json",
	}, h)
	s.sdk.AddResource(&sdk.Resource{
		URI: "labntp://schema/config", Name: "schema-config",
		Description: "Published v1alpha1 config JSON Schema.",
		MIMEType:    "application/schema+json",
	}, h)
	s.sdk.AddResource(&sdk.Resource{
		URI: "labntp://features", Name: "features",
		Description: "Frozen live vs reset-only catalog.",
		MIMEType:    "application/json",
	}, h)
	s.sdk.AddResource(&sdk.Resource{
		URI: "labntp://state", Name: "state",
		Description: "Redacted spec plus revision metadata (same as GET /v1/state).",
		MIMEType:    "application/json",
	}, h)
	s.sdk.AddResource(&sdk.Resource{
		URI: "labntp://filters", Name: "filters",
		Description: "First-match filters in document order.",
		MIMEType:    "application/json",
	}, h)
	s.sdk.AddResource(&sdk.Resource{
		URI: "labntp://audit/recent", Name: "audit-recent",
		Description: "Recent in-memory audit events.",
		MIMEType:    "application/json",
	}, h)
}

func (s *Server) readResource(ctx context.Context, req *sdk.ReadResourceRequest) (*sdk.ReadResourceResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, rpcError(domainerr.Internal("request canceled"))
	}
	actor := s.actorFrom(ctx)
	uri := ""
	if req != nil && req.Params != nil {
		uri = req.Params.URI
	}
	if err := s.authorizeResource(actor, uri); err != nil {
		return nil, rpcError(err)
	}
	body, mime, err := s.resourceBody(ctx, actor, uri)
	if err != nil {
		return nil, rpcError(err)
	}
	return &sdk.ReadResourceResult{
		Contents: []*sdk.ResourceContents{{
			URI:      uri,
			MIMEType: mime,
			Text:     string(body),
		}},
	}, nil
}

func (s *Server) resourceBody(ctx context.Context, actor app.Actor, uri string) ([]byte, string, error) {
	switch uri {
	case "labntp://capabilities":
		b, err := marshalAPI(fromCapabilities())
		return b, "application/json", err
	case "labntp://status":
		st, err := s.svc.Status(ctx, actor)
		if err != nil {
			return nil, "", err
		}
		b, err := marshalAPI(st)
		return b, "application/json", err
	case "labntp://schema/config":
		b, err := config.SchemaBytes()
		return b, "application/schema+json", err
	case "labntp://features":
		list, err := s.svc.Features(ctx, actor)
		if err != nil {
			return nil, "", err
		}
		b, err := marshalAPI(list)
		return b, "application/json", err
	case "labntp://state":
		v, err := s.svc.GetState(ctx, actor)
		if err != nil {
			return nil, "", err
		}
		b, err := marshalAPI(v)
		return b, "application/json", err
	case "labntp://filters":
		list, err := s.svc.ListFilters(ctx, actor)
		if err != nil {
			return nil, "", err
		}
		b, err := marshalAPI(list)
		return b, "application/json", err
	case "labntp://audit/recent":
		list, err := s.svc.QueryAudit(ctx, actor, app.AuditQuery{})
		if err != nil {
			return nil, "", err
		}
		b, err := marshalAPI(list)
		return b, "application/json", err
	default:
		return nil, "", domainerr.NotFound("not found")
	}
}
