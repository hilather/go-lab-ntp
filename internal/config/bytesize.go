package config

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/hilather/go-lab-ntp/internal/domainerr"
)

var sizeFields = map[string]bool{
	"bodyLimit": true,
}

// ParseByteSize accepts an integer plus an IEC unit (B, KiB, MiB, GiB, TiB).
func ParseByteSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty byte size")
	}
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 || i == len(s) {
		return 0, fmt.Errorf("byte size must use an IEC unit (for example 1MiB)")
	}
	n, err := strconv.ParseInt(s[:i], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("byte size integer overflow")
	}
	unit := s[i:]
	var mul int64
	switch strings.ToLower(unit) {
	case "b":
		mul = 1
	case "kib":
		mul = 1 << 10
	case "mib":
		mul = 1 << 20
	case "gib":
		mul = 1 << 30
	case "tib":
		mul = 1 << 40
	default:
		return 0, fmt.Errorf("byte size must use an IEC unit (B, KiB, MiB, GiB, TiB)")
	}
	if n < 0 {
		return 0, fmt.Errorf("byte size must be >= 0")
	}
	if mul > 1 && n > (1<<63-1)/mul {
		return 0, fmt.Errorf("byte size integer overflow")
	}
	return n * mul, nil
}

// FormatByteSize is the canonical IEC spelling used in export and hashes.
func FormatByteSize(n int64) string {
	if n == 0 {
		return "0B"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var s string
	switch {
	case n%(1<<40) == 0:
		s = strconv.FormatInt(n/(1<<40), 10) + "TiB"
	case n%(1<<30) == 0:
		s = strconv.FormatInt(n/(1<<30), 10) + "GiB"
	case n%(1<<20) == 0:
		s = strconv.FormatInt(n/(1<<20), 10) + "MiB"
	case n%(1<<10) == 0:
		s = strconv.FormatInt(n/(1<<10), 10) + "KiB"
	default:
		s = strconv.FormatInt(n, 10) + "B"
	}
	if neg {
		return "-" + s
	}
	return s
}

func convertByteSizes(v any, path string) []domainerr.FieldViolation {
	switch x := v.(type) {
	case map[string]any:
		var vs []domainerr.FieldViolation
		for k, child := range x {
			p := joinPath(path, k)
			if sizeFields[k] {
				if viol, ok := coerceByteSizeField(x, k, child, p); !ok {
					vs = append(vs, viol)
				}
				continue
			}
			vs = append(vs, convertByteSizes(child, p)...)
		}
		return vs
	case []any:
		var vs []domainerr.FieldViolation
		for i, child := range x {
			vs = append(vs, convertByteSizes(child, indexPath(path, i))...)
		}
		return vs
	default:
		return nil
	}
}

func coerceByteSizeField(obj map[string]any, key string, child any, path string) (domainerr.FieldViolation, bool) {
	if child == nil {
		return domainerr.FieldViolation{}, true
	}
	switch n := child.(type) {
	case string:
		if looksLikeBareNumber(n) {
			// Bare numeric strings are accepted as bytes (D: bodyLimit: 1048576).
			i, err := strconv.ParseInt(n, 10, 64)
			if err != nil || i < 0 {
				return domainerr.FieldViolation{
					Path:    path,
					Code:    violationInvalidValue,
					Message: "byte size must be >= 0",
				}, false
			}
			obj[key] = i
			return domainerr.FieldViolation{}, true
		}
		sz, err := ParseByteSize(n)
		if err != nil {
			return domainerr.FieldViolation{
				Path:    path,
				Code:    violationInvalidValue,
				Message: err.Error(),
			}, false
		}
		obj[key] = sz
		return domainerr.FieldViolation{}, true
	default:
		if i, ok := jsonNumberInt(n); ok {
			if i < 0 {
				return domainerr.FieldViolation{
					Path:    path,
					Code:    violationInvalidValue,
					Message: "byte size must be >= 0",
				}, false
			}
			obj[key] = i
			return domainerr.FieldViolation{}, true
		}
		return domainerr.FieldViolation{
			Path:    path,
			Code:    violationInvalidValue,
			Message: "byte size must be an IEC string such as 1MiB or a non-negative integer",
		}, false
	}
}

func convertByteSizeNumbersToStrings(v any) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			if sizeFields[k] {
				if n, ok := jsonNumberInt(child); ok {
					x[k] = FormatByteSize(n)
				}
			}
			convertByteSizeNumbersToStrings(child)
		}
	case []any:
		for _, child := range x {
			convertByteSizeNumbersToStrings(child)
		}
	}
}

func jsonNumberInt(v any) (int64, bool) {
	switch n := v.(type) {
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, false
		}
		return i, true
	case int64:
		return n, true
	case float64:
		return int64(n), true
	case int:
		return int64(n), true
	default:
		return 0, false
	}
}

func looksLikeBareNumber(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
