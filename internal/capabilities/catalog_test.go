package capabilities

import (
	"strings"
	"testing"
)

func TestCatalogRowCountAndNTPPrefix(t *testing.T) {
	if err := ValidateCatalog(); err != nil {
		t.Fatal(err)
	}
	if len(All()) != TableRowCount {
		t.Fatalf("rows %d", len(All()))
	}
	for _, name := range Tools() {
		if !strings.HasPrefix(name, "ntp_") {
			t.Errorf("tool %s must start with ntp_", name)
		}
		if strings.HasPrefix(name, "labntp_") {
			t.Errorf("tool %s uses rejected labntp_ prefix", name)
		}
	}
	for _, r := range Resources() {
		if !strings.HasPrefix(r, "labntp://") {
			t.Errorf("resource %s must use labntp://", r)
		}
	}
}

func TestFeaturesFrozen(t *testing.T) {
	ids := FeatureIDs()
	want := []string{
		"filters", "views", "restrict", "admission", "allowClientCidrs", "queryLog", "management.http",
		"listeners.ntp.address", "listeners.management.address", "ntp.nts", "ntp.symmetricKeys", "auth",
	}
	if len(ids) != len(want) {
		t.Fatalf("%v", ids)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("id[%d]=%s want %s", i, ids[i], want[i])
		}
	}
	for _, f := range Features() {
		if f.Apply != FeatureApplyLive && f.Apply != FeatureApplyResetOnly {
			t.Errorf("%s apply=%s", f.ID, f.Apply)
		}
	}
}
