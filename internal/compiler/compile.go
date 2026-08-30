package compiler

import (
	"fmt"
	"net/netip"
	"os"
	"time"

	"github.com/hilather/go-lab-ntp/internal/config"
	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/model"
	"github.com/hilather/go-lab-ntp/internal/ntpkeys"
	"github.com/hilather/go-lab-ntp/internal/ntpview"
	"github.com/hilather/go-lab-ntp/internal/snapshot"
	"github.com/hilather/go-lab-ntp/internal/testutil"
)

// CompileOpts controls revision metadata and the compile clock.
type CompileOpts struct {
	Clock             testutil.Clock
	BootstrapRevision model.Revision
	Generation        model.Generation
	Previous          *snapshot.Snapshot
}

// Compile normalizes and validates st, compiles filters and views, loads keys
// if a file is configured, and returns an immutable Snapshot.
func Compile(st *model.State, opts CompileOpts) (*snapshot.Snapshot, error) {
	n, warns, err := config.Normalize(st)
	if err != nil {
		return nil, err
	}
	if err := config.Validate(n); err != nil {
		return nil, err
	}
	clk := opts.Clock
	if clk == nil {
		clk = testutil.SystemClock{}
	}
	now := clk.Now()

	filters, more, err := compileFilters(n.Spec.Filters, now, opts)
	if err != nil {
		return nil, err
	}
	warns = append(warns, more...)
	warns = append(warns, overlapWarnings(filters)...)

	allow, denyAll, err := compileAllow(n.Spec.NTP.AllowClientCidrs)
	if err != nil {
		return nil, err
	}

	vers := map[uint8]bool{}
	for _, v := range n.Spec.NTP.Versions {
		vers[uint8(v)] = true
	}

	keys, err := loadKeys(n.Spec.NTP.SymmetricKeys.File)
	if err != nil {
		return nil, err
	}

	rev, err := config.Revision(n)
	if err != nil {
		return nil, err
	}
	bootRev := opts.BootstrapRevision
	if bootRev == "" {
		bootRev = rev
	}

	return &snapshot.Snapshot{
		Canonical:         n,
		Revision:          rev,
		BootstrapRevision: bootRev,
		Generation:        opts.Generation,
		CompiledAt:        now,
		Warnings:          warns,
		NTPAddress:        n.Spec.Listeners.NTP.Address,
		ManagementAddress: n.Spec.Listeners.Management.Address,
		RESTPath:          n.Spec.Listeners.Management.RESTPath,
		MCPPath:           n.Spec.Listeners.Management.MCPPath,
		Versions:          vers,
		ServeMode:         n.Spec.NTP.ServeMode,
		Allow:             allow,
		AllowDenyAll:      denyAll,
		RestrictDefault:   n.Spec.NTP.Restrict.Default,
		KoD:               n.Spec.NTP.Restrict.KoD,
		MaxPacketsPerSec:  n.Spec.NTP.Admission.MaxPacketsPerSec,
		MaxPacketsPerIP:   n.Spec.NTP.Admission.MaxPacketsPerIP,
		QueryLogSize:      n.Spec.NTP.QueryLog.Size,
		Filters:           filters,
		Keys:              keys,
	}, nil
}

func compileAllow(cidrs []string) ([]netip.Prefix, bool, error) {
	if cidrs != nil && len(cidrs) == 0 {
		return []netip.Prefix{}, true, nil
	}
	var out []netip.Prefix
	for i, c := range cidrs {
		p, err := netip.ParsePrefix(c)
		if err != nil {
			return nil, false, domainerr.ValidationFailed("invalid allowClientCidrs",
				domainerr.FieldViolation{Path: fmt.Sprintf("spec.ntp.allowClientCidrs[%d]", i), Code: "invalid_value", Message: err.Error()})
		}
		out = append(out, p)
	}
	return out, false, nil
}

func compileFilters(in []model.Filter, now time.Time, opts CompileOpts) ([]snapshot.Filter, []config.Warning, error) {
	var out []snapshot.Filter
	var warns []config.Warning
	for i, f := range in {
		var prefs []netip.Prefix
		for j, c := range f.Match.CIDRs {
			p, err := netip.ParsePrefix(c)
			if err != nil {
				return nil, nil, domainerr.ValidationFailed("invalid CIDR",
					domainerr.FieldViolation{Path: fmt.Sprintf("spec.filters[%d].match.cidrs[%d]", i, j), Code: "invalid_value", Message: err.Error()})
			}
			prefs = append(prefs, p)
		}
		view, w, err := compileView(f, now, opts)
		if err != nil {
			return nil, nil, err
		}
		warns = append(warns, w...)
		out = append(out, snapshot.Filter{
			Name:     f.Name,
			Enabled:  f.Enabled,
			Prefixes: prefs,
			View:     view,
		})
	}
	return out, warns, nil
}

