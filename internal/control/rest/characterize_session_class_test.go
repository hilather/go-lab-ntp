package rest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/app"
	"github.com/hilather/go-lab-ntp/internal/auth"
	"github.com/hilather/go-lab-ntp/internal/model"
)

func TestCharacterizeLoopbackSessionActorClass(t *testing.T) {
	const yamlBody = `apiVersion: labntp.dev/v1alpha1
kind: LabNTP
metadata:
  name: loopback-session
spec:
  auth:
    mode: dev-loopback-unauth
    tokens: []
  filters:
    - name: default
      match:
        cidrs: ["0.0.0.0/0", "::/0"]
      view:
        mode: follow-real
`
	svc := bootYAMLApp(t, yamlBody)
	verifier, err := auth.FromSpec(svc.Active().Canonical.Spec.Auth)
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(Config{Service: svc, Auth: verifier, RatePerSec: -1})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/session", nil)
	req.RemoteAddr = "127.0.0.1:9"
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	res := w.Result()
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("session create %d %s", res.StatusCode, b)
	}
	cookies := res.Cookies()
	if len(cookies) != 1 || cookies[0].Name != auth.CookieName {
		t.Fatalf("cookie %+v", cookies)
	}
	var created sessionCreateJSON
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.CSRF == "" {
		t.Fatal("missing csrf")
	}

	body := applyRestrictBody(string(svc.Active().Revision), "loopback-cookie")
	mut := httptest.NewRequest(http.MethodPost, "/v1/changes:apply", strings.NewReader(body))
	mut.RemoteAddr = "127.0.0.1:9"
	mut.Header.Set("Content-Type", "application/json")
	mut.Header.Set("Cookie", cookies[0].Name+"="+cookies[0].Value)
	mut.Header.Set(auth.CSRFHeader, created.CSRF)
	mw := httptest.NewRecorder()
	s.Handler().ServeHTTP(mw, mut)
	if mw.Code != http.StatusOK {
		t.Fatalf("cookie apply %d %s", mw.Code, mw.Body.String())
	}

	list, err := svc.QueryAudit(context.Background(), app.Actor{ID: "loopback", Scopes: []string{model.ScopeNTPAuditRead}}, app.AuditQuery{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, ev := range list.Events {
		if ev.Capability != "changes.apply" {
			continue
		}
		found = true
		if ev.ActorClass != auth.ClassToken {
			t.Fatalf("actorClass %q want token", ev.ActorClass)
		}
		if ev.ActorID != "loopback" {
			t.Fatalf("actorId %q want loopback", ev.ActorID)
		}
	}
	if !found {
		t.Fatalf("no changes.apply audit row: %+v", list.Events)
	}
}
