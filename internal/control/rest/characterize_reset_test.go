package rest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/capabilities"
	"github.com/hilather/go-lab-ntp/internal/domainerr"
)

func TestCharacterizeResetFailureText(t *testing.T) {
	svc, s, _, _, boot := bootReload(t)
	writeReloadBoot(t, dirOf(boot), `    mode: bearer
    tokens: []
`)
	before := string(svc.Active().Revision)
	req := httptest.NewRequest(http.MethodPost, "/v1/state:reset", strings.NewReader(`{"reason":"empty bearer"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+reloadSecret)
	req.RemoteAddr = "192.0.2.1:1234"
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	res := w.Result()
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d body %s", res.StatusCode, b)
	}
	var p capabilities.Problem
	if err := json.NewDecoder(res.Body).Decode(&p); err != nil {
		t.Fatal(err)
	}
	if p.Code != domainerr.CodeValidationFailed || p.Title != "Validation failed" || p.Detail != "management auth cannot be loaded" {
		t.Fatalf("problem %+v", p)
	}
	if len(p.FieldViolations) != 1 {
		t.Fatalf("violations %+v", p.FieldViolations)
	}
	v := p.FieldViolations[0]
	if v.Path != "spec.auth.tokens" || v.Code != "invalid_value" || v.Message != "spec.auth.mode bearer requires at least one usable token" {
		t.Fatalf("violation %+v", v)
	}
	if got := string(svc.Active().Revision); got != before {
		t.Fatalf("revision changed %s -> %s", before, got)
	}
}

func dirOf(path string) string {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return "."
	}
	return path[:i]
}
