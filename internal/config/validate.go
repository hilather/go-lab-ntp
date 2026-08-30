package config

import (
	"fmt"
	"math"
	"net"
	"net/netip"
	"regexp"
	"strings"
	"time"

	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/model"
)

var filterNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)

// Validate checks a (preferably normalized) state. It does not mutate st.
func Validate(st *model.State) error {
	if st == nil {
		return domainerr.ValidationFailed("nil state",
			domainerr.FieldViolation{Path: "", Code: violationRequired, Message: "state is nil"})
	}
	var vs []domainerr.FieldViolation
	validateDocument(st, &vs)
	validateListeners(&st.Spec.Listeners, &vs)
	validateAuth(&st.Spec.Auth, &vs)
	validateManagement(&st.Spec.Management, &vs)
	validateNTP(&st.Spec.NTP, &vs)
	validateFilters(st.Spec.Filters, &vs)
	validateObservability(&st.Spec.Observability, &vs)
	if len(vs) > 0 {
		return domainerr.ValidationFailed("Candidate state is invalid.", vs...)
	}
	return nil
}

func validateDocument(st *model.State, vs *[]domainerr.FieldViolation) {
	if st.APIVersion != model.APIVersionV1Alpha1 {
		code := violationUnsupportedVersion
		msg := fmt.Sprintf("apiVersion must be %q", model.APIVersionV1Alpha1)
		if strings.TrimSpace(st.APIVersion) == "" {
			code = violationRequired
			msg = "apiVersion is required"
		}
		*vs = append(*vs, domainerr.FieldViolation{Path: "apiVersion", Code: code, Message: msg})
	}
	if st.Kind != model.KindLabNTP {
		code := violationInvalidValue
		msg := fmt.Sprintf("kind must be %q", model.KindLabNTP)
		if strings.TrimSpace(st.Kind) == "" {
			code = violationRequired
			msg = "kind is required"
		}
		*vs = append(*vs, domainerr.FieldViolation{Path: "kind", Code: code, Message: msg})
	}
	if strings.TrimSpace(st.Metadata.Name) == "" {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "metadata.name",
			Code:    violationRequired,
			Message: "metadata.name is required",
		})
	}
}

func validateListeners(l *model.ListenersSpec, vs *[]domainerr.FieldViolation) {
	validateUDPAddr("spec.listeners.ntp.address", l.NTP.Address, vs)
	validateTCPAddr("spec.listeners.management.address", l.Management.Address, vs)
	if l.Management.RESTPath != "" && !strings.HasPrefix(l.Management.RESTPath, "/") {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "spec.listeners.management.restPath",
			Code:    violationInvalidValue,
			Message: "restPath must start with /",
		})
	}
	if l.Management.MCPPath != "" && !strings.HasPrefix(l.Management.MCPPath, "/") {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "spec.listeners.management.mcpPath",
			Code:    violationInvalidValue,
			Message: "mcpPath must start with /",
		})
	}
}

func validateAuth(a *model.AuthSpec, vs *[]domainerr.FieldViolation) {
	switch a.Mode {
	case "", model.MgmtAuthBearer, model.MgmtAuthDevLoopbackUnauth:
	default:
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "spec.auth.mode",
			Code:    violationInvalidValue,
			Message: "mode must be bearer or dev-loopback-unauth",
		})
	}
	ids := map[string]int{}
	for i, tok := range a.Tokens {
		p := fmt.Sprintf("spec.auth.tokens[%d]", i)
		if strings.TrimSpace(tok.ID) == "" {
			*vs = append(*vs, domainerr.FieldViolation{Path: p + ".id", Code: violationEmptyID, Message: "token id is required"})
		} else if prev, ok := ids[tok.ID]; ok {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    p + ".id",
				Code:    violationDuplicateID,
				Message: fmt.Sprintf("duplicate token id %q (also tokens[%d])", tok.ID, prev),
			})
		} else {
			ids[tok.ID] = i
		}
		switch tok.Role {
		case "", model.RoleViewer, model.RoleOperator, model.RoleAdministrator:
		default:
			*vs = append(*vs, domainerr.FieldViolation{Path: p + ".role", Code: violationInvalidValue, Message: "role must be viewer, operator, or administrator"})
		}
		if strings.TrimSpace(tok.SecretFile) == "" {
			*vs = append(*vs, domainerr.FieldViolation{Path: p + ".secretFile", Code: violationRequired, Message: "secretFile is required (file ref, never inline)"})
		}
	}
}

