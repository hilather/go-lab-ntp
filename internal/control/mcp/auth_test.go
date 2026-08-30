package mcp

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBearerRequired(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", w.Code)
	}
}

func TestNoBasic(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", w.Code)
	}
}
