package observability

import "strings"

var (
	forbiddenSet = indexStrings(ForbiddenLabels)
	allowedSet   = indexStrings(AllowedLabels)
)

func indexStrings(in []string) map[string]struct{} {
	out := make(map[string]struct{}, len(in))
	for _, s := range in {
		out[strings.ToLower(s)] = struct{}{}
	}
	return out
}

// ForbiddenLabel reports whether key is a prohibited default label.
func ForbiddenLabel(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	if k == "" {
		return false
	}
	if _, ok := forbiddenSet[k]; ok {
		return true
	}
	if strings.Contains(k, "client_ip") || strings.Contains(k, "remote_addr") ||
		strings.Contains(k, "password") || strings.Contains(k, "cookie") {
		return true
	}
	return false
}

// AllowedLabel reports whether key is in the global allowlist.
func AllowedLabel(key string) bool {
	_, ok := allowedSet[strings.ToLower(strings.TrimSpace(key))]
	return ok
}

func checkLabelsDef(def MetricDef, labels map[string]string) error {
	allowed := make(map[string]struct{}, len(def.Labels))
	for _, l := range def.Labels {
		allowed[l] = struct{}{}
	}
	for k := range labels {
		if ForbiddenLabel(k) {
			return labelError("forbidden_label")
		}
		if _, ok := allowed[k]; !ok {
			return labelError("unknown_label")
		}
	}
	return nil
}

type labelError string

func (e labelError) Error() string { return string(e) }

// LabelReason is the bounded drop reason for a rejected sample.
func LabelReason(err error) string {
	if err == nil {
		return ""
	}
	if r, ok := err.(labelError); ok {
		return string(r)
	}
	return "invalid"
}

// PacketDecision collapses a packet-path outcome to a catalog decision label.
func PacketDecision(d string) string {
	switch strings.ToLower(strings.TrimSpace(d)) {
	case "serve", "drop", "kod", "ignore", "allowlist", "admission", "unmatched",
		"short", "oversize", "version", "mode", "zero_xmit":
		return strings.ToLower(strings.TrimSpace(d))
	default:
		return "drop"
	}
}

// AuthFailureReason collapses an auth failure to a bounded label.
func AuthFailureReason(reason string) string {
	switch strings.ToLower(strings.TrimSpace(reason)) {
	case "missing", "invalid", "denied":
		return strings.ToLower(strings.TrimSpace(reason))
	default:
		return "invalid"
	}
}

// CodeClass collapses an HTTP status to a bounded class.
func CodeClass(status int) string {
	switch {
	case status >= 500:
		return "5xx"
	case status >= 400:
		return "4xx"
	case status >= 300:
		return "3xx"
	case status >= 200:
		return "2xx"
	default:
		return "1xx"
	}
}

// ApplyResult collapses a mutation outcome to a bounded label.
func ApplyResult(result string) string {
	switch strings.ToLower(strings.TrimSpace(result)) {
	case "ok", "error", "conflict":
		return strings.ToLower(strings.TrimSpace(result))
	default:
		if result == "" {
			return "ok"
		}
		return "error"
	}
}