func validateManagement(m *model.ManagementSpec, vs *[]domainerr.FieldViolation) {
	if m.BodyLimit < 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.management.bodyLimit", Code: violationInvalidValue, Message: "bodyLimit must be >= 0"})
	}
	if m.RequestsPerSecond < 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.management.requestsPerSecond", Code: violationInvalidValue, Message: "requestsPerSecond must be >= 0"})
	}
	if m.Burst < 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.management.burst", Code: violationInvalidValue, Message: "burst must be >= 0"})
	}
	if m.MaxConcurrent < 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.management.maxConcurrent", Code: violationInvalidValue, Message: "maxConcurrent must be >= 0"})
	}
	for i, o := range m.AllowedOrigins {
		if o == "*" {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    fmt.Sprintf("spec.management.allowedOrigins[%d]", i),
				Code:    violationInvalidValue,
				Message: `"*" is not allowed in allowedOrigins`,
			})
		}
	}
}

func validateNTP(n *model.NTPSpec, vs *[]domainerr.FieldViolation) {
	if n.ServeMode != "" && n.ServeMode != model.ServeModeUnicast {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "spec.ntp.serveMode",
			Code:    violationInvalidValue,
			Message: "serveMode must be unicast",
		})
	}
	if n.NTS.Enabled {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "spec.ntp.nts.enabled",
			Code:    violationInvalidValue,
			Message: "nts.enabled must be false in v1",
		})
	}
	switch n.Restrict.Default {
	case "", model.RestrictServe, model.RestrictLimited, model.RestrictIgnore:
	default:
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "spec.ntp.restrict.default",
			Code:    violationInvalidValue,
			Message: "restrict.default must be serve, limited, or ignore",
		})
	}
	seenV := map[int]bool{}
	for i, v := range n.Versions {
		if v != 3 && v != 4 {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    fmt.Sprintf("spec.ntp.versions[%d]", i),
				Code:    violationInvalidValue,
				Message: "versions must be a subset of {3,4}",
			})
		}
		if seenV[v] {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    fmt.Sprintf("spec.ntp.versions[%d]", i),
				Code:    violationDuplicateID,
				Message: "duplicate version",
			})
		}
		seenV[v] = true
	}
	if n.Admission.MaxPacketsPerSec < 1 {
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.ntp.admission.maxPacketsPerSec", Code: violationInvalidValue, Message: "maxPacketsPerSec must be >= 1"})
	}
	if n.Admission.MaxPacketsPerIP < 1 {
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.ntp.admission.maxPacketsPerIP", Code: violationInvalidValue, Message: "maxPacketsPerIP must be >= 1"})
	}
	if n.QueryLog.Size < 1 || n.QueryLog.Size > MaxQueryLogSize {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "spec.ntp.queryLog.size",
			Code:    violationInvalidValue,
			Message: fmt.Sprintf("queryLog.size must be in [1, %d]", MaxQueryLogSize),
		})
	}
	for i, c := range n.AllowClientCidrs {
		if _, err := netip.ParsePrefix(c); err != nil {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    fmt.Sprintf("spec.ntp.allowClientCidrs[%d]", i),
				Code:    violationInvalidValue,
				Message: fmt.Sprintf("invalid CIDR %q", c),
			})
		}
	}
}

func validateFilters(filters []model.Filter, vs *[]domainerr.FieldViolation) {
	names := map[string]int{}
	hasV4Default := false
	hasV6Default := false
	v4Default, _ := netip.ParsePrefix("0.0.0.0/0")
	v6Default, _ := netip.ParsePrefix("::/0")
	for i, f := range filters {
		p := fmt.Sprintf("spec.filters[%d]", i)
		if strings.TrimSpace(f.Name) == "" {
			*vs = append(*vs, domainerr.FieldViolation{Path: p + ".name", Code: violationEmptyID, Message: "filter name is required"})
		} else if !filterNamePattern.MatchString(f.Name) {
			*vs = append(*vs, domainerr.FieldViolation{Path: p + ".name", Code: violationInvalidValue, Message: "filter name must match [a-z0-9][a-z0-9._-]{0,62}"})
		} else if prev, ok := names[f.Name]; ok {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    p + ".name",
				Code:    violationDuplicateID,
				Message: fmt.Sprintf("duplicate filter name %q (also filters[%d])", f.Name, prev),
			})
		} else {
			names[f.Name] = i
		}
		if f.Enabled && len(f.Match.CIDRs) == 0 {
			*vs = append(*vs, domainerr.FieldViolation{Path: p + ".match.cidrs", Code: violationRequired, Message: "enabled filter requires at least one CIDR"})
		}
		for j, c := range f.Match.CIDRs {
			pfx, err := netip.ParsePrefix(c)
			if err != nil {
				*vs = append(*vs, domainerr.FieldViolation{
					Path:    fmt.Sprintf("%s.match.cidrs[%d]", p, j),
					Code:    violationInvalidValue,
					Message: fmt.Sprintf("invalid CIDR %q", c),
				})
				continue
			}
			if f.Enabled {
				if pfx == v4Default {
					hasV4Default = true
				}
				if pfx == v6Default {
					hasV6Default = true
				}
			}
		}
		validateView(p+".view", f.View, vs)
	}
	if !hasV4Default || !hasV6Default {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "spec.filters",
			Code:    violationRequired,
			Message: "enabled filters must include catch-all 0.0.0.0/0 and ::/0",
		})
	}
}

