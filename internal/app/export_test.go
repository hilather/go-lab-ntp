package app

// SetMgmtOverrideForTest sets the process management-listen override.
// Production code has no setter: labntp serve pins it at construction, and
// YAML cannot express off. Tests use this to reach Rebind("").
func (s *App) SetMgmtOverrideForTest(v string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.mgmtOverride = v
	s.mu.Unlock()
}
