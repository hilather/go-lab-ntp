package app

import (
	"encoding/json"
	"time"

	"github.com/hilather/go-lab-ntp/internal/audit"
	"github.com/hilather/go-lab-ntp/internal/buildinfo"
	"github.com/hilather/go-lab-ntp/internal/capabilities"
	"github.com/hilather/go-lab-ntp/internal/model"
	"github.com/hilather/go-lab-ntp/internal/observability"
	"github.com/hilather/go-lab-ntp/internal/querylog"
)

// Actor is the caller identity recorded on audit and used for scope checks.
type Actor struct {
	ID        string
	Class     string
	Role      string
	Scopes    []string
	Transport string
}

// ChangeIn is the shared plan/apply envelope.
type ChangeIn struct {
	ExpectedRevision model.Revision
	IdempotencyKey   string
	Reason           string
	Force            bool
	Operations       []model.Operation
}

// ValidateIn validates a candidate document and/or operations.
type ValidateIn struct {
	State      *model.State
	Operations []model.Operation
}

// ResetIn is the privileged bootstrap reread. expectedRevision is not required.
type ResetIn struct {
	Reason string
}

// Plan is the dry-run result of validate/plan (and the body of apply).
type Plan struct {
	PreviousRevision  model.Revision
	CandidateRevision model.Revision
	Drifted           bool
	Diff              []DiffEntry
	Warnings          []Warning
	Operations        []model.Operation
}

// ApplyResult is a committed mutation result.
type ApplyResult struct {
	Plan
	Applied         bool
	Generation      model.Generation
	RuntimeRevision model.Revision
	AuditEventID    string
}

// ExportFormat selects canonical YAML or JSON. Comments are never preserved.
type ExportFormat string

const (
	ExportYAML ExportFormat = "yaml"
	ExportJSON ExportFormat = "json"
)

// Export is canonical desired state plus drift material.
type Export struct {
	Format            ExportFormat
	Body              []byte
	Revision          model.Revision
	BootstrapRevision model.Revision
	Drifted           bool
	HumanDiff         string
}

// StateView is GET /v1/state. Canonical is a copy; mutating it cannot
// affect the live snapshot.
type StateView struct {
	BootstrapRevision model.Revision
	RuntimeRevision   model.Revision
	Generation        model.Generation
	Drifted           bool
	LoadedAt          time.Time
	Canonical         *model.State
}

// Status is the agent-readable process DTO.
type Status struct {
	Ready     bool
	Revisions RevisionView
	Listeners []ListenerStatus
	HostTime  time.Time
	Warnings  []Warning
}

// RevisionView is bootstrap vs runtime identity.
type RevisionView struct {
	BootstrapRevision model.Revision
	RuntimeRevision   model.Revision
	Generation        model.Generation
	Drifted           bool
	LoadedAt          time.Time
}

// ListenerStatus is one bound (or configured) listener.
type ListenerStatus struct {
	Name    string
	Address string
}

// Warning is a bounded, stable-coded note.
type Warning struct {
	Code    string
	Message string
}

// DiffEntry is one canonical-path change. Paths are sorted in plans.
type DiffEntry struct {
	Path   string          `json:"path"`
	Op     string          `json:"op"`
	Before json.RawMessage `json:"before,omitempty"`
	After  json.RawMessage `json:"after,omitempty"`
}

// CapabilityView lists frozen capability names for discovery.
type CapabilityView struct {
	Capabilities []CapabilityInfo
}

// CapabilityInfo is one registry discovery row (tool name, or health.* id).
type CapabilityInfo struct {
	Name        string
	Version     string
	Description string
	Mutating    bool
	Idempotent  bool
}

// FeatureList is GET /v1/features.
type FeatureList struct {
	Items []capabilities.Feature
}

// FilterList is GET /v1/filters.
type FilterList struct {
	Items []model.Filter
}

// PutFilterIn upserts one filter through Apply.
type PutFilterIn struct {
	ExpectedRevision model.Revision
	IdempotencyKey   string
	Reason           string
	Filter           model.Filter
}

// DeleteIn deletes one filter through Apply.
type DeleteIn struct {
	ExpectedRevision model.Revision
	IdempotencyKey   string
	Reason           string
}

// Preview is GET /v1/views/preview.
type Preview struct {
	IP             string     `json:"ip"`
	Filter         string     `json:"filter"`
	ServedTime     *time.Time `json:"servedTime"`
	HostTime       time.Time  `json:"hostTime"`
	Mode           string     `json:"mode,omitempty"`
	Leap           string     `json:"leap,omitempty"`
	Stratum        int        `json:"stratum,omitempty"`
	RefID          string     `json:"refid,omitempty"`
	OffsetFromHost string     `json:"offsetFromHost,omitempty"`
	Reason         string     `json:"reason,omitempty"`
}

// Page is opaque-cursor pagination. Empty cursor starts at the beginning.
type Page struct {
	Limit  int
	Cursor string
}

// QueryList is GET /v1/queries.
type QueryList struct {
	Items      []querylog.Entry
	NextCursor string
}

// AuditQuery lists recent in-memory events.
type AuditQuery struct {
	Limit int
}

// AuditList is a newest-first page of the ring.
type AuditList struct {
	Events []AuditEvent
}

// AuditEvent is one mutation or security record.
type AuditEvent = audit.Event

// HealthFacts is the input to Status.Ready / observability.Evaluate.
type HealthFacts = observability.Facts

// Version is buildinfo.Info.
type Version = buildinfo.Info
