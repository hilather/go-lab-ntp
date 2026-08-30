package config

const (
	DefaultNTPAddress        = ":123"
	DefaultMgmtAddress       = ":8088"
	DefaultRESTPath          = "/v1"
	DefaultMCPPath           = "/mcp"
	DefaultBodyLimit         = int64(1 << 20)
	DefaultRequestsPerSecond = 32
	DefaultBurst             = 64
	DefaultMaxConcurrent     = 256
	DefaultAuditRing         = 128
	DefaultMetricsListen     = "127.0.0.1:9090"
	DefaultMaxPacketsPerSec  = 256
	DefaultMaxPacketsPerIP   = 32
	DefaultQueryLogSize      = 256
	DefaultPrecision         = -20
	DefaultStratum           = 2
	MaxDocumentBytes         = 1 << 20
	MaxQueryLogSize          = 4096
	MinPoll                  = -6
	MaxPoll                  = 17
	MaxAbsRate               = 100.0

	violationUnknownField       = "unknown_field"
	violationRequired           = "required"
	violationInvalidValue       = "invalid_value"
	violationReservedName       = "reserved_name"
	violationDuplicateKey       = "duplicate_key"
	violationTooLarge           = "document_too_large"
	violationUnsupportedVersion = "unsupported_version"
	violationDuplicateID        = "duplicate_id"
	violationEmptyID            = "empty_id"

	warningUniversalAllowlist = "universal_allowlist"
	warningOverlap            = "overlap"
	warningServedTimeClamped  = "servedTimeClamped"
	warningUnsyncStratum      = "unsync_stratum"
)
