package rest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/app"
	"github.com/hilather/go-lab-ntp/internal/auth"
	"github.com/hilather/go-lab-ntp/internal/capabilities"
	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/model"
)

func TestCharacterizeRESTAuthText(t *testing.T) {
	s, svc := newTestServer(t)
	rev := string(svc.Active().Revision)
	applyBody := applyRestrictBody(rev, "characterize-auth")

	ok := callREST(t, s, http.MethodGet, "/v1/state", "", headerAuth(testToken), nil)
	if ok.StatusCode != http.StatusOK {
		t.Fatalf("admin read %d %s", ok.StatusCode, mustBody(t, ok))
	}
	wrote := callREST(t, s, http.MethodPost, "/v1/changes:apply", applyBody, headerAuth(testToken), nil)
	if wrote.StatusCode != http.StatusOK {
		t.Fatalf("admin write %d %s", wrote.StatusCode, mustBody(t, wrote))
	}

	assertProblem(t, callREST(t, s, http.MethodGet, "/v1/state", "", map[string]string{"Authorization": "Bearer nope"}, nil),
		http.StatusUnauthorized, domainerr.CodeUnauthenticated, "Unauthenticated", "authentication required")
	assertProblem(t, callREST(t, s, http.MethodGet, "/v1/state", "", nil, nil),
		http.StatusUnauthorized, domainerr.CodeUnauthenticated, "Unauthenticated", "authentication required")
	assertProblem(t, callREST(t, s, http.MethodPost, "/v1/changes:apply", applyBody, nil, nil),
		http.StatusUnauthorized, domainerr.CodeUnauthenticated, "Unauthenticated", "authentication required")
	basic := callREST(t, s, http.MethodGet, "/v1/state", "", map[string]string{"Authorization": "Basic YWRtaW46c2VjcmV0"}, nil)
	if strings.Contains(mustBody(t, basic), "MCP accepts bearer tokens only") {
		t.Fatal("REST Basic must not use the MCP sentence")
	}
	assertProblem(t, basic, http.StatusUnauthorized, domainerr.CodeUnauthenticated, "Unauthenticated", "authentication required")
	assertProblem(t, callREST(t, s, http.MethodGet, "/v1/state", "", map[string]string{"Authorization": "Token abc"}, nil),
		http.StatusUnauthorized, domainerr.CodeUnauthenticated, "Unauthenticated", "authentication required")

	nilSrv := newAuthServer(t, svc, nil)
	assertProblem(t, callREST(t, nilSrv, http.MethodGet, "/v1/state", "", headerAuth(testToken), nil),
		http.StatusUnauthorized, domainerr.CodeUnauthenticated, "Unauthenticated", "authentication required")

	viewerSvc := bootTestApp(t)
	viewer := newAuthServer(t, viewerSvc, auth.Static(testToken, "viewer", model.RoleViewer))
	assertProblem(t, callREST(t, viewer, http.MethodPost, "/v1/changes:apply", applyRestrictBody(string(viewerSvc.Active().Revision), "viewer"), headerAuth(testToken), nil),
		http.StatusForbidden, domainerr.CodeForbidden, "Forbidden", "missing scope "+model.ScopeNTPAdmin)

	originSrv := newOriginServer(t, svc, []string{"https://lab.example"})
	assertProblem(t, callREST(t, originSrv, http.MethodGet, "/v1/state", "", headerAuth(testToken), map[string]string{"Origin": "https://evil.example"}),
		http.StatusForbidden, domainerr.CodeForbidden, "Forbidden", "origin is not allowed")
	assertProblem(t, callREST(t, originSrv, http.MethodGet, "/v1/state", "", headerAuth(testToken), map[string]string{"Origin": "http://"}),
		http.StatusForbidden, domainerr.CodeForbidden, "Forbidden", "origin is not allowed")
	allowed := callREST(t, originSrv, http.MethodGet, "/v1/state", "", headerAuth(testToken), map[string]string{"Origin": "https://lab.example"})
	if allowed.StatusCode != http.StatusOK {
		t.Fatalf("allowed origin %d %s", allowed.StatusCode, mustBody(t, allowed))
	}

	created := callREST(t, s, http.MethodPost, "/v1/session", "", headerAuth(testToken), nil)
	if created.StatusCode != http.StatusOK {
		t.Fatalf("session %d %s", created.StatusCode, mustBody(t, created))
	}
	cookie := created.Cookies()
	if len(cookie) != 1 || cookie[0].Name != auth.CookieName {
		t.Fatalf("set-cookie %+v", cookie)
	}
	csrf := decodeMap(t, created)["csrf"].(string)
	cookieHdr := map[string]string{"Cookie": cookie[0].Name + "=" + cookie[0].Value}
	freshRev := string(svc.Active().Revision)
	assertProblem(t, callREST(t, s, http.MethodPost, "/v1/changes:apply", applyRestrictBody(freshRev, "csrf-missing"), cookieHdr, nil),
		http.StatusForbidden, domainerr.CodeForbidden, "Forbidden", "CSRF token is missing or invalid")
	assertProblem(t, callREST(t, s, http.MethodPost, "/v1/changes:apply", applyRestrictBody(freshRev, "csrf-wrong"), cookieHdr, map[string]string{auth.CSRFHeader: "not-the-token"}),
		http.StatusForbidden, domainerr.CodeForbidden, "Forbidden", "CSRF token is missing or invalid")
	right := callREST(t, s, http.MethodPost, "/v1/changes:apply", applyRestrictBody(freshRev, "csrf-right"), cookieHdr, map[string]string{auth.CSRFHeader: csrf})
	if right.StatusCode != http.StatusOK {
		t.Fatalf("right csrf %d %s", right.StatusCode, mustBody(t, right))
	}
}

