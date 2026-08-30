package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/model"
)

var expectedInvalid = map[string]string{
	"unknown-field.yaml":              violationUnknownField,
	"unknown-kebab.yaml":              violationUnknownField,
	"unknown-camel-minPoll.yaml":      violationUnknownField,
	"unknown-camel-refID.yaml":        violationUnknownField,
	"unknown-originAllowlist.yaml":    violationUnknownField,
	"unknown-nested-view.yaml":        violationUnknownField,
	"missing-catchall.yaml":           violationRequired,
	"missing-catchall-v6.yaml":        violationRequired,
	"rate-missing-key.yaml":           violationRequired,
	"rate-inf.yaml":                   violationInvalidValue,
	"rate-nan.yaml":                   violationInvalidValue,
	"rate-101.yaml":                   violationInvalidValue,
	"minpoll-gt-maxpoll.yaml":         violationInvalidValue,
	"nts-enabled.yaml":                violationInvalidValue,
	"inline-key.yaml":                 violationUnknownField,
	"forbidden-offset-on-follow.yaml": violationInvalidValue,
	"bare-offset.yaml":                violationInvalidValue,
	"reserved-chrony.yaml":            violationReservedName,
	"multi-doc.yaml":                  violationInvalidValue,
	"freeze-missing.yaml":             violationRequired,
	"duplicate-filter.yaml":           violationDuplicateID,
}

func TestConfigCompat(t *testing.T) {
	t.Chdir(repoRoot(t))
	validDir := testdata(t, "valid")
	ents, err := os.ReadDir(validDir)
	if err != nil {
		t.Fatal(err)
	}
	var validCount int
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		validCount++
		name := e.Name()
		t.Run("valid/"+name, func(t *testing.T) {
			st, warns, err := LoadFileWithWarnings(filepath.Join(validDir, name))
			if err != nil {
				t.Fatal(err)
			}
			if st.APIVersion != model.APIVersionV1Alpha1 || st.Kind != model.KindLabNTP {
				t.Fatalf("api=%q kind=%q", st.APIVersion, st.Kind)
			}
			rev, err := Revision(st)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := CanonicalJSON(st)
			if err != nil {
				t.Fatal(err)
			}
			again, err := Load(raw)
			if err != nil {
				t.Fatal(err)
			}
			rev2, err := Revision(again)
			if err != nil {
				t.Fatal(err)
			}
			if rev != rev2 {
				t.Fatalf("round-trip revision %s != %s", rev, rev2)
			}
			switch name {
			case "allow-omitted.yaml", "allow-null.yaml":
				if !hasWarning(warns, warningUniversalAllowlist) {
					t.Fatalf("want universal_allowlist warning, got %+v", warns)
				}
				if len(st.Spec.NTP.AllowClientCidrs) != 2 {
					t.Fatalf("materialized allow = %v", st.Spec.NTP.AllowClientCidrs)
				}
			case "allow-empty.yaml":
				if hasWarning(warns, warningUniversalAllowlist) {
					t.Fatalf("empty list must not warn: %+v", warns)
				}
				if st.Spec.NTP.AllowClientCidrs == nil || len(st.Spec.NTP.AllowClientCidrs) != 0 {
					t.Fatalf("empty list must stay deny-all, got %#v", st.Spec.NTP.AllowClientCidrs)
				}
			case "rate-zero.yaml":
				if st.Spec.Filters[0].View.Rate == nil || *st.Spec.Filters[0].View.Rate != 0 {
					t.Fatalf("rate:0 must be present pointer to 0, got %#v", st.Spec.Filters[0].View.Rate)
				}
			case "omitted-rate-offset.yaml":
				if st.Spec.Filters[0].View.Rate != nil {
					t.Fatalf("omitted rate must stay nil, got %#v", st.Spec.Filters[0].View.Rate)
				}
			}
		})
	}
	if validCount < 2 {
		t.Fatalf("expected at least defaults and full, got %d", validCount)
	}

	invalidDir := testdata(t, "invalid")
	ients, err := os.ReadDir(invalidDir)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, e := range ients {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		name := e.Name()
		want, ok := expectedInvalid[name]
		if !ok {
			t.Errorf("invalid/%s has no expectedInvalid entry", name)
			continue
		}
		seen[name] = true
		t.Run("invalid/"+name, func(t *testing.T) {
			_, err := LoadFile(filepath.Join(invalidDir, name))
			if err == nil {
				t.Fatal("expected error")
			}
			de, ok := domainerr.As(err)
			if !ok {
				t.Fatalf("error is %T %v", err, err)
			}
			if de.Code != domainerr.CodeValidationFailed {
				t.Fatalf("code=%s", de.Code)
			}
			found := false
			for _, v := range de.FieldViolations {
				if v.Code == want {
					found = true
					break
				}
			}
			if !found && want != "" {
				t.Fatalf("want violation %q in %+v (err=%v)", want, de.FieldViolations, err)
			}
		})
	}
	for name := range expectedInvalid {
		if !seen[name] {
			t.Errorf("expectedInvalid %s missing on disk", name)
		}
	}
}

func hasWarning(ws []Warning, code string) bool {
	for _, w := range ws {
		if w.Code == code {
			return true
		}
	}
	return false
}

func TestOmittedVsRateZero(t *testing.T) {
	st, err := LoadFile(testdata(t, "valid", "omitted-rate-offset.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Spec.Filters[0].View.Rate != nil {
		t.Fatal("omitted rate")
	}
	st0, err := LoadFile(testdata(t, "valid", "rate-zero.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if st0.Spec.Filters[0].View.Rate == nil || *st0.Spec.Filters[0].View.Rate != 0 {
		t.Fatal("rate:0")
	}
}

func TestMinPollExplicitZero(t *testing.T) {
	raw := []byte(`apiVersion: labntp.dev/v1alpha1
kind: LabNTP
metadata:
  name: x
spec:
  filters:
    - name: default
      match:
        cidrs: ["0.0.0.0/0", "::/0"]
      view:
        mode: follow-real
        minpoll: 0
        maxpoll: 6
`)
	st, err := Load(raw)
	if err != nil {
		t.Fatal(err)
	}
	if st.Spec.Filters[0].View.MinPoll == nil || *st.Spec.Filters[0].View.MinPoll != 0 {
		t.Fatalf("explicit minpoll 0 must not be omitted: %#v", st.Spec.Filters[0].View.MinPoll)
	}
}

func TestValidateNil(t *testing.T) {
	_ = requireValidation(t, Validate(nil), violationRequired)
}

func TestSchemaFilePresent(t *testing.T) {
	t.Chdir(repoRoot(t))
	b, err := SchemaBytes()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "labntp.dev/v1alpha1") {
		t.Fatal("schema missing api version")
	}
	if !strings.Contains(string(b), "minpoll") || strings.Contains(string(b), `"minPoll"`) {
		t.Fatal("schema must use minpoll, not minPoll")
	}
	if !strings.Contains(string(b), `"refid"`) {
		t.Fatal("schema missing refid")
	}
}
