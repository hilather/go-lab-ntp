package model

import "time"

const (
	MgmtAuthBearer            = "bearer"
	MgmtAuthDevLoopbackUnauth = "dev-loopback-unauth"

	RoleViewer        = "viewer"
	RoleOperator      = "operator"
	RoleAdministrator = "administrator"

	ScopeNTPRead      = "ntp.read"
	ScopeNTPWrite     = "ntp.write"
	ScopeNTPAdmin     = "ntp.admin"
	ScopeNTPAuditRead = "ntp.audit.read"

	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"

	ModeFollowReal = "follow-real"
	ModeOffset     = "offset"
	ModeAbsolute   = "absolute"
	ModeFreeze     = "freeze"
	ModeRate       = "rate"

	LeapNone   = "none"
	LeapInsert = "insert"
	LeapDelete = "delete"
	LeapUnsync = "unsync"

	RestrictServe   = "serve"
	RestrictLimited = "limited"
	RestrictIgnore  = "ignore"

	ServeModeUnicast = "unicast"
)

// KnownRole reports whether role is a v1alpha1 token role.
func KnownRole(role string) bool {
	switch role {
	case RoleViewer, RoleOperator, RoleAdministrator:
		return true
	default:
		return false
	}
}

// ListenersSpec configures the NTP and management listeners.
type ListenersSpec struct {
	NTP        NTPListenerSpec  `json:"ntp"`
	Management MgmtListenerSpec `json:"management"`
}

// NTPListenerSpec is the data-plane UDP listener.
type NTPListenerSpec struct {
	Address string `json:"address"`
}

// MgmtListenerSpec is the control-plane HTTP listener.
type MgmtListenerSpec struct {
	Address  string `json:"address"`
	RESTPath string `json:"restPath"`
	MCPPath  string `json:"mcpPath"`
}

// AuthSpec is spec.auth (not spec.management.auth).
type AuthSpec struct {
	Mode   string      `json:"mode"`
	Tokens []TokenSpec `json:"tokens"`
}

// TokenSpec is one static bearer principal. Secrets are file refs only.
type TokenSpec struct {
	ID         string   `json:"id"`
	Role       string   `json:"role"`
	SecretFile string   `json:"secretFile"`
	Scopes     []string `json:"scopes,omitempty"`
}

// UISpec is spec.ui.enabled.
type UISpec struct {
	Enabled bool `json:"enabled"`
}

// ManagementSpec is origins, MCP, and HTTP limits — not ui, not auth.
type ManagementSpec struct {
	AllowedOrigins    []string `json:"allowedOrigins"`
	MCP               MCPSpec  `json:"mcp"`
	BodyLimit         int64    `json:"bodyLimit"`
	RequestsPerSecond int      `json:"requestsPerSecond"`
	Burst             int      `json:"burst"`
	MaxConcurrent     int      `json:"maxConcurrent"`
}

// MCPSpec is management MCP adapter knobs.
type MCPSpec struct {
	AllowLegacyClients bool `json:"allowLegacyClients"`
}

// NTPSpec is the data-plane posture.
type NTPSpec struct {
	Versions         []int            `json:"versions"`
	ServeMode        string           `json:"serveMode"`
	NTS              NTSSpec          `json:"nts"`
	SymmetricKeys    SymmetricKeySpec `json:"symmetricKeys"`
	Restrict         RestrictSpec     `json:"restrict"`
	AllowClientCidrs []string         `json:"allowClientCidrs"`
	Admission        NTPAdmissionSpec `json:"admission"`
	QueryLog         QueryLogSpec     `json:"queryLog"`
}

// NTSSpec is a schema key; enabled must be false in v1.
type NTSSpec struct {
	Enabled bool `json:"enabled"`
}

// SymmetricKeySpec is a file ref. Inline keys are rejected.
type SymmetricKeySpec struct {
	File string `json:"file"`
}

// RestrictSpec is serve / limited / ignore plus KoD.
type RestrictSpec struct {
	Default string `json:"default"`
	KoD     bool   `json:"kod"`
}

// NTPAdmissionSpec is process-global and per-IP token buckets.
type NTPAdmissionSpec struct {
	MaxPacketsPerSec int `json:"maxPacketsPerSec"`
	MaxPacketsPerIP  int `json:"maxPacketsPerIP"`
}

// QueryLogSpec is the in-memory query ring size.
type QueryLogSpec struct {
	Size int `json:"size"`
}

// Filter is one first-match CIDR view.
type Filter struct {
	Name    string    `json:"name"`
	Enabled bool      `json:"enabled"`
	Match   MatchSpec `json:"match"`
	View    ViewSpec  `json:"view"`
}

// MatchSpec is IP/CIDR only in v1.
type MatchSpec struct {
	CIDRs []string `json:"cidrs"`
}

// ViewSpec is a virtual clock. Rate/MinPoll/MaxPoll are pointers so omitted
// vs explicit 0 is distinguishable (D20). JSON names minpoll/maxpoll/refid
// are the FR spellings; KnownFields rejects minPoll/refID.
type ViewSpec struct {
	Mode           string        `json:"mode"`
	Offset         time.Duration `json:"offset"`
	Absolute       string        `json:"absolute,omitempty"`
	FreezeAt       string        `json:"freezeAt,omitempty"`
	Rate           *float64      `json:"rate,omitempty"`
	Epoch          string        `json:"epoch,omitempty"`
	Leap           string        `json:"leap"`
	Stratum        int           `json:"stratum"`
	RefID          string        `json:"refid"`
	Precision      int           `json:"precision"`
	RootDelay      time.Duration `json:"rootDelay"`
	RootDispersion time.Duration `json:"rootDispersion"`
	Jitter         time.Duration `json:"jitter"`
	MinPoll        *int          `json:"minpoll,omitempty"`
	MaxPoll        *int          `json:"maxpoll,omitempty"`
}

// ObservabilitySpec is log level and metrics bind.
type ObservabilitySpec struct {
	LogLevel string      `json:"logLevel"`
	Metrics  MetricsSpec `json:"metrics"`
	Audit    AuditSpec   `json:"audit"`
}

// MetricsSpec is hand-rolled OpenMetrics (later PR).
type MetricsSpec struct {
	Listen     string `json:"listen"`
	PublicPath bool   `json:"publicPath"`
}

// AuditSpec is the management audit ring size.
type AuditSpec struct {
	Ring int `json:"ring"`
}