func newAuthServer(t *testing.T, svc *app.App, verifier *auth.Verifier) *Server {
	t.Helper()
	s, err := New(Config{Service: svc, RatePerSec: -1, Auth: verifier})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func newOriginServer(t *testing.T, svc *app.App, origins []string) *Server {
	t.Helper()
	s, err := New(Config{
		Service:        svc,
		RatePerSec:     -1,
		Auth:           auth.Static(testToken, "admin", model.RoleAdministrator),
		AllowedOrigins: origins,
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func headerAuth(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

func applyRestrictBody(rev, key string) string {
	return `{"expectedRevision":"` + rev + `","idempotencyKey":"` + key + `","reason":"characterize","operations":[{"op":"replaceRestrict","restrict":{"default":"limited","kod":true}}]}`
}

func callREST(t *testing.T, s *Server, method, path, body string, headers map[string]string, extra map[string]string) *http.Response {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.RemoteAddr = "192.0.2.1:1234"
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	for k, v := range extra {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	return w.Result()
}

func mustBody(t *testing.T, r *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	r.Body = io.NopCloser(strings.NewReader(string(b)))
	return string(b)
}

func assertProblem(t *testing.T, r *http.Response, status int, code domainerr.Code, title, detail string) {
	t.Helper()
	defer func() { _ = r.Body.Close() }()
	if r.StatusCode != status {
		b, _ := io.ReadAll(r.Body)
		t.Fatalf("status %d want %d body %s", r.StatusCode, status, b)
	}
	if ct := r.Header.Get("Content-Type"); ct != capabilities.ProblemContentType {
		t.Fatalf("content-type %q", ct)
	}
	if status == http.StatusUnauthorized {
		if got := r.Header.Get("WWW-Authenticate"); got != `Bearer realm="labntp"` {
			t.Fatalf("www-authenticate %q", got)
		}
	}
	var p capabilities.Problem
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		t.Fatal(err)
	}
	if p.Code != code || p.Title != title || p.Detail != detail || p.Status != status {
		t.Fatalf("problem %+v", p)
	}
}
