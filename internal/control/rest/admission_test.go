package rest

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/auth"
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
