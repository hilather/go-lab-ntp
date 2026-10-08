package rest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/capabilities"
	"github.com/hilather/go-lab-ntp/internal/domainerr"
)

func TestRejectUnknownMutationFields(t *testing.T) {
	s, svc := newTestServer(t)
	rev := string(svc.Active().Revision)
	body := fmt.Sprintf(`{"expectedRevision":%q,"operations":[],"notAField":true}`, rev)
	resp := doJSON(t, s, http.MethodPost, "/v1/changes:plan", body)
	raw, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("unknown field accepted, status %d body %s", resp.StatusCode, raw)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d body %s", resp.StatusCode, raw)
	}
	var p capabilities.Problem
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("problem: %v body %s", err, raw)
	}
	if p.Code != domainerr.CodeValidationFailed {
		t.Fatalf("code %s body %s", p.Code, raw)
	}
	found := false
	for _, v := range p.FieldViolations {
		if v.Code == "unknown_field" && v.Path == "notAField" {
			found = true
		}
	}
	if !found {
		t.Fatalf("violations %+v", p.FieldViolations)
	}
}
