package capabilities

// ID is a frozen capability identifier. Renames are a public-surface change.
type ID string

// First-GA capability IDs. Order matches the implementation-design table.
const (
	HealthLive      ID = "health.live"
	HealthReady     ID = "health.ready"
	VersionGet      ID = "version.get"
	CapabilitiesGet ID = "capabilities.get"
	StatusGet       ID = "status.get"
	SchemaGet       ID = "schema.get"
	FeaturesList    ID = "features.list"
	StateGet        ID = "state.get"
	StateValidate   ID = "state.validate"
	StateExport     ID = "state.export"
	StateReset      ID = "state.reset"
	ChangesPlan     ID = "changes.plan"
	ChangesApply    ID = "changes.apply"
	SessionCreate   ID = "session.create"
	SessionGet      ID = "session.get"
	SessionDelete   ID = "session.delete"
	FiltersList     ID = "filters.list"
	FiltersGet      ID = "filters.get"
	FiltersPut      ID = "filters.put"
	FiltersDelete   ID = "filters.delete"
	ViewsPreview    ID = "views.preview"
	QueriesList     ID = "queries.list"
	AuditList       ID = "audit.list"
	AuditGet        ID = "audit.get"
	MetricsGet      ID = "metrics.get"
)

// VersionTag is the first-GA capability schema version embedded on every row.
const VersionTag = "v1"