func validateView(path string, v model.ViewSpec, vs *[]domainerr.FieldViolation) {
	switch v.Mode {
	case model.ModeFollowReal, model.ModeOffset, model.ModeAbsolute, model.ModeFreeze, model.ModeRate:
	case "":
		*vs = append(*vs, domainerr.FieldViolation{Path: path + ".mode", Code: violationRequired, Message: "mode is required"})
		return
	default:
		*vs = append(*vs, domainerr.FieldViolation{Path: path + ".mode", Code: violationInvalidValue, Message: "mode must be follow-real, offset, absolute, freeze, or rate"})
		return
	}

	offsetPresent := v.Offset != 0
	absolutePresent := strings.TrimSpace(v.Absolute) != ""
	freezePresent := strings.TrimSpace(v.FreezeAt) != ""
	ratePresent := v.Rate != nil
	epochPresent := strings.TrimSpace(v.Epoch) != ""

	forbid := func(field, why string) {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    path + "." + field,
			Code:    violationInvalidValue,
			Message: why,
		})
	}

	switch v.Mode {
	case model.ModeFollowReal:
		if offsetPresent {
			forbid("offset", "offset is forbidden on follow-real")
		}
		if absolutePresent {
			forbid("absolute", "absolute is forbidden on follow-real")
		}
		if freezePresent {
			forbid("freezeAt", "freezeAt is forbidden on follow-real")
		}
		if ratePresent {
			forbid("rate", "rate is forbidden on follow-real")
		}
		if epochPresent {
			forbid("epoch", "epoch is forbidden on follow-real")
		}
	case model.ModeOffset:
		if absolutePresent {
			forbid("absolute", "absolute is forbidden on offset")
		}
		if freezePresent {
			forbid("freezeAt", "freezeAt is forbidden on offset")
		}
		if ratePresent {
			forbid("rate", "rate is forbidden on offset")
		}
		if epochPresent {
			forbid("epoch", "epoch is forbidden on offset")
		}
	case model.ModeAbsolute:
		if !absolutePresent {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".absolute", Code: violationRequired, Message: "absolute RFC3339 is required on mode absolute"})
		} else if _, err := time.Parse(time.RFC3339Nano, v.Absolute); err != nil {
			if _, err2 := time.Parse(time.RFC3339, v.Absolute); err2 != nil {
				*vs = append(*vs, domainerr.FieldViolation{Path: path + ".absolute", Code: violationInvalidValue, Message: "absolute must be RFC3339"})
			}
		}
		if offsetPresent {
			forbid("offset", "offset is forbidden on absolute")
		}
		if freezePresent {
			forbid("freezeAt", "freezeAt is forbidden on absolute")
		}
		if ratePresent {
			forbid("rate", "rate is forbidden on absolute")
		}
		if epochPresent {
			forbid("epoch", "epoch is forbidden on absolute")
		}
	case model.ModeFreeze:
		if !freezePresent {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".freezeAt", Code: violationRequired, Message: "freezeAt RFC3339 is required on mode freeze"})
		} else if !validRFC3339(v.FreezeAt) {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".freezeAt", Code: violationInvalidValue, Message: "freezeAt must be RFC3339"})
		}
		if offsetPresent {
			forbid("offset", "offset is forbidden on freeze")
		}
		if absolutePresent {
			forbid("absolute", "absolute is forbidden on freeze")
		}
		if ratePresent {
			forbid("rate", "rate is forbidden on freeze")
		}
		if epochPresent {
			forbid("epoch", "epoch is forbidden on freeze")
		}
	case model.ModeRate:
		if !ratePresent {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".rate", Code: violationRequired, Message: "rate key is required on mode rate"})
		} else {
			r := *v.Rate
			if math.IsNaN(r) || math.IsInf(r, 0) {
				*vs = append(*vs, domainerr.FieldViolation{Path: path + ".rate", Code: violationInvalidValue, Message: "rate must be finite"})
			} else if math.Abs(r) > MaxAbsRate {
				*vs = append(*vs, domainerr.FieldViolation{Path: path + ".rate", Code: violationInvalidValue, Message: "rate |rate| must be <= 100"})
			}
		}
		if offsetPresent {
			forbid("offset", "offset is forbidden on rate")
		}
		if absolutePresent {
			forbid("absolute", "absolute is forbidden on rate")
		}
		if freezePresent {
			forbid("freezeAt", "freezeAt is forbidden on rate")
		}
		if epochPresent && !validRFC3339(v.Epoch) {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".epoch", Code: violationInvalidValue, Message: "epoch must be RFC3339"})
		}
	}

	switch v.Leap {
	case "", model.LeapNone, model.LeapInsert, model.LeapDelete, model.LeapUnsync:
	default:
		*vs = append(*vs, domainerr.FieldViolation{Path: path + ".leap", Code: violationInvalidValue, Message: "leap must be none, insert, delete, or unsync"})
	}
	if v.Stratum < 1 || v.Stratum > 16 {
		*vs = append(*vs, domainerr.FieldViolation{Path: path + ".stratum", Code: violationInvalidValue, Message: "stratum must be 1–16 (0 is reserved for KoD)"})
	}
	if v.RootDelay < 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: path + ".rootDelay", Code: violationInvalidValue, Message: "rootDelay must be >= 0"})
	}
	if v.RootDispersion < 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: path + ".rootDispersion", Code: violationInvalidValue, Message: "rootDispersion must be >= 0"})
	}
	if v.Jitter < 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: path + ".jitter", Code: violationInvalidValue, Message: "jitter must be >= 0"})
	}
	if v.MinPoll != nil {
		if *v.MinPoll < MinPoll || *v.MinPoll > MaxPoll {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".minpoll", Code: violationInvalidValue, Message: "minpoll must be in [-6, 17]"})
		}
	}
	if v.MaxPoll != nil {
		if *v.MaxPoll < MinPoll || *v.MaxPoll > MaxPoll {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".maxpoll", Code: violationInvalidValue, Message: "maxpoll must be in [-6, 17]"})
		}
	}
	if v.MinPoll != nil && v.MaxPoll != nil && *v.MinPoll > *v.MaxPoll {
		*vs = append(*vs, domainerr.FieldViolation{Path: path + ".minpoll", Code: violationInvalidValue, Message: "minpoll must be <= maxpoll"})
	}
	if v.RefID != "" && len(v.RefID) > 4 && net.ParseIP(v.RefID) == nil {
		*vs = append(*vs, domainerr.FieldViolation{Path: path + ".refid", Code: violationInvalidValue, Message: "refid must be 1–4 ASCII or an IPv4 dotted-quad"})
	}
}

