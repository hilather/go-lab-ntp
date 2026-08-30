package model

import "testing"

func TestAPIConstants(t *testing.T) {
	if APIVersionV1Alpha1 != "labntp.dev/v1alpha1" {
		t.Fatalf("apiVersion = %q", APIVersionV1Alpha1)
	}
	if KindLabNTP != "LabNTP" {
		t.Fatalf("kind = %q", KindLabNTP)
	}
	if RevisionPrefix != "sha256:" {
		t.Fatalf("prefix = %q", RevisionPrefix)
	}
	if !KnownOp(OpReplaceFilters) || KnownOp("jsonPatch") {
		t.Fatal("KnownOp")
	}
}

func TestViewJSONNames(t *testing.T) {
	// Presence types must stay pointers so omitted vs 0 is distinguishable.
	var v ViewSpec
	if v.Rate != nil || v.MinPoll != nil || v.MaxPoll != nil {
		t.Fatal("zero ViewSpec must have nil presence pointers")
	}
}
