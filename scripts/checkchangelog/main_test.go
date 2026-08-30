package main

import "testing"

func TestCheckChangelogRequiresEntry(t *testing.T) {
	err := checkChangelog([]string{"internal/ntpserver/server.go"})
	if err == nil {
		t.Fatal("expected missing changelog")
	}
	err = checkChangelog([]string{"internal/ntpserver/server.go", "CHANGELOG.md"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCheckChangelogIgnoresTestsAndUnobservable(t *testing.T) {
	if err := checkChangelog([]string{"internal/ntpview/view_test.go", "docs/01-architecture.md"}); err != nil {
		t.Fatal(err)
	}
}

func TestObservableRel(t *testing.T) {
	if !observableRel("docs/02-ntp-semantics.md") {
		t.Fatal("ntp-semantics should be observable")
	}
	if !observableRel("cmd/labntp/main.go") {
		t.Fatal("cmd should be observable")
	}
	if observableRel("internal/ntpview/view_test.go") {
		t.Fatal("tests are not observable")
	}
	if observableRel("CHANGELOG.md") {
		t.Fatal("changelog itself is not an observable trigger")
	}
}
