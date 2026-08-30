package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/hilather/go-lab-ntp/internal/app"
	"github.com/hilather/go-lab-ntp/internal/buildinfo"
	"github.com/hilather/go-lab-ntp/internal/capabilities"
	"github.com/hilather/go-lab-ntp/internal/config"
	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/model"
	"github.com/hilather/go-lab-ntp/internal/observability"
)

func (s *Server) dispatch(w http.ResponseWriter, r *http.Request, instance string, actor app.Actor, rt compiledRoute, params map[string]string) {
	ctx := r.Context()
	if err := ctx.Err(); err != nil {
		s.writeProblem(w, r, instance, domainerr.Internal("request canceled"))
		return
	}
	switch rt.cap.ID {
	case capabilities.HealthLive:
		s.handleHealthLive(w, r)
	case capabilities.HealthReady:
		s.handleHealthReady(w, r, ctx)
	case capabilities.VersionGet:
		s.writeJSON(w, http.StatusOK, fromVersion(buildinfo.Current()))
	case capabilities.CapabilitiesGet:
		s.writeJSON(w, http.StatusOK, fromCapabilities())
	case capabilities.StatusGet:
		s.handleStatus(w, r, instance, ctx, actor)
	case capabilities.SchemaGet:
		s.handleSchema(w, r, instance)
	case capabilities.FeaturesList:
		s.handleFeatures(w, r, instance, ctx, actor)
	case capabilities.StateGet:
		s.handleGetState(w, r, instance, ctx, actor)
	case capabilities.StateValidate:
		s.handleValidate(w, r, instance, ctx, actor)
	case capabilities.ChangesPlan:
		s.handlePlan(w, r, instance, ctx, actor)
	case capabilities.ChangesApply:
		s.handleApply(w, r, instance, ctx, actor)
	case capabilities.SessionCreate:
		s.handleSessionCreate(w, r, instance, actor)
	case capabilities.SessionDelete:
		s.handleSessionDelete(w, r, instance, actor)
	case capabilities.SessionGet:
		s.handleSessionGet(w, r, instance, actor)
	case capabilities.StateExport:
		s.handleExport(w, r, instance, ctx, actor)
	case capabilities.StateReset:
		s.handleReset(w, r, instance, ctx, actor)
	case capabilities.FiltersList:
		s.handleFiltersList(w, r, instance, ctx, actor)
	case capabilities.FiltersGet:
		s.handleFiltersGet(w, r, instance, ctx, actor, params["name"])
	case capabilities.FiltersPut:
		s.handleFiltersPut(w, r, instance, ctx, actor, params["name"])
	case capabilities.FiltersDelete:
		s.handleFiltersDelete(w, r, instance, ctx, actor, params["name"])
	case capabilities.ViewsPreview:
		s.handlePreview(w, r, instance, ctx, actor)
	case capabilities.QueriesList:
		s.handleQueries(w, r, instance, ctx, actor)
	case capabilities.AuditList:
		s.handleAuditList(w, r, instance, ctx, actor)
	case capabilities.AuditGet:
		s.handleAuditGet(w, r, instance, ctx, actor, params["eventId"])
	case capabilities.MetricsGet:
		s.handleMetrics(w, r, instance)
	default:
		s.writeProblem(w, r, instance, domainerr.NotFound("not found"))
	}
}

func (s *Server) handleHealthLive(w http.ResponseWriter, r *http.Request) {
	if !s.isLive() {
		s.writeJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "down"})
		return
	}
	s.writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
	_ = r
}