func validateObservability(o *model.ObservabilitySpec, vs *[]domainerr.FieldViolation) {
	switch o.LogLevel {
	case "", model.LogLevelDebug, model.LogLevelInfo, model.LogLevelWarn, model.LogLevelError:
	default:
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.observability.logLevel", Code: violationInvalidValue, Message: "logLevel must be debug, info, warn, or error"})
	}
}

func validateUDPAddr(path, addr string, vs *[]domainerr.FieldViolation) {
	if strings.TrimSpace(addr) == "" {
		*vs = append(*vs, domainerr.FieldViolation{Path: path, Code: violationRequired, Message: "address is required"})
		return
	}
	if _, err := net.ResolveUDPAddr("udp", addr); err != nil {
		*vs = append(*vs, domainerr.FieldViolation{Path: path, Code: violationInvalidValue, Message: "invalid UDP host:port"})
	}
}

func validateTCPAddr(path, addr string, vs *[]domainerr.FieldViolation) {
	if strings.TrimSpace(addr) == "" {
		*vs = append(*vs, domainerr.FieldViolation{Path: path, Code: violationRequired, Message: "address is required"})
		return
	}
	if _, err := net.ResolveTCPAddr("tcp", addr); err != nil {
		*vs = append(*vs, domainerr.FieldViolation{Path: path, Code: violationInvalidValue, Message: "invalid TCP host:port"})
	}
}

func validRFC3339(s string) bool {
	if _, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return true
	}
	_, err := time.Parse(time.RFC3339, s)
	return err == nil
}

// ParseRFC3339 parses a view RFC3339 instant as UTC.
func ParseRFC3339(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t, err = time.Parse(time.RFC3339, s)
	}
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}
