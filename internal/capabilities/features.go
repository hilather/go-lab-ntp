package capabilities

// FeatureApplyLive and FeatureApplyResetOnly are the only apply values.
const (
	FeatureApplyLive      = "live"
	FeatureApplyResetOnly = "reset-only"
)

// Feature is one frozen live vs reset-only row.
type Feature struct {
	ID    string `json:"id"`
	Apply string `json:"apply"`
	Path  string `json:"path"`
}

// Features is the frozen operator catalog. PR 13 must not add ids.
func Features() []Feature {
	return []Feature{
		{ID: "filters", Apply: FeatureApplyLive, Path: "spec.filters"},
		{ID: "views", Apply: FeatureApplyLive, Path: "spec.filters[].view"},
		{ID: "restrict", Apply: FeatureApplyLive, Path: "spec.ntp.restrict"},
		{ID: "admission", Apply: FeatureApplyLive, Path: "spec.ntp.admission"},
		{ID: "allowClientCidrs", Apply: FeatureApplyLive, Path: "spec.ntp.allowClientCidrs"},
		{ID: "queryLog", Apply: FeatureApplyLive, Path: "spec.ntp.queryLog"},
		{ID: "management.http", Apply: FeatureApplyLive, Path: "spec.management.bodyLimit|requestsPerSecond|burst|maxConcurrent"},
		{ID: "listeners.ntp.address", Apply: FeatureApplyResetOnly, Path: "spec.listeners.ntp.address"},
		{ID: "listeners.management.address", Apply: FeatureApplyResetOnly, Path: "spec.listeners.management.address"},
		{ID: "ntp.nts", Apply: FeatureApplyResetOnly, Path: "spec.ntp.nts"},
		{ID: "ntp.symmetricKeys", Apply: FeatureApplyResetOnly, Path: "spec.ntp.symmetricKeys"},
		{ID: "auth", Apply: FeatureApplyResetOnly, Path: "spec.auth"},
	}
}

// FeatureIDs is the frozen id list in catalog order.
func FeatureIDs() []string {
	fs := Features()
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.ID
	}
	return out
}
