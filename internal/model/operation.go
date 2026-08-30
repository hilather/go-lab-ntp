package model

// Closed plan/apply verbs. Listen/NTS/keys/auth are reset-only.
const (
	OpReplaceFilters          = "replaceFilters"
	OpUpsertFilter            = "upsertFilter"
	OpRemoveFilter            = "removeFilter"
	OpReplaceRestrict         = "replaceRestrict"
	OpReplaceAdmission        = "replaceAdmission"
	OpReplaceAllowClientCidrs = "replaceAllowClientCidrs"
	OpReplaceQueryLog         = "replaceQueryLog"
	OpReplaceManagementHTTP   = "replaceManagementHTTP"
)

// ChangeSet is the plan/apply envelope.
type ChangeSet struct {
	ExpectedRevision string      `json:"expectedRevision"`
	IdempotencyKey   string      `json:"idempotencyKey"`
	Reason           string      `json:"reason"`
	Force            bool        `json:"force"`
	Operations       []Operation `json:"operations"`
}

// Operation is one typed config mutation.
type Operation struct {
	Op               string            `json:"op"`
	Filters          []Filter          `json:"filters,omitempty"`
	Filter           *Filter           `json:"filter,omitempty"`
	Name             string            `json:"name,omitempty"`
	Restrict         *RestrictSpec     `json:"restrict,omitempty"`
	Admission        *NTPAdmissionSpec `json:"admission,omitempty"`
	AllowClientCidrs []string          `json:"allowClientCidrs,omitempty"`
	QueryLog         *QueryLogSpec     `json:"queryLog,omitempty"`
	ManagementHTTP   *ManagementHTTP   `json:"managementHTTP,omitempty"`
}

// ManagementHTTP is the live-apply subset of ManagementSpec.
type ManagementHTTP struct {
	BodyLimit         int64 `json:"bodyLimit"`
	RequestsPerSecond int   `json:"requestsPerSecond"`
	Burst             int   `json:"burst"`
	MaxConcurrent     int   `json:"maxConcurrent"`
}

// KnownOp reports whether op is a v1alpha1 plan/apply verb.
func KnownOp(op string) bool {
	switch op {
	case OpReplaceFilters, OpUpsertFilter, OpRemoveFilter,
		OpReplaceRestrict, OpReplaceAdmission, OpReplaceAllowClientCidrs,
		OpReplaceQueryLog, OpReplaceManagementHTTP:
		return true
	default:
		return false
	}
}