func (s *Server) handleHealthReady(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	if !s.isReady(ctx) {
		s.writeJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "not ready"})
		return
	}
	s.writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
	_ = r
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor app.Actor) {
	st, err := s.svc.Status(ctx, actor)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	rev, err := marshalAPI(st.Revisions)
	if err != nil {
		s.writeProblem(w, r, instance, domainerr.Internal("internal error"))
		return
	}
	listeners := make([]listenerJSON, 0, len(st.Listeners))
	for _, l := range st.Listeners {
		listeners = append(listeners, listenerJSON{Name: l.Name, Address: l.Address})
	}
	s.writeJSON(w, http.StatusOK, statusResponse{
		Ready:     s.isReady(ctx),
		Revisions: rev,
		Listeners: listeners,
		HostTime:  rfc3339(st.HostTime),
		Warnings:  st.Warnings,
	})
	_ = r
}

func (s *Server) handleSchema(w http.ResponseWriter, r *http.Request, instance string) {
	b, err := config.SchemaBytes()
	if err != nil {
		s.writeProblem(w, r, instance, domainerr.Internal("schema unavailable"))
		return
	}
	s.writeBytes(w, http.StatusOK, "application/schema+json", b)
	_ = r
}

func (s *Server) handleFeatures(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor app.Actor) {
	list, err := s.svc.Features(ctx, actor)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"items": list.Items})
	_ = r
}

func (s *Server) handleGetState(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor app.Actor) {
	v, err := s.svc.GetState(ctx, actor)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	canon, err := marshalAPI(v.Canonical)
	if err != nil {
		s.writeProblem(w, r, instance, domainerr.Internal("internal error"))
		return
	}
	if v.RuntimeRevision != "" {
		w.Header().Set(headerRevision, string(v.RuntimeRevision))
	}
	s.writeJSON(w, http.StatusOK, stateViewJSON{
		BootstrapRevision: string(v.BootstrapRevision),
		RuntimeRevision:   string(v.RuntimeRevision),
		Generation:        uint64(v.Generation),
		Drifted:           v.Drifted,
		LoadedAt:          rfc3339(v.LoadedAt),
		Canonical:         canon,
	})
	_ = r
}

func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor app.Actor) {
	var in changeRequest
	if !s.decodeJSON(w, r, instance, &in) {
		return
	}
	st, err := decodeCandidateState(in.State)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	plan, err := s.svc.Validate(ctx, actor, app.ValidateIn{State: st, Operations: in.Operations})
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, fromPlan(plan))
}

func (s *Server) handlePlan(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor app.Actor) {
	in, ok := s.readChange(w, r, instance)
	if !ok {
		return
	}
	plan, err := s.svc.Plan(ctx, actor, in)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, fromPlan(plan))
}

func (s *Server) handleApply(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor app.Actor) {
	in, ok := s.readChange(w, r, instance)
	if !ok {
		return
	}
	res, err := s.svc.Apply(ctx, actor, in)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	if res != nil && res.RuntimeRevision != "" {
		w.Header().Set(headerRevision, string(res.RuntimeRevision))
	}
	s.writeJSON(w, http.StatusOK, fromApply(res))
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor app.Actor) {
	format := app.ExportYAML
	switch strings.ToLower(r.URL.Query().Get("format")) {
	case "", "yaml", "yml":
	case "json":
		format = app.ExportJSON
	default:
		s.writeProblem(w, r, instance, domainerr.ValidationFailed("unknown export format",
			domainerr.FieldViolation{Path: "format", Code: "invalid_value", Message: "format must be yaml or json"}))
		return
	}
	exp, err := s.svc.Export(ctx, actor, format)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	if exp.Revision != "" {
		w.Header().Set(headerRevision, string(exp.Revision))
	}
	if format == app.ExportYAML {
		s.writeBytes(w, http.StatusOK, "application/yaml", exp.Body)
		return
	}
	s.writeJSON(w, http.StatusOK, exportJSON{
		Format:            string(exp.Format),
		Revision:          string(exp.Revision),
		BootstrapRevision: string(exp.BootstrapRevision),
		Drifted:           exp.Drifted,
		Body:              json.RawMessage(exp.Body),
		HumanDiff:         exp.HumanDiff,
	})
}

