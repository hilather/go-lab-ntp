package config

// Warning is a non-fatal compile/validate observation.
type Warning struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
