package app

import (
	"context"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/model"
)

func TestCharacterizeIdempotency(t *testing.T) {
	if defaultIdempotencyMax != 256 {
		t.Fatalf("capacity %d", defaultIdempotencyMax)
	}
	empty := newIdempCache(4)
	hit, err := empty.lookup("", "fp")
	if err != nil || hit != nil {
		t.Fatalf("empty key lookup = %v %v", hit, err)
	}

	svc, snap := mustBoot(t)
	ops := []model.Operation{{
		Op:       model.OpReplaceRestrict,
		Restrict: &model.RestrictSpec{Default: model.RestrictLimited, KoD: true},
	}}
	in := ChangeIn{
		ExpectedRevision: snap.Revision,
		IdempotencyKey:   "plan-apply-plan",
		Reason:           "pin",
		Operations:       ops,
	}
	a := actor()
	ctx := context.Background()
	p1, err := svc.Plan(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	applied, err := svc.Apply(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := svc.Plan(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	if p2.PreviousRevision != p1.PreviousRevision || p2.CandidateRevision != p1.CandidateRevision {
		t.Fatalf("second plan rebuilt: first prev %s second prev %s", p1.PreviousRevision, p2.PreviousRevision)
	}
	fresh := in
	fresh.IdempotencyKey = "fresh-after-apply"
	fresh.ExpectedRevision = applied.RuntimeRevision
	p3, err := svc.Plan(ctx, a, fresh)
	if err != nil {
		t.Fatal(err)
	}
	if p3.PreviousRevision == p2.PreviousRevision {
		t.Fatal("a new key must plan against the post-apply snapshot")
	}
	if p3.PreviousRevision != applied.RuntimeRevision {
		t.Fatalf("fresh previous %s want %s", p3.PreviousRevision, applied.RuntimeRevision)
	}

	replay := ChangeIn{
		ExpectedRevision: svc.Active().Revision,
		IdempotencyKey:   "replay",
		Reason:           "same",
		Operations:       ops,
	}
	r1, err := svc.Apply(ctx, a, replay)
	if err != nil {
		t.Fatal(err)
	}
	replay.ExpectedRevision = applied.RuntimeRevision
	r2, err := svc.Apply(ctx, a, replay)
	if err != nil {
		t.Fatal(err)
	}
	if r1.RuntimeRevision != r2.RuntimeRevision || r1.AuditEventID != r2.AuditEventID {
		t.Fatalf("expectedRevision-only change must replay: %+v %+v", r1, r2)
	}

	replay.Reason = "other"
	_, err = svc.Apply(ctx, a, replay)
	assertIdemConflict(t, err)
	replay.Reason = "same"
	replay.Operations = []model.Operation{{
		Op:       model.OpReplaceRestrict,
		Restrict: &model.RestrictSpec{Default: model.RestrictIgnore, KoD: false},
	}}
	_, err = svc.Apply(ctx, a, replay)
	assertIdemConflict(t, err)

	// newIdempCache(0) substitutes 256. The 257th key evicts the least recently used.
	cache := newIdempCache(0)
	for i := 0; i < 256; i++ {
		cache.storePlan(keyN(i), "fp", &Plan{PreviousRevision: "sha256:aa"})
	}
	if hit, err := cache.lookup(keyN(0), "fp"); err != nil || hit == nil {
		t.Fatalf("256 keys must fit: %v %v", hit, err)
	}
	cache.storePlan("k-new", "fp", &Plan{PreviousRevision: "sha256:bb"})
	if hit, err := cache.lookup(keyN(0), "fp"); err != nil || hit == nil {
		t.Fatal("touched key must survive")
	}
	if hit, err := cache.lookup(keyN(1), "fp"); err != nil || hit != nil {
		t.Fatal("least recently used key must be evicted")
	}
	if hit, err := cache.lookup("k-new", "fp"); err != nil || hit == nil {
		t.Fatal("inserted key missing")
	}
}

func assertIdemConflict(t *testing.T, err error) {
	t.Helper()
	de, ok := domainerr.As(err)
	if !ok || de.Code != domainerr.CodeIdempotencyConflict || de.Message != "idempotency key reused with a different request" {
		t.Fatalf("err=%v", err)
	}
}

func keyN(i int) string {
	return "k" + string(rune('a'+i/26)) + string(rune('a'+i%26))
}
