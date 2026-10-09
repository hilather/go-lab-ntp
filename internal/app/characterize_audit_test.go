package app

import (
	"context"
	"testing"

	"github.com/hilather/go-lab-ntp/internal/audit"
)

func TestCharacterizeAuditRecord(t *testing.T) {
	svc, _ := mustBoot(t)
	ctx := context.Background()
	id := svc.recordAudit(ctx, audit.Event{Capability: "changes.apply", ActorID: "admin", ActorClass: "token"})
	ev, err := svc.GetAudit(ctx, actor(), id)
	if err != nil {
		t.Fatal(err)
	}
	if ev.ID != "aud-1" || ev.Result != audit.ResultOK {
		t.Fatalf("empty result must become ok: %+v", ev)
	}
	id2 := svc.recordAudit(ctx, audit.Event{Capability: "changes.apply", Result: audit.ResultDenied})
	ev2, err := svc.GetAudit(ctx, actor(), id2)
	if err != nil {
		t.Fatal(err)
	}
	if ev2.ID != "aud-2" || ev2.Result != audit.ResultDenied {
		t.Fatalf("explicit result: %+v", ev2)
	}
	list, err := svc.QueryAudit(ctx, actor(), AuditQuery{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Events) < 2 || list.Events[0].ID != "aud-2" || list.Events[1].ID != "aud-1" {
		t.Fatalf("newest-first %+v", list.Events)
	}
}
