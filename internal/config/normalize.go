package config

import (
	"encoding/json"
	"strings"

	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/model"
)

// Normalize returns a copy of st with defaults materialized. allowClientCidrs
// nil (omitted/null) becomes the universal list and a warning is returned.
func Normalize(st *model.State) (*model.State, []Warning, error) {
	if st == nil {
		return nil, nil, domainerr.ValidationFailed("nil state",
			domainerr.FieldViolation{Path: "", Code: violationRequired, Message: "state is nil"})
	}
	out, err := cloneState(st)
	if err != nil {
		return nil, nil, err
	}
	var warns []Warning
	materializeDefaults(&out.Spec, &warns)
	return out, warns, nil
}

func cloneState(st *model.State) (*model.State, error) {
	b, err := json.Marshal(st)
	if err != nil {
		return nil, domainerr.Internal("clone marshal: " + err.Error())
	}
	var out model.State
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, domainerr.Internal("clone unmarshal: " + err.Error())
	}
	return &out, nil
}

func materializeDefaults(sp *model.Spec, warns *[]Warning) {
	if strings.TrimSpace(sp.Listeners.NTP.Address) == "" {
		sp.Listeners.NTP.Address = DefaultNTPAddress
	}
	if strings.TrimSpace(sp.Listeners.Management.Address) == "" {
		sp.Listeners.Management.Address = DefaultMgmtAddress
	}
	if strings.TrimSpace(sp.Listeners.Management.RESTPath) == "" {
		sp.Listeners.Management.RESTPath = DefaultRESTPath
	}
	if strings.TrimSpace(sp.Listeners.Management.MCPPath) == "" {
		sp.Listeners.Management.MCPPath = DefaultMCPPath
	}
	if strings.TrimSpace(sp.Auth.Mode) == "" {
		sp.Auth.Mode = model.MgmtAuthBearer
	}
	if sp.Auth.Tokens == nil {
		sp.Auth.Tokens = []model.TokenSpec{}
	}
	if sp.Management.AllowedOrigins == nil {
		sp.Management.AllowedOrigins = []string{}
	}
	if len(sp.NTP.Versions) == 0 {
		sp.NTP.Versions = []int{3, 4}
	}
	if strings.TrimSpace(sp.NTP.ServeMode) == "" {
		sp.NTP.ServeMode = model.ServeModeUnicast
	}
	if strings.TrimSpace(sp.NTP.Restrict.Default) == "" {
		sp.NTP.Restrict.Default = model.RestrictServe
	}
	if sp.NTP.AllowClientCidrs == nil {
		sp.NTP.AllowClientCidrs = []string{"0.0.0.0/0", "::/0"}
		*warns = append(*warns, Warning{
			Path:    "spec.ntp.allowClientCidrs",
			Code:    warningUniversalAllowlist,
			Message: "omitted or null allowClientCidrs materializes 0.0.0.0/0 and ::/0",
		})
	}
	if sp.Filters == nil {
		sp.Filters = []model.Filter{}
	}
	for i := range sp.Filters {
		materializeView(&sp.Filters[i].View)
	}
	if strings.TrimSpace(sp.Observability.LogLevel) == "" {
		sp.Observability.LogLevel = model.LogLevelInfo
	}
	if strings.TrimSpace(sp.Observability.Metrics.Listen) == "" {
		sp.Observability.Metrics.Listen = DefaultMetricsListen
	}
	if sp.Observability.Audit.Ring == 0 {
		sp.Observability.Audit.Ring = DefaultAuditRing
	}
}

func materializeView(v *model.ViewSpec) {
	if strings.TrimSpace(v.Leap) == "" {
		v.Leap = model.LeapNone
	}
	if strings.TrimSpace(v.RefID) == "" {
		switch v.Stratum {
		case 1:
			v.RefID = "GPS"
		case 16:
			v.RefID = "INIT"
		default:
			v.RefID = "LOCL"
		}
	}
}
