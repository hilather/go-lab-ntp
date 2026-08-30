package app

import (
	"context"
	"os"

	"github.com/hilather/go-lab-ntp/internal/audit"
	"github.com/hilather/go-lab-ntp/internal/compiler"
	"github.com/hilather/go-lab-ntp/internal/config"
	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/model"
	"github.com/hilather/go-lab-ntp/internal/observability"
	"github.com/hilather/go-lab-ntp/internal/snapshot"
)

// Reset rereads the bootstrap mount, compiles, wipes the query log, rebinds
// NTP/HTTP per D8, and swaps only after success. It never writes the file.
func (s *App) Reset(ctx context.Context, actor Actor, in ResetIn) (*ApplyResult, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	s.mu.Lock()
	res, hooks, err := s.resetLocked(ctx, actor, in)
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	for _, fn := range hooks {
		fn()
	}
	return res, nil
}

func (s *App) resetLocked(ctx context.Context, actor Actor, in ResetIn) (*ApplyResult, []func(), error) {
	prev := s.snaps.Load()
	gen := model.Generation(0)
	if prev != nil {
		gen = prev.Generation + 1
	}

	next, err := s.loadBootstrapCandidate(gen)
	if err != nil {
		return nil, nil, err
	}

	diff, _, err := diffStates(canonicalOf(prev), next.Canonical)
	if err != nil {
		return nil, nil, err
	}

	oldNTP := ""
	oldMgmt := ""
	if prev != nil {
		oldNTP = effectiveNTP(s.ntpOverride, prev.NTPAddress)
		oldMgmt = effectiveMgmt(s.mgmtOverride, prev.ManagementAddress)
	}
	newNTP := effectiveNTP(s.ntpOverride, next.NTPAddress)
	newMgmt := effectiveMgmt(s.mgmtOverride, next.ManagementAddress)

	if s.ntpRebind != nil && newNTP != "" && newNTP != oldNTP {
		if err := s.ntpRebind(newNTP); err != nil {
			return nil, nil, asDomain(err)
		}
	}
	if s.httpRebind != nil && newMgmt != oldMgmt {
		if err := s.httpRebind(newMgmt); err != nil {
			return nil, nil, asDomain(err)
		}
	}

	if s.queryLog != nil {
		s.queryLog.Reset()
		if next.QueryLogSize > 0 {
			s.queryLog.Resize(next.QueryLogSize)
		}
	}

	displaced := s.snaps.Swap(next)
	s.snaps.SetBootstrap(next)
	s.idemp.clear()
	hooks := append([]func(){}, s.resetHooks...)

	res := &ApplyResult{
		Plan:            *s.planFrom(&candidate{prev: displaced, next: next, diff: diff, warn: warningsOf(next)}),
		Applied:         true,
		Generation:      next.Generation,
		RuntimeRevision: next.Revision,
	}
	if s.logger != nil {
		s.logger.Log(observability.Record{
			Event:     observability.EventStateReset,
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
		Capability: "state.reset",
		Reason:     in.Reason,
		Revision:   next.Revision,
		Previous:   revisionOf(displaced),
		Result:     audit.ResultOK,
		Diff:       toAuditDiff(diff),
	})
	return cloneApply(res), hooks, nil
}

func (s *App) loadBootstrapCandidate(gen model.Generation) (*snapshot.Snapshot, error) {
	if s.bootstrapPath != "" {
		if _, err := os.Stat(s.bootstrapPath); err != nil {
			if os.IsNotExist(err) {
				return nil, domainerr.ValidationFailed("bootstrap file unavailable",
					domainerr.FieldViolation{Path: "bootstrapPath", Code: "required", Message: "bootstrap file is missing; active snapshot unchanged"})
			}
			return nil, domainerr.Internal("stat bootstrap: " + err.Error())
		}
		st, err := config.LoadFile(s.bootstrapPath)
		if err != nil {
			return nil, asDomain(err)
		}
		snap, err := compiler.Compile(st, compiler.CompileOpts{
			Clock:      s.clock,
			Generation: gen,
		})
		if err != nil {
			return nil, asDomain(err)
		}
		return snap, nil
	}
	boot := s.snaps.Bootstrap()
	if boot == nil || boot.Canonical == nil {
		return nil, domainerr.ValidationFailed("no bootstrap snapshot",
			domainerr.FieldViolation{Path: "bootstrap", Code: "required", Message: "no bootstrap path or snapshot to reset to"})
	}
	copied, err := cloneState(boot.Canonical)
	if err != nil {
		return nil, err
	}
	snap, err := compiler.Compile(copied, compiler.CompileOpts{
		Clock:      s.clock,
		Generation: gen,
	})
	if err != nil {
		return nil, asDomain(err)
	}
	return snap, nil
}

func canonicalOf(s *snapshot.Snapshot) *model.State {
	if s == nil {
		return nil
	}
	return s.Canonical
}
