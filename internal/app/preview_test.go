package app

import (
	"context"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/model"
)

func TestPreviewFollowReal(t *testing.T) {
	svc, _ := mustBoot(t)
	p, err := svc.Preview(context.Background(), actor(), "10.99.42.20")
	if err != nil {
		t.Fatal(err)
	}
	if p.Filter != "default" || p.ServedTime == nil || p.Mode != model.ModeFollowReal {
		t.Fatalf("%+v", p)
	}
	if p.Reason != "" {
		t.Fatal(p.Reason)
	}
}

func TestPreviewMissingAndBadIP(t *testing.T) {
	svc, _ := mustBoot(t)
	_, err := svc.Preview(context.Background(), actor(), "")
	requireCode(t, err, domainerr.CodeValidationFailed)
	_, err = svc.Preview(context.Background(), actor(), "not-an-ip")
	requireCode(t, err, domainerr.CodeValidationFailed)
}

func TestPreviewAllowlistDenyAll(t *testing.T) {
	svc, snap := mustBoot(t)
	_, err := svc.Apply(context.Background(), actor(), ChangeIn{
		ExpectedRevision: snap.Revision,
		Operations: []model.Operation{{
			Op:               model.OpReplaceAllowClientCidrs,
			AllowClientCidrs: []string{},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := svc.Preview(context.Background(), actor(), "10.99.42.20")
	if err != nil {
		t.Fatal(err)
	}
	if p.Reason != "allowlist" || p.ServedTime != nil {
		t.Fatalf("%+v", p)
	}
}

func TestFilterCRUD(t *testing.T) {
	svc, snap := mustBoot(t)
	ctx := context.Background()
	a := actor()
	rate := 0.0
	def := snap.Canonical.Spec.Filters[0]
	res, err := svc.Apply(ctx, a, ChangeIn{
		ExpectedRevision: snap.Revision,
		Operations: []model.Operation{{
			Op: model.OpReplaceFilters,
			Filters: []model.Filter{
				{
					Name:    "tester",
					Enabled: true,
					Match:   model.MatchSpec{CIDRs: []string{"10.99.42.20/32"}},
					View:    model.ViewSpec{Mode: model.ModeRate, Rate: &rate, Leap: model.LeapNone, Stratum: 2},
				},
				def,
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetFilter(ctx, a, "tester")
	if err != nil || got.View.Mode != model.ModeRate {
		t.Fatalf("%+v %v", got, err)
	}
	p, err := svc.Preview(ctx, a, "10.99.42.20")
	if err != nil || p.Filter != "tester" {
		t.Fatalf("%+v %v", p, err)
	}
	_, err = svc.DeleteFilter(ctx, a, "tester", DeleteIn{ExpectedRevision: res.RuntimeRevision})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.GetFilter(ctx, a, "tester")
	requireCode(t, err, domainerr.CodeNotFound)
}
