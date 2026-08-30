package snapshot

import (
	"net/netip"
	"time"

	"github.com/hilather/go-lab-ntp/internal/config"
	"github.com/hilather/go-lab-ntp/internal/model"
	"github.com/hilather/go-lab-ntp/internal/ntpkeys"
	"github.com/hilather/go-lab-ntp/internal/ntpview"
)

// Snapshot is immutable after Compile returns.
type Snapshot struct {
	Canonical         *model.State
	Revision          model.Revision
	BootstrapRevision model.Revision
	Generation        model.Generation
	CompiledAt        time.Time
	Warnings          []config.Warning

	NTPAddress        string
	ManagementAddress string
	RESTPath          string
	MCPPath           string

	Versions         map[uint8]bool
	ServeMode        string
	Allow            []netip.Prefix
	AllowDenyAll     bool
	RestrictDefault  string
	KoD              bool
	MaxPacketsPerSec int
	MaxPacketsPerIP  int
	QueryLogSize     int

	Filters []Filter
	Keys    *ntpkeys.Table
}

// Filter is one compiled first-match entry.
type Filter struct {
	Name     string
	Enabled  bool
	Prefixes []netip.Prefix
	View     ntpview.View
}

// Match returns the first enabled filter containing unmapped ip, or nil.
func (s *Snapshot) Match(ip netip.Addr) *Filter {
	if s == nil {
		return nil
	}
	probe := ip.Unmap()
	for i := range s.Filters {
		f := &s.Filters[i]
		if !f.Enabled {
			continue
		}
		for _, p := range f.Prefixes {
			if p.Contains(probe) || p.Contains(ip) {
				return f
			}
		}
	}
	return nil
}

// Allowed reports whether unmapped ip is in allowClientCidrs.
func (s *Snapshot) Allowed(ip netip.Addr) bool {
	if s == nil {
		return false
	}
	if s.AllowDenyAll {
		return false
	}
	if len(s.Allow) == 0 {
		return true
	}
	probe := ip.Unmap()
	for _, p := range s.Allow {
		if p.Contains(probe) || p.Contains(ip) {
			return true
		}
	}
	return false
}

// Drifted reports whether the live revision differs from bootstrap.
func (s *Snapshot) Drifted() bool {
	if s == nil {
		return false
	}
	return s.Revision != "" && s.BootstrapRevision != "" && s.Revision != s.BootstrapRevision
}

// Spec is the compiled canonical spec, or a zero spec.
func (s *Snapshot) Spec() model.Spec {
	if s == nil || s.Canonical == nil {
		return model.Spec{}
	}
	return s.Canonical.Spec
}

// VersionOK reports whether VN is in the compiled set.
func (s *Snapshot) VersionOK(vn uint8) bool {
	if s == nil || len(s.Versions) == 0 {
		return vn == 3 || vn == 4
	}
	return s.Versions[vn]
}
