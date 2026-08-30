package domainerr

import (
	"errors"
	"testing"
)

func TestCodes(t *testing.T) {
	got := Codes()
	if len(got) == 0 {
		t.Fatal("empty catalog")
	}
	if got[0] != CodeValidationFailed {
		t.Fatalf("first = %q", got[0])
	}
	if !Retryable(CodeRevisionConflict) {
		t.Fatal("revision_conflict should be retryable")
	}
	if Retryable(CodeValidationFailed) {
		t.Fatal("validation_failed should not be retryable")
	}
	if Retryable(Code("nope")) {
		t.Fatal("unknown codes are not retryable")
	}
}

func TestConstructors(t *testing.T) {
	e := ValidationFailed("msg", FieldViolation{Path: "x", Code: "required", Message: "need x"})
	if e.Code != CodeValidationFailed || e.Error() == "" {
		t.Fatalf("%#v", e)
	}
	if !errors.Is(e, ValidationFailed("other")) {
		t.Fatal("Is by code")
	}
	got, ok := As(e)
	if !ok || got.Code != CodeValidationFailed {
		t.Fatal("As")
	}
	c := e.WithRemediation("hint").WithRevision("sha256:abc")
	if c.Remediation != "hint" || c.CurrentRevision != "sha256:abc" {
		t.Fatalf("%#v", c)
	}
	if New(CodeNotFound, "").Error() != string(CodeNotFound) {
		t.Fatal("empty message")
	}
}
