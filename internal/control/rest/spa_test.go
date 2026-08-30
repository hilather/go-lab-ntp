package rest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/web"
)

func TestSPAFallbackWhenEnabled(t *testing.T) {
	s, _ := newTestServer(t)
	s.cfg.UI = web.NewHandler(nil)
	s.cfg.UIEnabled = func() bool { return true }

	got := doReq(t, s, http.MethodGet, "/", "")
	if got.Code != http.StatusOK {
		t.Fatalf("GET / code=%d body=%s", got.Code, got.Body.String())
	}
	if !strings.Contains(got.Body.String(), "LabNTP") {
		t.Fatalf("GET / body=%s", got.Body.String())
	}
	if ct := got.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("GET / content-type=%q", ct)
	}

	preview := doReq(t, s, http.MethodGet, "/preview", "")
	if preview.Code != http.StatusOK || !strings.Contains(preview.Body.String(), "LabNTP") {
		t.Fatalf("SPA fallback code=%d body=%s", preview.Code, preview.Body.String())
	}
}

func TestSPADisabledIs404(t *testing.T) {
	s, _ := newTestServer(t)
	s.cfg.UI = web.NewHandler(nil)
	s.cfg.UIEnabled = func() bool { return false }

	got := doReq(t, s, http.MethodGet, "/", "")
	if got.Code != http.StatusNotFound {
		t.Fatalf("GET / code=%d body=%s", got.Code, got.Body.String())
	}
	if ct := got.Header().Get("Content-Type"); !strings.Contains(ct, "problem+json") {
		t.Fatalf("content-type=%q", ct)
	}
	body := got.Body.String()
	if !strings.Contains(body, `"code":"not_found"`) {
		t.Fatalf("body=%s", body)
	}
	if strings.Contains(body, "<!doctype") {
		t.Fatalf("disabled UI served HTML: %s", body)
	}
}

func TestSPADoesNotCaptureAPI(t *testing.T) {
	s, _ := newTestServer(t)
	s.cfg.UI = web.NewHandler(nil)
	s.cfg.UIEnabled = func() bool { return true }

	got := doReq(t, s, http.MethodGet, "/v1/does-not-exist", "")
	if got.Code != http.StatusNotFound {
		t.Fatalf("code=%d", got.Code)
	}
	if ct := got.Header().Get("Content-Type"); !strings.Contains(ct, "problem+json") {
		t.Fatalf("content-type=%q", ct)
	}
	if strings.Contains(got.Body.String(), "<!doctype") {
		t.Fatal("API miss served HTML")
	}
}

func doReq(t *testing.T, s *Server, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	return w
}
