// Package difftranscript builds a normalized management transcript and compares two runs.
package difftranscript

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	sha256Re   = regexp.MustCompile(`sha256:[0-9a-fA-F]{64}`)
	auditRe    = regexp.MustCompile(`\baud-[0-9]+\b`)
	cookieRe   = regexp.MustCompile(`(?i)(labntp_session=)([0-9a-fA-F]{64})`)
	csrfJSONRe = regexp.MustCompile(`(?i)("csrf"\s*:\s*")([0-9a-fA-F]{64})(")`)
	csrfHdrRe  = regexp.MustCompile(`(?i)(X-LabNTP-CSRF:\s*)([0-9a-fA-F]{64})`)
	requestURN = regexp.MustCompile(`(urn:labntp:request:)([^\s"\\]+)`)
	requestHdr = regexp.MustCompile(`(?i)(X-Request-ID:\s*)(\S+)`)
	// reqStable is the harness X-Request-ID, req-<block>-<n>. ntp copies a
	// non-empty incoming header (internal/control/rest/server.go requestID
	// and internal/control/mcp/server.go requestID) and only falls back to
	// crypto/rand hex when the header is absent. The req-N-N form is
	// deterministic per scenario and must stay verbatim. Any other value is
	// still the named <request:n> rule.
	reqStable   = regexp.MustCompile(`^req-\d+-\d+$`)
	etagRe      = regexp.MustCompile(`(?i)(ETag:\s*)(\S+)`)
	hex64Re     = regexp.MustCompile(`\b[0-9a-fA-F]{64}\b`)
	hex32JSONRe = regexp.MustCompile(`"([0-9a-fA-F]{32})"`)
	commitRe    = regexp.MustCompile(`"commit"\s*:\s*"[^"]*"`)
	buildTimeRe = regexp.MustCompile(`"buildTime"\s*:\s*"[^"]*"`)
	versionDev  = regexp.MustCompile(`"version"\s*:\s*"dev"`)
	rfc3339Re   = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})`)
	unixFieldRe = regexp.MustCompile(`"(time|at|ts|createdAt|expiresAt|lastSeen|generatedAt)"\s*:\s*-?\d+(?:\.\d+)?`)
	dateHdrRe   = regexp.MustCompile(`(?m)^Date:.*\r?\n`)
	histDropRe  = regexp.MustCompile(`(?m)^labntp_\S*(?:_sum|_bucket)(?:\{[^}]*\})?\s+\S+\r?\n`)
	gaugeDropRe = regexp.MustCompile(`(?m)^labntp_udp_inflight(?:\{[^}]*\})?\s+\S+\r?\n`)
)

// Normalize rewrites nondeterministic bytes in place. It does not re-encode JSON.
// Revisions (sha256: plus 64 hex digits) are preserved. addrs are listener
// addresses, longest first, replaced with <addr>. Harness source addresses
// are not in addrs and stay visible.
func Normalize(in []byte, addrs []string) []byte {
	s := string(in)
	var saved []string
	s = sha256Re.ReplaceAllStringFunc(s, func(m string) string {
		saved = append(saved, m)
		return "\x00SHA" + strconv.Itoa(len(saved)-1) + "\x00"
	})

	seq := map[string]map[string]string{}
	next := func(kind, raw string) string {
		m := seq[kind]
		if m == nil {
			m = map[string]string{}
			seq[kind] = m
		}
		if v, ok := m[raw]; ok {
			return v
		}
		v := "<" + kind + ":" + strconv.Itoa(len(m)+1) + ">"
		m[raw] = v
		return v
	}

	s = auditRe.ReplaceAllStringFunc(s, func(m string) string { return next("audit", m) })
	s = cookieRe.ReplaceAllStringFunc(s, func(m string) string {
		parts := cookieRe.FindStringSubmatch(m)
		return parts[1] + next("cookie", strings.ToLower(parts[2]))
	})
	s = csrfJSONRe.ReplaceAllStringFunc(s, func(m string) string {
		parts := csrfJSONRe.FindStringSubmatch(m)
		return parts[1] + next("csrf", strings.ToLower(parts[2])) + parts[3]
	})
	s = csrfHdrRe.ReplaceAllStringFunc(s, func(m string) string {
		parts := csrfHdrRe.FindStringSubmatch(m)
		return parts[1] + next("csrf", strings.ToLower(parts[2]))
	})
	s = requestURN.ReplaceAllStringFunc(s, func(m string) string {
		parts := requestURN.FindStringSubmatch(m)
		if reqStable.MatchString(parts[2]) {
			return m
		}
		return parts[1] + next("request", parts[2])
	})
	s = requestHdr.ReplaceAllStringFunc(s, func(m string) string {
		parts := requestHdr.FindStringSubmatch(m)
		if reqStable.MatchString(parts[2]) {
			return m
		}
		return parts[1] + next("request", parts[2])
	})
	s = etagRe.ReplaceAllStringFunc(s, func(m string) string {
		parts := etagRe.FindStringSubmatch(m)
		return parts[1] + next("etag", parts[2])
	})
	s = hex64Re.ReplaceAllStringFunc(s, func(m string) string { return next("secret", strings.ToLower(m)) })
	s = hex32JSONRe.ReplaceAllStringFunc(s, func(m string) string {
		parts := hex32JSONRe.FindStringSubmatch(m)
		return `"` + next("session", strings.ToLower(parts[1])) + `"`
	})

	for i, raw := range saved {
		s = strings.ReplaceAll(s, "\x00SHA"+strconv.Itoa(i)+"\x00", raw)
	}

	ordered := append([]string(nil), addrs...)
	sort.Slice(ordered, func(i, j int) bool { return len(ordered[i]) > len(ordered[j]) })
	for _, addr := range ordered {
		if addr == "" {
			continue
		}
		s = strings.ReplaceAll(s, addr, "<addr>")
	}

	s = commitRe.ReplaceAllString(s, `"commit":"<build>"`)
	s = buildTimeRe.ReplaceAllString(s, `"buildTime":"<build>"`)
	s = versionDev.ReplaceAllString(s, `"version":"<build>"`)
	s = rfc3339Re.ReplaceAllString(s, "<ts>")
	// Keep a JSON number. "<ts>" would change the type of a numeric time.
	s = unixFieldRe.ReplaceAllString(s, `"$1":0`)
	s = dateHdrRe.ReplaceAllString(s, "")
	s = histDropRe.ReplaceAllString(s, "")
	s = gaugeDropRe.ReplaceAllString(s, "")
	return []byte(s)
}

// SelfDiff normalizes both transcripts. A remaining byte difference is not
// covered by a named rule.
func SelfDiff(a, b []byte, addrs []string) error {
	na := Normalize(a, addrs)
	nb := Normalize(b, addrs)
	if bytes.Equal(na, nb) {
		return nil
	}
	return fmt.Errorf("self-diff: difference is not covered by a named rule\n%s", snippet(na, nb))
}

func snippet(a, b []byte) string {
	la := strings.Split(string(a), "\n")
	lb := strings.Split(string(b), "\n")
	n := len(la)
	if len(lb) < n {
		n = len(lb)
	}
	for i := 0; i < n; i++ {
		if la[i] != lb[i] {
			return fmt.Sprintf("line %d\n- %s\n+ %s", i+1, la[i], lb[i])
		}
	}
	return fmt.Sprintf("length %d vs %d", len(a), len(b))
}
