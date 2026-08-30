package app

import (
	"context"

	"github.com/hilather/go-lab-ntp/internal/buildinfo"
	"github.com/hilather/go-lab-ntp/internal/model"
)

// Service is the HTTP-less capability surface. REST and MCP must call these
// methods rather than implementing mutation or query logic.
type Service interface {
	Version(ctx context.Context, actor Actor) (*buildinfo.Info, error)
	Capabilities(ctx context.Context, actor Actor) (*CapabilityView, error)
	Status(ctx context.Context, actor Actor) (*Status, error)
	ConfigSchema(ctx context.Context, actor Actor) ([]byte, error)
	Features(ctx context.Context, actor Actor) (*FeatureList, error)

	GetState(ctx context.Context, actor Actor) (*StateView, error)
	Validate(ctx context.Context, actor Actor, in ValidateIn) (*Plan, error)
	Plan(ctx context.Context, actor Actor, in ChangeIn) (*Plan, error)
	Apply(ctx context.Context, actor Actor, in ChangeIn) (*ApplyResult, error)
	Export(ctx context.Context, actor Actor, format ExportFormat) (*Export, error)
	Reset(ctx context.Context, actor Actor, in ResetIn) (*ApplyResult, error)

	ListFilters(ctx context.Context, actor Actor) (*FilterList, error)
	GetFilter(ctx context.Context, actor Actor, name string) (*model.Filter, error)
	PutFilter(ctx context.Context, actor Actor, in PutFilterIn) (*ApplyResult, error)
	DeleteFilter(ctx context.Context, actor Actor, name string, in DeleteIn) (*ApplyResult, error)

	Preview(ctx context.Context, actor Actor, ip string) (*Preview, error)
	ListQueries(ctx context.Context, actor Actor, page Page) (*QueryList, error)

	QueryAudit(ctx context.Context, actor Actor, in AuditQuery) (*AuditList, error)
	GetAudit(ctx context.Context, actor Actor, id string) (*AuditEvent, error)
}
