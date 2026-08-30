package app

import (
	"context"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/capabilities"
	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/model"
)

func TestLiveVsResetOnly(t *testing.T) {
	svc, snap := mustBoot(t)
	ctx := context.Background()
	a := actor()

	feats, err := svc.Features(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	live := map[string]bool{}
	resetOnly := map[string]bool{}
	for _, f := range feats.Items {
		if f.Apply == capabilities.FeatureApplyLive {
			live[f.ID] = true
		}
		if f.Apply == capabilities.FeatureApplyResetOnly {
			resetOnly[f.ID] = true
		}
	}
	for _, id := range []string{"filters", "views", "restrict", "admission", "allowClientCidrs", "queryLog", "management.http"} {
		if !live[id] {
			t.Errorf("expected live %s", id)
		}
	}
	for _, id := range []string{"listeners.ntp.address", "listeners.management.address", "ntp.nts", "ntp.symmetricKeys", "auth"} {
		if !resetOnly[id] {
			t.Errorf("expected reset-only %s", id)
		}
	}

	// Live apply of filters succeeds.
	offset := int64(0)
	_ = offset
	res, err := svc.Apply(ctx, a, ChangeIn{
		ExpectedRevision: snap.Revision,
		Operations: []model.Operation{{
			Op:       model.OpReplaceRestrict,
			Restrict: &model.RestrictSpec{Default: model.RestrictServe, KoD: true},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Applied {
		t.Fatal("apply")
	}

	// Unknown op that would change listen is rejected as reset-only.
	_, err = svc.Apply(ctx, a, ChangeIn{
		ExpectedRevision: res.RuntimeRevision,
		Operations:       []model.Operation{{Op: "replaceListeners"}},
	})
	requireCode(t, err, domainerr.CodeValidationFailed)
	de, _ := domainerr.As(err)
	if de.Remediation == "" {
		t.Fatal("remediation")
	}
}

func TestApplyRequiresExpectedRevision(t *testing.T) {
	svc, _ := mustBoot(t)
	_, err := svc.Apply(context.Background(), actor(), ChangeIn{
		Operations: []model.Operation{{
			Op:       model.OpReplaceRestrict,
			Restrict: &model.RestrictSpec{Default: model.RestrictServe},
		}},
	})
	requireCode(t, err, domainerr.CodeValidationFailed)
}

func TestIdempotentApply(t *testing.T) {
	svc, snap := mustBoot(t)
	in := ChangeIn{
		ExpectedRevision: snap.Revision,
		IdempotencyKey:   "k1",
		Operations: []model.Operation{{
			Op:       model.OpReplaceRestrict,
			Restrict: &model.RestrictSpec{Default: model.RestrictLimited, KoD: true},
		}},
	}
	a := actor()
	r1, err := svc.Apply(context.Background(), a, in)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := svc.Apply(context.Background(), a, in)
	if err != nil {
		t.Fatal(err)
	}
	if r1.RuntimeRevision != r2.RuntimeRevision {
		t.Fatal("idempotent replay must return the same revision")
	}
	in.Reason = "other"
	_, err = svc.Apply(context.Background(), a, in)
	requireCode(t, err, domainerr.CodeIdempotencyConflict)
}
