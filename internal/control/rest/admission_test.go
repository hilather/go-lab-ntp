package rest

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/auth"
	"github.com/hilather/go-lab-ntp/internal/control/mcp"
	"github.com/hilather/go-lab-ntp/internal/model"
)

// replaceManagementHTTP updates the snapshot. The running limiter, body limit,
// and inflight gate must follow on apply.
func TestApplyManagementHTTPTightensLimiter(t *testing.T) {
	svc := bootTestApp(t)
	const startupMax = 64
	s, err := New(Config{
		Service:       svc,
		Auth:          auth.Static(testToken, "admin", model.RoleAdministrator),
		RatePerSec:    1000,
		RateBurst:     1000,
		MaxConcurrent: startupMax,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := s.inflightMax(); got != startupMax {
		t.Fatalf("startup inflight max %d, want %d", got, startupMax)
	}
	rev := string(svc.Active().Revision)
	body := fmt.Sprintf(`{"expectedRevision":%q,"operations":[{"op":"replaceManagementHTTP","managementHTTP":{"bodyLimit":1048576,"requestsPerSecond":1,"burst":1,"maxConcurrent":1}}]}`, rev)
	resp := doJSON(t, s, http.MethodPost, "/v1/changes:apply", body)
	raw, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("apply status %d body %s", resp.StatusCode, raw)
	}
	limited := 0
	for i := 0; i < 8; i++ {
		req := httptest.NewRequest(http.MethodGet, "/v1/version", nil)
		req.Header.Set("Authorization", "Bearer "+testToken)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, req)
		if w.Code == http.StatusTooManyRequests {
			limited++
		}
	}
	if limited == 0 {
		t.Fatal("applied requestsPerSecond=1 burst=1 but 8 follow-up requests were all admitted")
	}
	if got := s.inflightMax(); got != 1 {
		t.Fatalf("applied inflight max %d, want 1", got)
	}
}

// A lowered bodyLimit must bound the mounted /mcp body, not only REST JSON decode.
func TestApplyLowerBodyLimitRejectsOversizedMCPPost(t *testing.T) {
	svc := bootTestApp(t)
	const limit = int64(64)
	mount := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	s, err := New(Config{
		Service:    svc,
		Auth:       auth.Static(testToken, "admin", model.RoleAdministrator),
		RatePerSec: -1,
		Mounts:     map[string]http.Handler{"/mcp": mount},
	})
	if err != nil {
		t.Fatal(err)
	}
	applyManagementHTTP(t, s, string(svc.Active().Revision), limit, 32, 64, 256)
	if got := s.maxBody.Load(); got != limit {
		t.Fatalf("live body limit %d, want %d", got, limit)
	}

	over := doMounted(t, s, http.MethodPost, "/mcp", strings.Repeat("b", int(limit)+1))
	if over.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized /mcp POST status %d, want %d", over.StatusCode, http.StatusRequestEntityTooLarge)
	}
	under := doMounted(t, s, http.MethodPost, "/mcp", strings.Repeat("a", int(limit)-1))
	if under.StatusCode != http.StatusOK {
		t.Fatalf("under-limit /mcp POST status %d, want %d", under.StatusCode, http.StatusOK)
	}
}

// The mounted handler is mcp.Server.Handler. The SDK reads the live
// MaxBytesReader body and maps *http.MaxBytesError to 413. The stub above
// only checks that dispatchMount wrapped the body.
func TestApplyLowerBodyLimitRejectsOversizedRealMCPPost(t *testing.T) {
	svc := bootTestApp(t)
	const limit = int64(64)
	mcpSrv, err := mcp.New(mcp.Config{
		Service:            svc,
		RatePerSec:         -1,
		AllowLegacyClients: true,
		// Startup SDK ceiling stays at 1 MiB, above the lowered live limit,
		// so 413 comes from dispatchMount rather than the SDK cap.
		MaxBodyBytes: DefaultMaxBodyBytes,
		Auth:         auth.Static(testToken, "admin", model.RoleAdministrator),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mcpSrv.Close)
	s, err := New(Config{
		Service:    svc,
		Auth:       auth.Static(testToken, "admin", model.RoleAdministrator),
		RatePerSec: -1,
		Mounts:     map[string]http.Handler{"/mcp": mcpSrv.Handler()},
	})
	if err != nil {
		t.Fatal(err)
	}
	applyManagementHTTP(t, s, string(svc.Active().Revision), limit, 32, 64, 256)
	if got := s.maxBody.Load(); got != limit {
		t.Fatalf("live body limit %d, want %d", got, limit)
	}

	over := doMCPPost(t, s, strings.Repeat("b", int(limit)+1))
	if over.StatusCode != http.StatusRequestEntityTooLarge {
		raw, _ := io.ReadAll(over.Body)
		t.Fatalf("oversized /mcp POST status %d, want %d, body %s", over.StatusCode, http.StatusRequestEntityTooLarge, raw)
	}
	under := doMCPPost(t, s, strings.Repeat("a", int(limit)-1))
	if under.StatusCode == http.StatusRequestEntityTooLarge {
		raw, _ := io.ReadAll(under.Body)
		t.Fatalf("under-limit /mcp POST status %d, body %s", under.StatusCode, raw)
	}
}

// bodyLimit 0 is the startup default, matching New, not the previous live value.
func TestApplyZeroBodyLimitRestoresDefault(t *testing.T) {
	svc := bootTestApp(t)
	const custom = int64(4096)
	s, err := New(Config{
		Service:      svc,
		Auth:         auth.Static(testToken, "admin", model.RoleAdministrator),
		RatePerSec:   -1,
		MaxBodyBytes: custom,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := s.maxBody.Load(); got != custom {
		t.Fatalf("startup body limit %d, want %d", got, custom)
	}
	applyManagementHTTP(t, s, string(svc.Active().Revision), 0, 0, 0, 0)
	if got := s.maxBody.Load(); got != DefaultMaxBodyBytes {
		t.Fatalf("live body limit after bodyLimit 0 is %d, want default %d", got, DefaultMaxBodyBytes)
	}
}

func applyManagementHTTP(t *testing.T, s *Server, rev string, bodyLimit int64, rps, burst, maxConcurrent int) {
	t.Helper()
	body := fmt.Sprintf(`{"expectedRevision":%q,"operations":[{"op":"replaceManagementHTTP","managementHTTP":{"bodyLimit":%d,"requestsPerSecond":%d,"burst":%d,"maxConcurrent":%d}}]}`, rev, bodyLimit, rps, burst, maxConcurrent)
	resp := doJSON(t, s, http.MethodPost, "/v1/changes:apply", body)
	raw, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("apply status %d body %s", resp.StatusCode, raw)
	}
}

func doMounted(t *testing.T, s *Server, method, path, body string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	resp := w.Result()
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// doMCPPost sends the headers the real MCP handler and SDK require before
// they read the body: bearer auth, protocol version, JSON content type, and
// an Accept list that includes both JSON and event-stream.
func doMCPPost(t *testing.T, s *Server, body string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Mcp-Protocol-Version", mcp.ProtocolVersion)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	resp := w.Result()
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}
