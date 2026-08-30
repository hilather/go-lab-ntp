package app

import (
	"context"

	"github.com/hilather/go-lab-ntp/internal/config"
	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/observability"
)

// Export returns canonical YAML or JSON of the active snapshot.
func (s *App) Export(ctx context.Context, actor Actor, format ExportFormat) (*Export, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	snap, err := s.active()
	if err != nil {
		return nil, err
	}
	if format == "" {
		format = ExportYAML
	}
	if format != ExportYAML && format != ExportJSON {
		return nil, domainerr.ValidationFailed("unknown export format",
			domainerr.FieldViolation{Path: "format", Code: "invalid_value", Message: "format must be yaml or json"})
	}
	var body []byte
	switch format {
	case ExportJSON:
		body, err = config.CanonicalJSON(snap.Canonical)
	default:
		body, err = config.CanonicalYAML(snap.Canonical)
	}
	if err != nil {
		return nil, asDomain(err)
	}
	bootCanon := snap.Canonical
	if b := s.snaps.Bootstrap(); b != nil {
		bootCanon = b.Canonical
	}
	_, human, err := diffStates(bootCanon, snap.Canonical)
	if err != nil {
		return nil, err
	}
	return &Export{
		Format:            format,
		Body:              append([]byte(nil), body...),
		Revision:          snap.Revision,
		BootstrapRevision: snap.BootstrapRevision,
		Drifted:           snap.Drifted(),
		HumanDiff:         human,
	}, nil
}

// GetState returns a copy of the live spec plus revision metadata.
func (s *App) GetState(ctx context.Context, actor Actor) (*StateView, error) {
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
	return &StateView{
		BootstrapRevision: snap.BootstrapRevision,
		RuntimeRevision:   snap.Revision,
		Generation:        snap.Generation,
		Drifted:           snap.Drifted(),
		LoadedAt:          snap.CompiledAt,
		Canonical:         copied,
	}, nil
}

// Status is revisions plus listeners and hostTime.
func (s *App) Status(ctx context.Context, actor Actor) (*Status, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	snap, err := s.active()
	if err != nil {
		return nil, err
	}
	probe := observability.Evaluate(s.HealthFacts())
	warns := make([]Warning, 0, len(probe.Warnings))
	for _, w := range probe.Warnings {
		warns = append(warns, Warning{Code: w.Code, Message: w.Message})
	}
	ntpAddr := effectiveNTP(s.ntpOverride, snap.NTPAddress)
	mgmtAddr := effectiveMgmt(s.mgmtOverride, snap.ManagementAddress)
	if mgmtAddr == "" {
		mgmtAddr = "off"
	}
	return &Status{
		Ready: probe.Ready,
		Revisions: RevisionView{
			BootstrapRevision: snap.BootstrapRevision,
			RuntimeRevision:   snap.Revision,
			Generation:        snap.Generation,
			Drifted:           snap.Drifted(),
			LoadedAt:          snap.CompiledAt,
		},
		Listeners: []ListenerStatus{
			{Name: "ntp", Address: ntpAddr},
			{Name: "management", Address: mgmtAddr},
		},
		HostTime: s.clock.Now().UTC(),
		Warnings: warns,
	}, nil
}
