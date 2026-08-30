package app

import (
	"context"

	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/model"
)

func (s *App) ListFilters(ctx context.Context, actor Actor) (*FilterList, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	snap, err := s.active()
	if err != nil {
		return nil, err
	}
	copied, err := cloneState(snap.Canonical)
	if err != nil {
		return nil, err
	}
	return &FilterList{Items: append([]model.Filter(nil), copied.Spec.Filters...)}, nil
}

func (s *App) GetFilter(ctx context.Context, actor Actor, name string) (*model.Filter, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	if name == "" {
		return nil, domainerr.ValidationFailed("name is required",
			domainerr.FieldViolation{Path: "name", Code: "required", Message: "name is required"})
	}
	snap, err := s.active()
	if err != nil {
		return nil, err
	}
	for _, f := range snap.Canonical.Spec.Filters {
		if f.Name == name {
			copied, err := cloneState(snap.Canonical)
			if err != nil {
				return nil, err
			}
			for i := range copied.Spec.Filters {
				if copied.Spec.Filters[i].Name == name {
					out := copied.Spec.Filters[i]
					return &out, nil
				}
			}
		}
	}
	return nil, domainerr.NotFound("filter " + name + " not found")
}

func (s *App) PutFilter(ctx context.Context, actor Actor, in PutFilterIn) (*ApplyResult, error) {
	if in.Filter.Name == "" {
		return nil, domainerr.ValidationFailed("name is required",
			domainerr.FieldViolation{Path: "name", Code: "required", Message: "name is required"})
	}
	f := in.Filter
	return s.Apply(ctx, actor, ChangeIn{
		ExpectedRevision: in.ExpectedRevision,
		IdempotencyKey:   in.IdempotencyKey,
		Reason:           in.Reason,
		Operations: []model.Operation{{
			Op:     model.OpUpsertFilter,
			Filter: &f,
		}},
	})
}

func (s *App) DeleteFilter(ctx context.Context, actor Actor, name string, in DeleteIn) (*ApplyResult, error) {
	if name == "" {
		return nil, domainerr.ValidationFailed("name is required",
			domainerr.FieldViolation{Path: "name", Code: "required", Message: "name is required"})
	}
	return s.Apply(ctx, actor, ChangeIn{
		ExpectedRevision: in.ExpectedRevision,
		IdempotencyKey:   in.IdempotencyKey,
		Reason:           in.Reason,
		Operations: []model.Operation{{
			Op:   model.OpRemoveFilter,
			Name: name,
		}},
	})
}
