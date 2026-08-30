package app

import (
	"context"
	"encoding/json"

	"github.com/hilather/go-lab-ntp/internal/audit"
	"github.com/hilather/go-lab-ntp/internal/compiler"
	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/model"
	"github.com/hilather/go-lab-ntp/internal/ntpview"
	"github.com/hilather/go-lab-ntp/internal/observability"
	"github.com/hilather/go-lab-ntp/internal/snapshot"
	"github.com/hilather/go-lab-ntp/internal/testutil"
)

type candidate struct {
	prev *snapshot.Snapshot
	next *snapshot.Snapshot
	ops  []model.Operation
	diff []DiffEntry
	warn []Warning
}

// Plan dry-runs the mutation pipeline. expectedRevision is required.
func (s *App) Plan(ctx context.Context, actor Actor, in ChangeIn) (*Plan, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.planLocked(in)
}

func (s *App) planLocked(in ChangeIn) (*Plan, error) {
	fp, err := fingerprintChange(in)
	if err != nil {
		return nil, err
	}
	if hit, err := s.idemp.lookup(in.IdempotencyKey, fp); err != nil {
		return nil, err
	} else if hit != nil && hit.plan != nil {
		return clonePlan(hit.plan), nil
	}
	cand, err := s.buildCandidate(in, true)
	if err != nil {
		s.forgetIdempOnConflict(in.IdempotencyKey, err)
		return nil, err
	}
	p := s.planFrom(cand)
	s.idemp.storePlan(in.IdempotencyKey, fp, p)
	return clonePlan(p), nil
}

// Apply compiles the candidate and atomically swaps only after success.
func (s *App) Apply(ctx context.Context, actor Actor, in ChangeIn) (*ApplyResult, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	s.mu.Lock()
	res, hooks, err := s.applyLocked(ctx, actor, in)
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	for _, fn := range hooks {
		fn()
	}
	return res, nil
}

func (s *App) applyLocked(ctx context.Context, actor Actor, in ChangeIn) (*ApplyResult, []func(), error) {
	fp, err := fingerprintChange(in)
	if err != nil {
		return nil, nil, err
	}
	if hit, err := s.idemp.lookup(in.IdempotencyKey, fp); err != nil {
		return nil, nil, err
	} else if hit != nil && hit.apply != nil {
		return cloneApply(hit.apply), nil, nil
	}
	cand, err := s.buildCandidate(in, true)
	if err != nil {
		s.forgetIdempOnConflict(in.IdempotencyKey, err)
		s.observeApply("error")
		return nil, nil, err
	}
	if cand.next != nil && cand.next.QueryLogSize > 0 && s.queryLog != nil {
		s.queryLog.Resize(cand.next.QueryLogSize)
	}
	prev := s.snaps.Swap(cand.next)
	res := &ApplyResult{
		Plan:            *s.planFrom(cand),
		Applied:         true,
		Generation:      cand.next.Generation,
		RuntimeRevision: cand.next.Revision,
	}
	if s.logger != nil {
		s.logger.Log(observability.Record{
			Event:     observability.EventStateApply,
			Component: "app",
			Result:    "ok",
		})
	}
	s.observeApply("ok")
	res.AuditEventID = s.recordAudit(ctx, audit.Event{
		Time:       s.now(),
		ActorID:    actor.ID,
		ActorClass: actor.Class,
		Transport:  actor.Transport,
		Capability: "changes.apply",
		Reason:     in.Reason,
		Revision:   cand.next.Revision,
		Previous:   revisionOf(prev),
		Result:     audit.ResultOK,
		Diff:       toAuditDiff(cand.diff),
	})
	s.idemp.storeApply(in.IdempotencyKey, fp, res)
	return cloneApply(res), append([]func(){}, s.applyHooks...), nil
}

// Validate inspects a candidate document and/or operations. It never swaps
// and does not require expectedRevision.
func (s *App) Validate(ctx context.Context, actor Actor, in ValidateIn) (*Plan, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	s.mu.Lock()
	defer s.mu.Unlock()
	var prev *snapshot.Snapshot
	var base *model.State
	if in.State != nil {
		copied, err := cloneState(in.State)
		if err != nil {
			return nil, err
		}
		base = copied
		prev = s.snaps.Load()
	} else {
		snap, err := s.active()
		if err != nil {
			return nil, err
		}
		prev = snap
		copied, err := cloneState(snap.Canonical)
		if err != nil {
			return nil, err
		}
		base = copied
	}
	if err := applyOperations(base, in.Operations); err != nil {
		return nil, err
	}
	next, err := compileCandidate(base, prev, s.clock)
	if err != nil {
		return nil, asDomain(err)
	}
	beforeState := in.State
	if prev != nil {
		beforeState = prev.Canonical
	}
	diff, _, err := diffStates(beforeState, next.Canonical)
	if err != nil {
		return nil, err
	}
	return clonePlan(s.planFrom(&candidate{
		prev: prev,
		next: next,
		ops:  append([]model.Operation(nil), in.Operations...),
		diff: diff,
		warn: warningsOf(next),
	})), nil
}

