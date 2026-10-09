package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/auth"
	"github.com/hilather/go-lab-ntp/internal/model"
)

// TestRESTRateLimit429OmitsRetryAfter hits the management rate limiter.
// The 429 problem has no Retry-After header. Main does not send one.
func TestRESTRateLimit429OmitsRetryAfter(t *testing.T) {
	svc := bootTestApp(t)
	s, err := New(Config{
		Service:    svc,
		Auth:       auth.Static(testToken, "admin", model.RoleAdministrator),
		RatePerSec: 0.001,
		RateBurst:  1,
	})
	if err != nil {
		t.Fatal(err)
	}

	var limited *httptest.ResponseRecorder
	for i := 0; i < 4; i++ {
		req := httptest.NewRequest(http.MethodGet, "/v1/version", nil)
		req.RemoteAddr = "203.0.113.50:4000"
		req.Header.Set("Authorization", "Bearer "+testToken)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, req)
		if w.Code == http.StatusTooManyRequests {
			limited = w
			break
		}
	}
	if limited == nil {
		t.Fatal("rate limiter did not return 429")
	}
	if got, ok := limited.Header()["Retry-After"]; ok {
		t.Fatalf("Retry-After = %q", got)
	}

	var prob struct {
		Status int    `json:"status"`
		Code   string `json:"code"`
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(limited.Body.Bytes(), &prob); err != nil {
		t.Fatal(err)
	}
	if prob.Status != http.StatusTooManyRequests || prob.Code != "rate_limited" || prob.Detail != "too many management requests" {
		t.Fatalf("problem %+v", prob)
	}
}

// TestRESTEchoesXRequestID checks the two places main copies a non-empty
// incoming X-Request-ID: the response header, and a problem body's
// urn:labntp:request: instance.
func TestRESTEchoesXRequestID(t *testing.T) {
	svc := bootTestApp(t)
	s, err := New(Config{
		Service:    svc,
		Auth:       auth.Static(testToken, "admin", model.RoleAdministrator),
		RatePerSec: -1,
	})
	if err != nil {
		t.Fatal(err)
	}

	const id = "pinned-request-id"
	req := httptest.NewRequest(http.MethodGet, "/v1/version", nil)
	req.Header.Set("X-Request-ID", id)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status %d body %s", w.Code, w.Body.Bytes())
	}
	if got := w.Header().Get("X-Request-ID"); got != id {
		t.Fatalf("header %q, want %q", got, id)
	}
	var prob struct {
		Instance string `json:"instance"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &prob); err != nil {
		t.Fatal(err)
	}
	if prob.Instance != "urn:labntp:request:"+id {
		t.Fatalf("instance %q", prob.Instance)
	}

	okReq := httptest.NewRequest(http.MethodGet, "/v1/version", nil)
	okReq.Header.Set("Authorization", "Bearer "+testToken)
	okReq.Header.Set("X-Request-ID", id)
	okW := httptest.NewRecorder()
	s.Handler().ServeHTTP(okW, okReq)
	if okW.Code != http.StatusOK {
		t.Fatalf("status %d body %s", okW.Code, okW.Body.Bytes())
	}
	if got := okW.Header().Get("X-Request-ID"); got != id {
		t.Fatalf("success header %q, want %q", got, id)
	}
}
