package rest

import (
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