func (s *App) buildCandidate(in ChangeIn, requireRev bool) (*candidate, error) {
	prev, err := s.active()
	if err != nil {
		return nil, err
	}
	if requireRev {
		if in.ExpectedRevision == "" {
			return nil, domainerr.ValidationFailed("expectedRevision is required",
				domainerr.FieldViolation{Path: "expectedRevision", Code: "required", Message: "expectedRevision is required for plan and apply"})
		}
		if in.ExpectedRevision != prev.Revision {
			return nil, domainerr.RevisionConflict("active revision does not match expectedRevision", string(prev.Revision)).
				WithRemediation("Re-read GET state and re-plan against the current revision.")
		}
	}
	copied, err := cloneState(prev.Canonical)
	if err != nil {
		return nil, err
	}
	if err := applyOperations(copied, in.Operations); err != nil {
		return nil, err
	}
	if err := rejectResetOnly(prev.Canonical, copied); err != nil {
		return nil, err
	}
	next, err := compileCandidate(copied, prev, s.clock)
	if err != nil {
		return nil, asDomain(err)
	}
	diff, _, err := diffStates(prev.Canonical, next.Canonical)
	if err != nil {
		return nil, err
	}
	return &candidate{
		prev: prev,
		next: next,
		ops:  append([]model.Operation(nil), in.Operations...),
		diff: diff,
		warn: warningsOf(next),
	}, nil
}

func compileCandidate(st *model.State, prev *snapshot.Snapshot, clk ntpview.Clock) (*snapshot.Snapshot, error) {
	gen := model.Generation(1)
	boot := model.Revision("")
	if prev != nil {
		gen = prev.Generation + 1
		boot = prev.BootstrapRevision
	}
	var tclk testutil.Clock
	if clk != nil {
		tclk = clk
	}
	return compiler.Compile(st, compiler.CompileOpts{
		Clock:             tclk,
		Generation:        gen,
		BootstrapRevision: boot,
		Previous:          prev,
	})
}

func (s *App) planFrom(c *candidate) *Plan {
	prevRev := model.Revision("")
	if c.prev != nil {
		prevRev = c.prev.Revision
	}
	p := &Plan{
		PreviousRevision:  prevRev,
		CandidateRevision: c.next.Revision,
		Drifted:           c.next.Drifted(),
		Diff:              c.diff,
		Warnings:          c.warn,
		Operations:        append([]model.Operation(nil), c.ops...),
	}
	return p
}

func (s *App) forgetIdempOnConflict(key string, err error) {
	de, ok := domainerr.As(err)
	if !ok || de.Code != domainerr.CodeRevisionConflict {
		return
	}
	s.idemp.evict(key)
}

func (s *App) observeApply(result string) {
	if s == nil || s.metrics == nil {
		return
	}
	s.metrics.Inc(observability.MetricApplyTotal, map[string]string{"result": observability.ApplyResult(result)}, 1)
}

func revisionOf(s *snapshot.Snapshot) model.Revision {
	if s == nil {
		return ""
	}
	return s.Revision
}

func toAuditDiff(in []DiffEntry) []audit.RedactedEntry {
	if len(in) == 0 {
		return nil
	}
	out := make([]audit.RedactedEntry, len(in))
	for i, d := range in {
		out[i] = audit.RedactedEntry{Path: d.Path, Op: d.Op, Before: d.Before, After: d.After}
	}
	return out
}

func warningsOf(snap *snapshot.Snapshot) []Warning {
	if snap == nil {
		return nil
	}
	var out []Warning
	for _, w := range snap.Warnings {
		out = append(out, Warning{Code: w.Code, Message: w.Message})
	}
	return out
}

func cloneState(st *model.State) (*model.State, error) {
	if st == nil {
		return nil, domainerr.Internal("nil state")
	}
	b, err := json.Marshal(st)
	if err != nil {
		return nil, domainerr.Internal("clone: " + err.Error())
	}
	var out model.State
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, domainerr.Internal("clone: " + err.Error())
	}
	return &out, nil
}
