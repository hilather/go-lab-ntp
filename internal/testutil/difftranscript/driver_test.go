package difftranscript

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
)

var (
	modeFlag    = flag.String("mode", "", "scenario runs the two-process harness; empty runs unit tests")
	binaryFlag  = flag.String("binary", "", "labntp binary for -mode=scenario")
	fixtureFlag = flag.String("fixture", "", "fixed fixture directory shared by every run")
	outFlag     = flag.String("out", "", "normalized transcript path for -mode=scenario")
)

func TestMain(m *testing.M) {
	if !flag.Parsed() {
		flag.Parse()
	}
	if *modeFlag == "scenario" {
		if err := runScenario(*binaryFlag, *fixtureFlag, *outFlag); err != nil {
			fmt.Fprintf(os.Stderr, "difftranscript: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestNormalize(t *testing.T) {
	const sha = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cookie := strings.Repeat("a", 64)
	csrf := strings.Repeat("b", 64)
	raw := strings.Join([]string{
		"Date: Thu, 01 Jan 2026 00:00:00 GMT",
		"Set-Cookie: labntp_session=" + cookie + "; Path=/; Max-Age=43200; HttpOnly; SameSite=Lax",
		"X-LabNTP-CSRF: " + csrf,
		"ETag: W/\"etag-1\"",
		"X-Request-ID: req-1-1",
		`{"revision":"` + sha + `","version":"dev","commit":"abcdef012345","buildTime":"2026-01-02T03:04:05Z","apiVersion":"v1alpha1","versionName":"v1alpha1","csrf":"` + csrf + `","expiresAt":"2026-10-09T12:00:00.5Z","createdAt":1700000000,"id":"aud-3","instance":"urn:labntp:request:req-1-1","public":"cccccccccccccccccccccccccccccccc","listen":"127.0.0.1:40000","yamlListen":"127.0.0.1:9"}`,
		"Expires: Thu, 01 Jan 1970 00:00:00 GMT",
		"labntp_http_requests_total{code_class=\"2xx\"} 1",
		"labntp_handler_seconds_sum 0.2",
		"labntp_handler_seconds_bucket{le=\"+Inf\"} 1",
		"labntp_handler_seconds_count 1",
		"labntp_udp_inflight 0",
		`"generation":2`,
		`"version":"v1alpha1"`,
	}, "\n") + "\n"

	got := string(Normalize([]byte(raw), []string{"127.0.0.1:40000"}))
	if strings.Contains(got, "Date:") {
		t.Fatalf("Date header kept:\n%s", got)
	}
	if strings.Contains(got, "aaaaaaaa") || strings.Contains(got, "bbbbbbbb") {
		t.Fatalf("raw cookie or csrf kept:\n%s", got)
	}
	if !strings.Contains(got, "<cookie:1>") || !strings.Contains(got, "<csrf:1>") {
		t.Fatalf("cookie/csrf kinds missing:\n%s", got)
	}
	if !strings.Contains(got, sha) {
		t.Fatalf("revision rewritten:\n%s", got)
	}
	if !strings.Contains(got, `"commit":"<build>"`) || !strings.Contains(got, `"buildTime":"<build>"`) || !strings.Contains(got, `"version":"<build>"`) {
		t.Fatalf("build fields missing:\n%s", got)
	}
	if !strings.Contains(got, `"version":"v1alpha1"`) {
		t.Fatalf("capability version rewritten:\n%s", got)
	}
	if strings.Contains(got, "2026-10-09") || strings.Contains(got, "1700000000") {
		t.Fatalf("timestamp kept:\n%s", got)
	}
	if !strings.Contains(got, "Thu, 01 Jan 1970 00:00:00 GMT") {
		t.Fatalf("HTTP-date Expires rewritten:\n%s", got)
	}
	if strings.Contains(got, "127.0.0.1:40000") {
		t.Fatalf("listener kept:\n%s", got)
	}
	if !strings.Contains(got, "127.0.0.1:9") {
		t.Fatalf("configured address rewritten:\n%s", got)
	}
	if !strings.Contains(got, "<audit:1>") || !strings.Contains(got, "<request:1>") || !strings.Contains(got, "<session:1>") || !strings.Contains(got, "<etag:1>") {
		t.Fatalf("id kinds missing:\n%s", got)
	}
	if strings.Contains(got, "_sum") || strings.Contains(got, "_bucket") || strings.Contains(got, "udp_inflight") {
		t.Fatalf("histogram or gauge kept:\n%s", got)
	}
	if !strings.Contains(got, "labntp_handler_seconds_count 1") {
		t.Fatalf("histogram count dropped:\n%s", got)
	}
	if !strings.Contains(got, `"generation":2`) {
		t.Fatalf("generation rewritten:\n%s", got)
	}
	if !strings.Contains(got, "SameSite=Lax") {
		t.Fatalf("SameSite rewritten:\n%s", got)
	}
}

func TestSelfDiffRules(t *testing.T) {
	addrs := []string{"127.0.0.1:1111"}
	aCookie := strings.Repeat("a", 64)
	bCookie := strings.Repeat("b", 64)
	a := []byte("Set-Cookie: labntp_session=" + aCookie + "; SameSite=Lax\n" +
		`{"at":"2020-01-02T03:04:05Z","listen":"127.0.0.1:1111"}` + "\n")
	b := []byte("Set-Cookie: labntp_session=" + bCookie + "; SameSite=Lax\n" +
		`{"at":"2030-05-06T07:08:09.123Z","listen":"127.0.0.1:1111"}` + "\n")
	if err := SelfDiff(a, b, addrs); err != nil {
		t.Fatal(err)
	}
	c := []byte("Set-Cookie: labntp_session=" + aCookie + "; SameSite=Strict\n" +
		`{"at":"2020-01-02T03:04:05Z","listen":"127.0.0.1:1111"}` + "\n")
	if err := SelfDiff(a, c, addrs); err == nil || !strings.Contains(err.Error(), "not covered by a named rule") {
		t.Fatalf("SameSite diff: %v", err)
	}
	originA := []byte(`{"detail":"origin is not allowed"}` + "\n")
	originB := []byte(`{"detail":"origin is rejected by policy"}` + "\n")
	if err := SelfDiff(originA, originB, nil); err == nil {
		t.Fatal("origin sentence was masked")
	}
	revA := []byte("sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\n")
	revB := []byte("sha256:fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210\n")
	if err := SelfDiff(revA, revB, nil); err == nil {
		t.Fatal("revision was masked")
	}
}
