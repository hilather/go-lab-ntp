package config

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/hilather/go-lab-ntp/internal/domainerr"
)

var durationFields = map[string]bool{
	"offset":         true,
	"rootDelay":      true,
	"rootDispersion": true,
	"jitter":         true,
}

// convertDurations rewrites ParseDuration strings to nanoseconds for
// encoding/json → time.Duration. Bare numbers (offset: 5) are rejected.
func convertDurations(v any, path string) []domainerr.FieldViolation {
	switch x := v.(type) {
	case map[string]any:
		var vs []domainerr.FieldViolation
		for k, child := range x {
			p := joinPath(path, k)
			if durationFields[k] {
				if viol, ok := coerceDurationField(x, k, child, p); !ok {
					vs = append(vs, viol)
				}
				continue
			}
			vs = append(vs, convertDurations(child, p)...)
		}
		return vs
	case []any:
		var vs []domainerr.FieldViolation
		for i, child := range x {
			vs = append(vs, convertDurations(child, indexPath(path, i))...)
		}
		return vs
	default:
		return nil
	}
}

func coerceDurationField(obj map[string]any, key string, child any, path string) (domainerr.FieldViolation, bool) {
	if child == nil {
		return domainerr.FieldViolation{}, true
	}
	switch n := child.(type) {
	case string:
		d, err := time.ParseDuration(n)
		if err != nil {
			return domainerr.FieldViolation{
				Path:    path,
				Code:    violationInvalidValue,
				Message: "duration must use Go time.ParseDuration syntax (for example -6m)",
			}, false
		}
		obj[key] = int64(d)
		return domainerr.FieldViolation{}, true
	default:
		return domainerr.FieldViolation{
			Path:    path,
			Code:    violationInvalidValue,
			Message: "duration must be a string such as 5s, not a bare number",
		}, false
	}
}

func convertDurationNumbersToStrings(v any) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			if durationFields[k] {
				if d, ok := jsonNumberDuration(child); ok {
					x[k] = FormatDuration(d)
				}
			}
			convertDurationNumbersToStrings(child)
		}
	case []any:
		for _, child := range x {
			convertDurationNumbersToStrings(child)
		}
	}
}

func jsonNumberDuration(v any) (time.Duration, bool) {
	switch n := v.(type) {
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, false
		}
		return time.Duration(i), true
	case int64:
		return time.Duration(n), true
	case float64:
		return time.Duration(n), true
	case int:
		return time.Duration(n), true
	default:
		return 0, false
	}
}

// FormatDuration is the canonical duration spelling used in export and hashes.
func FormatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	if d < 0 {
		return "-" + FormatDuration(-d)
	}
	if d%time.Hour == 0 {
		return strconv.FormatInt(int64(d/time.Hour), 10) + "h"
	}
	if d%time.Minute == 0 {
		return strconv.FormatInt(int64(d/time.Minute), 10) + "m"
	}
	if d%time.Second == 0 {
		return strconv.FormatInt(int64(d/time.Second), 10) + "s"
	}
	if d%time.Millisecond == 0 {
		return strconv.FormatInt(int64(d/time.Millisecond), 10) + "ms"
	}
	if d%time.Microsecond == 0 {
		return strconv.FormatInt(int64(d/time.Microsecond), 10) + "us"
	}
	return strconv.FormatInt(int64(d), 10) + "ns"
}
