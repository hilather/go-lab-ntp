package mcp

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCharacterizeMCPGet405(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.RemoteAddr = "192.0.2.1:1234"
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	res := w.Result()
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status %d body %s", res.StatusCode, b)
	}
	if allow := res.Header.Get("Allow"); allow != http.MethodPost {
		t.Fatalf("Allow %q", allow)
	}
	var doc struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Error.Code != -32601 || doc.Error.Message != "method not allowed" {
		t.Fatalf("rpc %+v body %s", doc.Error, b)
	}
}
