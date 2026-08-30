package rest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/auth"
	"github.com/hilather/go-lab-ntp/internal/capabilities"
	"github.com/hilather/go-lab-ntp/internal/domainerr"
)

func TestHealthUnauthenticated(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/health/live", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("live %d", w.Code)
	}
}

func TestBearerRequired(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/state", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "problem+json") {
		t.Fatalf("ct %s", ct)
	}
	if wa := w.Header().Get("WWW-Authenticate"); !strings.Contains(wa, "Bearer") {
		t.Fatalf("www-authenticate %s", wa)
	}
	body, _ := io.ReadAll(w.Body)
	if !strings.Contains(string(body), `"code":"unauthenticated"`) {
		t.Fatalf("%s", body)
	}
}

func TestNilVerifierDenies(t *testing.T) {
	svc := bootTestApp(t)
	s, err := New(Config{Service: svc, RatePerSec: -1})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/state", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("nil verifier must deny, got %d", w.Code)
	}
}

func TestNoBasic(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/state", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("%d", w.Code)
	}
}

func TestFeaturesAndPreview(t *testing.T) {
	s, _ := newTestServer(t)
	resp := doJSON(t, s, http.MethodGet, "/v1/features", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatal(resp.StatusCode)
	}
	m := decodeMap(t, resp)
	items, _ := m["items"].([]any)
	if len(items) != len(capabilities.Features()) {
		t.Fatalf("features %d", len(items))
	}
	resp = doJSON(t, s, http.MethodGet, "/v1/views/preview?ip=10.99.42.20", "")
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("%d %s", resp.StatusCode, b)
	}
	m = decodeMap(t, resp)
	if m["filter"] != "default" {
		t.Fatalf("%v", m)
	}
}

func TestCSRFRequiredOnCookieMutation(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/session", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("session create %d", w.Code)
	}
	var cookie string
	for _, c := range w.Result().Cookies() {
		if c.Name == auth.CookieName {
			cookie = c.Value
		}
	}
	if cookie == "" {
		t.Fatal("cookie")
	}
	req = httptest.NewRequest(http.MethodPost, "/v1/state:reset", strings.NewReader(`{"reason":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: cookie})
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("csrf missing want 403 got %d", w.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/v1/state:reset", strings.NewReader(`{"reason":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(auth.CSRFHeader, "not-the-token")
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: cookie})
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("bad csrf %d", w.Code)
	}
}

func TestProblemJSONCode(t *testing.T) {
	p := capabilities.ProblemFrom(domainerr.ValidationFailed("x"), "urn:labntp:request:1")
	if p.Status != 400 || p.Code != domainerr.CodeValidationFailed {
		t.Fatalf("%+v", p)
	}
}

func TestFilterPutRoundTripDurationStrings(t *testing.T) {
	svc := bootYAMLApp(t, `apiVersion: labntp.dev/v1alpha1
kind: LabNTP
metadata:
  name: two-filters
spec:
  filters:
    - name: tester
      enabled: true
      match:
        cidrs: ["10.99.42.20/32"]
      view:
        mode: follow-real
    - name: default
      enabled: true
      match:
        cidrs: ["0.0.0.0/0", "::/0"]
      view:
        mode: follow-real
`)
	s, _ := newServerFor(t, svc)

	list := doJSON(t, s, http.MethodGet, "/v1/filters", "")
	if list.StatusCode != http.StatusOK {
		t.Fatalf("GET /v1/filters %d", list.StatusCode)
	}
	listed := decodeMap(t, list)
	items, _ := listed["items"].([]any)
	if len(items) == 0 {
		t.Fatal("no filters")
	}
	var follow map[string]any
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		view, _ := item["view"].(map[string]any)
		if view["mode"] == "follow-real" && item["name"] == "tester" {
			follow = item
			break
		}
	}
	if follow == nil {
		t.Fatal("no follow-real filter")
	}
	view, _ := follow["view"].(map[string]any)
	if view["offset"] != "0s" {
		t.Fatalf("GET offset=%v want 0s string", view["offset"])
	}

	st := doJSON(t, s, http.MethodGet, "/v1/state", "")
	if st.StatusCode != http.StatusOK {
		t.Fatalf("GET /v1/state %d", st.StatusCode)
	}
	state := decodeMap(t, st)
	rev, _ := state["runtimeRevision"].(string)
	if rev == "" {
		t.Fatal("missing runtimeRevision")
	}

	enabled, _ := follow["enabled"].(bool)
	follow["enabled"] = !enabled
	name, _ := follow["name"].(string)
	putBody, err := json.Marshal(map[string]any{
		"expectedRevision": rev,
		"reason":           "ui: disable filter",
		"filter":           follow,
	})
	if err != nil {
		t.Fatal(err)
	}
	put := doJSON(t, s, http.MethodPut, "/v1/filters/"+name, string(putBody))
	if put.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(put.Body)
		t.Fatalf("PUT duration strings %d %s", put.StatusCode, b)
	}
	_ = put.Body.Close()

	numeric := map[string]any{
		"expectedRevision": rev,
		"reason":           "ui: numeric offset",
		"filter": map[string]any{
			"name":    name,
			"enabled": enabled,
			"match":   follow["match"],
			"view": map[string]any{
				"mode":           "follow-real",
				"offset":         0,
				"leap":           view["leap"],
				"stratum":        view["stratum"],
				"refid":          view["refid"],
				"precision":      view["precision"],
				"rootDelay":      "0s",
				"rootDispersion": "0s",
				"jitter":         "0s",
			},
		},
	}
	numBody, err := json.Marshal(numeric)
	if err != nil {
		t.Fatal(err)
	}
	bad := doJSON(t, s, http.MethodPut, "/v1/filters/"+name, string(numBody))
	if bad.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(bad.Body)
		t.Fatalf("PUT numeric offset want 400 got %d %s", bad.StatusCode, b)
	}
	_ = bad.Body.Close()
}

func TestStatusRevisionsArePascalCase(t *testing.T) {
	s, _ := newTestServer(t)
	resp := doJSON(t, s, http.MethodGet, "/v1/status", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /v1/status %d", resp.StatusCode)
	}
	m := decodeMap(t, resp)
	rev, ok := m["revisions"].(map[string]any)
	if !ok {
		t.Fatalf("revisions type %T", m["revisions"])
	}
	if _, ok := rev["BootstrapRevision"]; !ok {
		t.Fatalf("status.revisions missing BootstrapRevision: %v", rev)
	}
	if _, ok := rev["runtimeRevision"]; ok {
		t.Fatalf("status.revisions unexpectedly camelCase runtimeRevision: %v", rev)
	}
}
