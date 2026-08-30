package observability

import (
	"encoding/json"
	"sort"
)

// CatalogID is the versioned metrics/events document identifier.
const CatalogID = "labntp.dev/metrics/v1alpha1"

// CatalogRelPath is the generated catalog artifact.
const CatalogRelPath = "api/metrics/v1alpha1.json"

// Kind is a catalog metric type.
type Kind string

const (
	KindCounter   Kind = "counter"
	KindGauge     Kind = "gauge"
	KindHistogram Kind = "histogram"
)

// Frozen metric names.
const (
	MetricPacketsTotal         = "labntp_packets_total"
	MetricFilterHitsTotal      = "labntp_filter_hits_total"
	MetricQuerylogDroppedTotal = "labntp_querylog_dropped_total"
	MetricApplyTotal           = "labntp_apply_total"
	MetricHTTPRequestsTotal    = "labntp_http_requests_total"
	MetricMCPCallsTotal        = "labntp_mcp_calls_total"
	MetricAuthFailuresTotal    = "labntp_auth_failures_total"
	MetricUDPInflight          = "labntp_udp_inflight"
	MetricTelemetryDropped     = "labntp_telemetry_dropped_total"
)

// Frozen structured-log event names.
const (
	EventNTPQuery    = "ntp.query"
	EventStateApply  = "state.apply"
	EventStateReset  = "state.reset"
	EventAuthFailure = "auth.failure"
	EventHTTPRequest = "http.request"
	EventMCPCall     = "mcp.call"
)

// AllowedLabels is the default bounded label set. Client IPs are never allowed.
var AllowedLabels = []string{
	"capability",
	"code_class",
	"component",
	"decision",
	"event",
	"filter",
	"reason",
	"result",
	"tool",
	"version",
}

// ForbiddenLabels must never appear on a catalog metric or recorded sample.
var ForbiddenLabels = []string{
	"actor",
	"actor_id",
	"address",
	"authorization",
	"body",
	"client",
	"client_ip",
	"cookie",
	"data",
	"detail",
	"err",
	"error",
	"error_text",
	"from",
	"host",
	"idempotency",
	"idempotency_key",
	"message",
	"password",
	"peer",
	"raw",
	"remote_addr",
	"set_cookie",
	"source_ip",
	"src",
	"src_ip",
	"subject",
	"to",
}

// MetricDef is one catalog row.
type MetricDef struct {
	Name   string   `json:"name"`
	Kind   Kind     `json:"kind"`
	Help   string   `json:"help"`
	Labels []string `json:"labels"`
	Unit   string   `json:"unit,omitempty"`
}

// EventDef is one stable structured-log event.
type EventDef struct {
	Name   string   `json:"name"`
	Fields []string `json:"fields"`
}

// Document is the versioned catalog artifact.
type Document struct {
	ID              string      `json:"id"`
	Version         string      `json:"version"`
	AllowedLabels   []string    `json:"allowedLabels"`
	ForbiddenLabels []string    `json:"forbiddenLabels"`
	Metrics         []MetricDef `json:"metrics"`
	Events          []EventDef  `json:"events"`
}

// EventFields is the frozen slog JSON field set.
var EventFields = []string{
	"timestamp", "level", "event", "component", "request_id",
	"capability", "result", "error_code", "duration_ms",
}

// Metrics returns the frozen first-GA catalog in stable name order.
func Metrics() []MetricDef {
	defs := []MetricDef{
		{Name: MetricPacketsTotal, Kind: KindCounter, Help: "NTP packets by version and decision.", Labels: []string{"version", "decision"}},
		{Name: MetricFilterHitsTotal, Kind: KindCounter, Help: "Packets answered by first-match filter name (cardinality capped).", Labels: []string{"filter"}},
		{Name: MetricQuerylogDroppedTotal, Kind: KindCounter, Help: "Query-log samples dropped because the ring lock was busy.", Labels: nil},
		{Name: MetricApplyTotal, Kind: KindCounter, Help: "Plan/apply/reset commits.", Labels: []string{"result"}},
		{Name: MetricHTTPRequestsTotal, Kind: KindCounter, Help: "Management HTTP requests.", Labels: []string{"code_class", "capability"}},
		{Name: MetricMCPCallsTotal, Kind: KindCounter, Help: "MCP tool invocations.", Labels: []string{"capability"}},
		{Name: MetricAuthFailuresTotal, Kind: KindCounter, Help: "Management authentication failures.", Labels: nil},
		{Name: MetricUDPInflight, Kind: KindGauge, Help: "In-flight NTP packet handlers.", Labels: nil},
		{Name: MetricTelemetryDropped, Kind: KindCounter, Help: "Telemetry samples dropped under policy or cardinality.", Labels: []string{"reason"}},
	}
	sort.Slice(defs, func(i, j int) bool { return defs[i].Name < defs[j].Name })
	for i := range defs {
		defs[i].Labels = append([]string(nil), defs[i].Labels...)
		sort.Strings(defs[i].Labels)
	}
	return defs
}

// Events returns the frozen structured-log event catalog.
func Events() []EventDef {
	names := []string{
		EventNTPQuery, EventStateApply, EventStateReset,
		EventAuthFailure, EventHTTPRequest, EventMCPCall,
	}
	out := make([]EventDef, len(names))
	for i, n := range names {
		out[i] = EventDef{Name: n, Fields: append([]string(nil), EventFields...)}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// LookupMetric returns the catalog definition for name.
func LookupMetric(name string) (MetricDef, bool) {
	def, ok := metricIndex[name]
	return def, ok
}

var metricIndex = func() map[string]MetricDef {
	defs := Metrics()
	m := make(map[string]MetricDef, len(defs))
	for _, d := range defs {
		m[d.Name] = d
	}
	return m
}()

// Catalog returns the versioned document.
func Catalog() Document {
	return Document{
		ID:              CatalogID,
		Version:         "v1alpha1",
		AllowedLabels:   append([]string(nil), AllowedLabels...),
		ForbiddenLabels: append([]string(nil), ForbiddenLabels...),
		Metrics:         Metrics(),
		Events:          Events(),
	}
}

// RenderCatalog is the generated JSON artifact.
func RenderCatalog() ([]byte, error) {
	b, err := json.MarshalIndent(Catalog(), "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
