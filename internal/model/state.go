package model

const (
	// APIVersionV1Alpha1 is the only first-GA config API version.
	APIVersionV1Alpha1 = "labntp.dev/v1alpha1"
	// KindLabNTP is the config document kind.
	KindLabNTP = "LabNTP"
)

// State is the canonical desired-state document.
type State struct {
	APIVersion string   `json:"apiVersion"`
	Kind       string   `json:"kind"`
	Metadata   Metadata `json:"metadata"`
	Spec       Spec     `json:"spec"`
}

// Metadata is document identity and labels.
type Metadata struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels,omitempty"`
}

// Spec is the v1alpha1 desired-state contract. YAML decode and default
// materialization live in config, not here.
type Spec struct {
	Listeners     ListenersSpec     `json:"listeners"`
	Auth          AuthSpec          `json:"auth"`
	NTP           NTPSpec           `json:"ntp"`
	Filters       []Filter          `json:"filters"`
	Management    ManagementSpec    `json:"management"`
	UI            UISpec            `json:"ui"`
	Observability ObservabilitySpec `json:"observability"`
}