func (s *Server) handleReset(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor app.Actor) {
	var in resetRequest
	if !s.decodeJSONOptional(w, r, instance, &in) {
		return
	}
	res, err := s.svc.Reset(ctx, actor, app.ResetIn{Reason: in.Reason})
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, fromApply(res))
}

func (s *Server) handleFiltersList(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor app.Actor) {
	list, err := s.svc.ListFilters(ctx, actor)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"items": list.Items})
	_ = r
}

func (s *Server) handleFiltersGet(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor app.Actor, name string) {
	f, err := s.svc.GetFilter(ctx, actor, name)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, f)
}

func (s *Server) handleFiltersPut(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor app.Actor, name string) {
	var in putFilterRequest
	if !s.decodeJSON(w, r, instance, &in) {
		return
	}
	f := model.Filter{}
	if in.Filter != nil {
		f = *in.Filter
	}
	if f.Name == "" {
		f.Name = name
	}
	if name != "" && f.Name != name {
		s.writeProblem(w, r, instance, domainerr.ValidationFailed("name mismatch",
			domainerr.FieldViolation{Path: "name", Code: "invalid_value", Message: "path name must match filter.name"}))
		return
	}
	res, err := s.svc.PutFilter(ctx, actor, app.PutFilterIn{
		ExpectedRevision: model.Revision(expectedRevision(r, in.ExpectedRevision)),
		IdempotencyKey:   idempotencyKey(r, in.IdempotencyKey),
		Reason:           in.Reason,
		Filter:           f,
	})
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, fromApply(res))
}

func (s *Server) handleFiltersDelete(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor app.Actor, name string) {
	var in deleteFilterRequest
	if !s.decodeJSONOptional(w, r, instance, &in) {
		return
	}
	res, err := s.svc.DeleteFilter(ctx, actor, name, app.DeleteIn{
		ExpectedRevision: model.Revision(expectedRevision(r, in.ExpectedRevision)),
		IdempotencyKey:   idempotencyKey(r, in.IdempotencyKey),
		Reason:           in.Reason,
	})
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, fromApply(res))
}

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor app.Actor) {
	ip := strings.TrimSpace(r.URL.Query().Get("ip"))
	p, err := s.svc.Preview(ctx, actor, ip)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, fromPreview(p))
}

func (s *Server) handleQueries(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor app.Actor) {
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			s.writeProblem(w, r, instance, domainerr.ValidationFailed("invalid limit",
				domainerr.FieldViolation{Path: "limit", Code: "invalid_value", Message: "limit must be a non-negative integer"}))
			return
		}
		limit = n
	}
	list, err := s.svc.ListQueries(ctx, actor, app.Page{Limit: limit, Cursor: r.URL.Query().Get("cursor")})
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	items := make([]queryJSON, 0, len(list.Items))
	for _, e := range list.Items {
		items = append(items, fromQuery(e))
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"items": items, "nextCursor": list.NextCursor})
}

func (s *Server) handleAuditList(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor app.Actor) {
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			s.writeProblem(w, r, instance, domainerr.ValidationFailed("invalid limit",
				domainerr.FieldViolation{Path: "limit", Code: "invalid_value", Message: "limit must be a non-negative integer"}))
			return
		}
		limit = n
	}
	list, err := s.svc.QueryAudit(ctx, actor, app.AuditQuery{Limit: limit})
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	events := list.Events
	if events == nil {
		events = []app.AuditEvent{}
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func (s *Server) handleAuditGet(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor app.Actor, id string) {
	ev, err := s.svc.GetAudit(ctx, actor, id)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, ev)
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request, instance string) {
	if !s.cfg.PublicMetrics {
		s.writeProblem(w, r, instance, domainerr.NotFound("not found"))
		return
	}
	w.Header().Set("Content-Type", observability.OpenMetricsContentType)
	if s.metrics == nil {
		_, _ = w.Write([]byte("# EOF\n"))
		return
	}
	_ = s.metrics.WriteOpenMetrics(w)
	_ = r
}