func compileView(f model.Filter, now time.Time, opts CompileOpts) (ntpview.View, []config.Warning, error) {
	v := f.View
	epochMono := now
	epochWall := now.UTC()
	var epochVirtual time.Time
	var warns []config.Warning

	prev := previousView(opts.Previous, f.Name)

	switch v.Mode {
	case model.ModeFollowReal:
		epochVirtual = epochWall
	case model.ModeOffset:
		epochVirtual = epochWall.Add(v.Offset)
	case model.ModeAbsolute:
		t, err := config.ParseRFC3339(v.Absolute)
		if err != nil {
			return ntpview.View{}, nil, err
		}
		epochVirtual = t
	case model.ModeFreeze:
		t, err := config.ParseRFC3339(v.FreezeAt)
		if err != nil {
			return ntpview.View{}, nil, err
		}
		epochVirtual = t
	case model.ModeRate:
		sameRate := prev != nil && rateEqual(prev, v.Rate)
		sameEpochYAML := prev != nil && prevEpochYAML(opts.Previous, f.Name) == v.Epoch
		switch {
		case sameRate && sameEpochYAML:
			epochVirtual = prev.EpochVirtual
			epochMono = prev.EpochMono
			epochWall = prev.EpochWall
		case v.Epoch != "":
			t, err := config.ParseRFC3339(v.Epoch)
			if err != nil {
				return ntpview.View{}, nil, err
			}
			epochVirtual = t
		case prev != nil && v.Epoch == "" && prev.HasRate && !sameRate:
			epochVirtual = prev.Served(now)
			epochMono = now
			epochWall = now.UTC()
		default:
			epochVirtual = epochWall
		}
	}

	out := ntpview.View{
		Name:           f.Name,
		Generation:     uint64(opts.Generation),
		Mode:           v.Mode,
		Offset:         v.Offset,
		Rate:           rateVal(v.Rate),
		HasRate:        v.Rate != nil,
		EpochVirtual:   epochVirtual,
		EpochMono:      epochMono,
		EpochWall:      epochWall,
		Leap:           v.Leap,
		Stratum:        v.Stratum,
		RefID:          v.RefID,
		Precision:      v.Precision,
		RootDelay:      v.RootDelay,
		RootDispersion: v.RootDispersion,
		Jitter:         v.Jitter,
		MinPoll:        v.MinPoll,
		MaxPoll:        v.MaxPoll,
	}
	if v.Mode == model.ModeAbsolute {
		t, _ := config.ParseRFC3339(v.Absolute)
		out.Absolute = t
	}
	if v.Mode == model.ModeFreeze {
		t, _ := config.ParseRFC3339(v.FreezeAt)
		out.FreezeAt = t
	}
	if out.ServedClamped(now) {
		warns = append(warns, config.Warning{
			Path:    "spec.filters[name=" + f.Name + "].view",
			Code:    "servedTimeClamped",
			Message: "served time clamps to [1900-01-01, 2172-03-15T12:56:32Z)",
		})
	}
	if v.Leap == model.LeapUnsync && v.Stratum != 16 {
		warns = append(warns, config.Warning{
			Path:    "spec.filters[name=" + f.Name + "].view",
			Code:    "unsync_stratum",
			Message: "leap unsync with stratum != 16",
		})
	}
	return out, warns, nil
}

func previousView(prev *snapshot.Snapshot, name string) *ntpview.View {
	if prev == nil {
		return nil
	}
	for i := range prev.Filters {
		if prev.Filters[i].Name == name {
			return &prev.Filters[i].View
		}
	}
	return nil
}

func prevEpochYAML(prev *snapshot.Snapshot, name string) string {
	if prev == nil || prev.Canonical == nil {
		return ""
	}
	for _, f := range prev.Canonical.Spec.Filters {
		if f.Name == name {
			return f.View.Epoch
		}
	}
	return ""
}

func rateEqual(prev *ntpview.View, rate *float64) bool {
	if prev == nil || !prev.HasRate || rate == nil {
		return false
	}
	return prev.Rate == *rate
}

func rateVal(r *float64) float64 {
	if r == nil {
		return 0
	}
	return *r
}

func overlapWarnings(filters []snapshot.Filter) []config.Warning {
	var ws []config.Warning
	for i := 0; i < len(filters); i++ {
		if !filters[i].Enabled {
			continue
		}
		for j := i + 1; j < len(filters); j++ {
			if !filters[j].Enabled {
				continue
			}
			if prefixesOverlap(filters[i].Prefixes, filters[j].Prefixes) {
				ws = append(ws, config.Warning{
					Path:    fmt.Sprintf("spec.filters[%d].match.cidrs", i),
					Code:    "overlap",
					Message: fmt.Sprintf("enabled filters %q and %q have overlapping CIDRs; first-match order wins", filters[i].Name, filters[j].Name),
				})
			}
		}
	}
	return ws
}

func prefixesOverlap(a, b []netip.Prefix) bool {
	for _, x := range a {
		for _, y := range b {
			if x.Overlaps(y) {
				return true
			}
		}
	}
	return false
}

func loadKeys(path string) (*ntpkeys.Table, error) {
	if path == "" {
		return nil, nil
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, domainerr.ValidationFailed("symmetric keys file missing",
				domainerr.FieldViolation{Path: "spec.ntp.symmetricKeys.file", Code: "required", Message: "keys file " + path + " does not exist"})
		}
		return nil, err
	}
	tab, err := ntpkeys.ParseFile(path)
	if err != nil {
		return nil, domainerr.ValidationFailed("symmetric keys parse failed",
			domainerr.FieldViolation{Path: "spec.ntp.symmetricKeys.file", Code: "invalid_value", Message: err.Error()})
	}
	return tab, nil
}
