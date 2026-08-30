package config

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/hilather/go-lab-ntp/internal/domainerr"
)

var reservedExact = map[string]string{
	"chrony":    "implies wrapping chrony",
	"ntpd":      "implies wrapping ntpd",
	"timesyncd": "implies wrapping systemd-timesyncd",
	"ptp":       "implies IEEE 1588 / PTP",
	"broadcast": "implies NTP broadcast mode",
	"multicast": "implies NTP multicast mode",
	"pool":      "implies NTP pool / anycast",
}

var reservedPrefixes = []struct {
	prefix string
	why    string
}{
	{"chrony", "implies wrapping chrony"},
	{"ntpd", "implies wrapping ntpd"},
	{"timesyncd", "implies wrapping systemd-timesyncd"},
}

func normalizeKey(k string) string {
	k = strings.TrimLeft(k, "-")
	var b strings.Builder
	b.Grow(len(k))
	for _, r := range k {
		if r == '-' || r == '_' {
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

func reservedReason(normalized string) string {
	if why, ok := reservedExact[normalized]; ok {
		return why
	}
	for _, p := range reservedPrefixes {
		if strings.HasPrefix(normalized, p.prefix) {
			return p.why
		}
	}
	return ""
}

func isFreeFormStringMap(path string) bool {
	return path == "metadata.labels" || strings.HasSuffix(path, ".labels")
}

func reservedFields(v any, path string) []domainerr.FieldViolation {
	switch x := v.(type) {
	case map[string]any:
		if isFreeFormStringMap(path) {
			return nil
		}
		var vs []domainerr.FieldViolation
		for k, child := range x {
			p := joinPath(path, k)
			if why := reservedReason(normalizeKey(k)); why != "" {
				vs = append(vs, domainerr.FieldViolation{
					Path:    p,
					Code:    violationReservedName,
					Message: fmt.Sprintf("reserved key %q %s — not a LabNTP surface", k, why),
				})
				continue
			}
			vs = append(vs, reservedFields(child, p)...)
		}
		return vs
	case []any:
		var vs []domainerr.FieldViolation
		for i, child := range x {
			vs = append(vs, reservedFields(child, indexPath(path, i))...)
		}
		return vs
	default:
		return nil
	}
}
