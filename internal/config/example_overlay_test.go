package config

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/model"
)

// TestLabOverlayExample loads examples/labntp.yaml (the mcp-integration-lab
// bootstrap BOM) and checks knobs the lab PR must not regress.
func TestLabOverlayExample(t *testing.T) {
	path := filepath.Join(repoRoot(t), "examples", "labntp.yaml")
	st, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Spec.Management.MCP.AllowLegacyClients {
		t.Fatal("lab overlay must set allowLegacyClients: true (D11)")
	}
	if st.Spec.Auth.Mode != model.MgmtAuthBearer {
		t.Fatalf("auth.mode=%q want bearer", st.Spec.Auth.Mode)
	}
	if st.Spec.Listeners.NTP.Address != ":123" || st.Spec.Listeners.Management.Address != ":8088" {
		t.Fatalf("listeners ntp=%q mgmt=%q", st.Spec.Listeners.NTP.Address, st.Spec.Listeners.Management.Address)
	}
	if !st.Spec.UI.Enabled {
		t.Fatal("ui.enabled must stay true (SPA lands in PR 13)")
	}
	if st.Spec.NTP.NTS.Enabled {
		t.Fatal("nts.enabled must stay false in v1")
	}
	wantCIDRs := []string{"10.99.42.0/24", "127.0.0.0/8", "::1/128"}
	if !slices.Equal(st.Spec.NTP.AllowClientCidrs, wantCIDRs) {
		t.Fatalf("allowClientCidrs=%v want %v", st.Spec.NTP.AllowClientCidrs, wantCIDRs)
	}
	found := false
	for _, tok := range st.Spec.Auth.Tokens {
		if tok.ID == "admin" && tok.SecretFile == "/run/secrets/labntp-token" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("missing admin token at /run/secrets/labntp-token")
	}
	if len(st.Spec.Filters) == 0 || st.Spec.Filters[len(st.Spec.Filters)-1].Name != "default" {
		t.Fatal("last filter must be default catch-all")
	}
	last := st.Spec.Filters[len(st.Spec.Filters)-1]
	if last.View.Mode != model.ModeFollowReal {
		t.Fatalf("default view mode=%q", last.View.Mode)
	}
}
