package observability

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// MaxSeriesPerMetric bounds cardinality.
const MaxSeriesPerMetric = 256

// OpenMetricsContentType is the scrape Content-Type.
const OpenMetricsContentType = "application/openmetrics-text; version=1.0.0; charset=utf-8"

var defaultBuckets = []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

// Sample is one scraped series.
type Sample struct {
	Name    string
	Kind    Kind
	Labels  map[string]string
	Value   float64
	Buckets []HistBucket
	Sum     float64
	Count   uint64
}

// HistBucket is a cumulative histogram bucket.
type HistBucket struct {
	Le    float64
	Count uint64
	Inf   bool
}

type seriesKey struct {
	name   string
	labels string
}

type hist struct {
	bounds []float64
	counts []uint64
	inf    uint64
	sum    float64
	n      uint64
}

// Registry is an in-process metric store.
type Registry struct {
	mu       sync.Mutex
	defs     map[string]MetricDef
	counters map[seriesKey]float64
	gauges   map[seriesKey]float64
	hists    map[seriesKey]*hist
	seriesN  map[string]int
	dropped  atomic.Int64
}

// NewRegistry returns an empty store keyed by the frozen catalog.
func NewRegistry() *Registry {
	defs := Metrics()
	m := make(map[string]MetricDef, len(defs))
	for _, d := range defs {
		m[d.Name] = d
	}
	return &Registry{
		defs:     m,
		counters: map[seriesKey]float64{},
		gauges:   map[seriesKey]float64{},
		hists:    map[seriesKey]*hist{},
		seriesN:  map[string]int{},
	}
}

// Dropped is the count of rejected increments.
func (r *Registry) Dropped() int64 {
	if r == nil {
		return 0
	}
	return r.dropped.Load()
}

// Inc adds n to a counter. Invalid labels are dropped, never recorded.
func (r *Registry) Inc(name string, labels map[string]string, n float64) {
	if r == nil || n == 0 {
		return
	}
	r.observe(name, KindCounter, labels, n, false)
}

// Set writes a gauge.
func (r *Registry) Set(name string, labels map[string]string, v float64) {
	if r == nil {
		return
	}
	r.observe(name, KindGauge, labels, v, true)
}

// Observe records a histogram observation.
func (r *Registry) Observe(name string, labels map[string]string, v float64) {
	if r == nil {
		return
	}
	r.observe(name, KindHistogram, labels, v, false)
}

func (r *Registry) observe(name string, want Kind, labels map[string]string, v float64, set bool) {
	def, ok := r.defs[name]
	if !ok || def.Kind != want {
		r.drop("unknown_metric")
		return
	}
	if err := checkLabelsDef(def, labels); err != nil {
		r.drop(LabelReason(err))
		return
	}
	clean := filterLabels(def, labels)
	key := seriesKey{name: name, labels: encodeLabels(def.Labels, clean)}

	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.ensureSeries(name, key, def.Kind) {
		r.dropLocked("cardinality")
		return
	}
	switch def.Kind {
	case KindCounter:
		r.counters[key] += v
	case KindGauge:
		if set {
			r.gauges[key] = v
		} else {
			r.gauges[key] += v
		}
	case KindHistogram:
		h := r.hists[key]
		if h == nil {
			h = newHist()
			r.hists[key] = h
		}
		h.add(v)
	}
}

func (r *Registry) ensureSeries(name string, key seriesKey, kind Kind) bool {
	switch kind {
	case KindCounter:
		if _, ok := r.counters[key]; ok {
			return true
		}
	case KindGauge:
		if _, ok := r.gauges[key]; ok {
			return true
		}
	case KindHistogram:
		if _, ok := r.hists[key]; ok {
			return true
		}
	}
	if r.seriesN[name] >= MaxSeriesPerMetric {
		return false
	}
	r.seriesN[name]++
	return true
}

func (r *Registry) drop(reason string) {
	if reason == "" {
		reason = "invalid"
	}
	r.mu.Lock()
	r.dropLocked(reason)
	r.mu.Unlock()
}

func (r *Registry) dropLocked(reason string) {
	r.dropped.Add(1)
	def, ok := r.defs[MetricTelemetryDropped]
	if !ok {
		return
	}
	labels := map[string]string{"reason": reason}
	key := seriesKey{name: MetricTelemetryDropped, labels: encodeLabels(def.Labels, labels)}
	if _, ok := r.counters[key]; !ok {
		if r.seriesN[MetricTelemetryDropped] >= MaxSeriesPerMetric {
			return
		}
		r.seriesN[MetricTelemetryDropped]++
	}
	r.counters[key]++
}

// Snapshot copies current series.
func (r *Registry) Snapshot() []Sample {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Sample
	for key, v := range r.counters {
		out = append(out, Sample{Name: key.name, Kind: KindCounter, Labels: scrapeLabels(key.name, key.labels), Value: v})
	}
	for key, v := range r.gauges {
		out = append(out, Sample{Name: key.name, Kind: KindGauge, Labels: scrapeLabels(key.name, key.labels), Value: v})
	}
	for key, h := range r.hists {
		b, sum, n := h.snapshot()
		out = append(out, Sample{Name: key.name, Kind: KindHistogram, Labels: scrapeLabels(key.name, key.labels), Buckets: b, Sum: sum, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return encodeLabels(sortedKeys(out[i].Labels), out[i].Labels) < encodeLabels(sortedKeys(out[j].Labels), out[j].Labels)
	})
	return out
}

// Get returns the counter/gauge value for an exact label set.
func (r *Registry) Get(name string, labels map[string]string) (float64, bool) {
	if r == nil {
		return 0, false
	}
	def, ok := r.defs[name]
	if !ok {
		return 0, false
	}
	key := seriesKey{name: name, labels: encodeLabels(def.Labels, filterLabels(def, labels))}
	r.mu.Lock()
	defer r.mu.Unlock()
	switch def.Kind {
	case KindCounter:
		v, ok := r.counters[key]
		return v, ok
	case KindGauge:
		v, ok := r.gauges[key]
		return v, ok
	default:
		if h, ok := r.hists[key]; ok {
			return float64(h.n), true
		}
		return 0, false
	}
}

// WritePrometheus writes the in-memory scrape in Prometheus text format.
func (r *Registry) WritePrometheus(w io.Writer) error {
	if r == nil || w == nil {
		return nil
	}
	var last string
	for _, s := range r.Snapshot() {
		def, _ := LookupMetric(s.Name)
		if s.Name != last {
			if _, err := fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n", s.Name, def.Help, s.Name, def.Kind); err != nil {
				return err
			}
			last = s.Name
		}
		switch s.Kind {
		case KindHistogram:
			for _, b := range s.Buckets {
				le := "+Inf"
				if !b.Inf {
					le = trimFloat(b.Le)
				}
				if _, err := fmt.Fprintf(w, "%s_bucket%s %d\n", s.Name, promLabels(mergeLe(s.Labels, le)), b.Count); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintf(w, "%s_sum%s %s\n%s_count%s %d\n", s.Name, promLabels(s.Labels), trimFloat(s.Sum), s.Name, promLabels(s.Labels), s.Count); err != nil {
				return err
			}
		default:
			if _, err := fmt.Fprintf(w, "%s%s %s\n", s.Name, promLabels(s.Labels), trimFloat(s.Value)); err != nil {
				return err
			}
		}
	}
	return nil
}

// WriteOpenMetrics writes Prometheus text plus the OpenMetrics EOF marker.
func (r *Registry) WriteOpenMetrics(w io.Writer) error {
	if err := r.WritePrometheus(w); err != nil {
		return err
	}
	if w == nil {
		return nil
	}
	_, err := io.WriteString(w, "# EOF\n")
	return err
}

func newHist() *hist {
	return &hist{bounds: append([]float64(nil), defaultBuckets...), counts: make([]uint64, len(defaultBuckets))}
}

func (h *hist) add(v float64) {
	h.n++
	h.sum += v
	placed := false
	for i, b := range h.bounds {
		if v <= b {
			h.counts[i]++
			placed = true
			break
		}
	}
	if !placed {
		h.inf++
	}
}

func (h *hist) snapshot() ([]HistBucket, float64, uint64) {
	out := make([]HistBucket, 0, len(h.bounds)+1)
	var cum uint64
	for i, b := range h.bounds {
		cum += h.counts[i]
		out = append(out, HistBucket{Le: b, Count: cum})
	}
	cum += h.inf
	out = append(out, HistBucket{Count: cum, Inf: true})
	return out, h.sum, h.n
}

func filterLabels(def MetricDef, in map[string]string) map[string]string {
	if len(def.Labels) == 0 {
		return nil
	}
	out := make(map[string]string, len(def.Labels))
	for _, k := range def.Labels {
		if v, ok := in[k]; ok {
			out[k] = v
		} else {
			out[k] = ""
		}
	}
	return out
}

func encodeLabels(order []string, labels map[string]string) string {
	if len(order) == 0 {
		return ""
	}
	var b strings.Builder
	for i, k := range order {
		if i > 0 {
			b.WriteByte(1)
		}
		b.WriteString(k)
		b.WriteByte(0)
		if labels != nil {
			b.WriteString(labels[k])
		}
	}
	return b.String()
}

func decodeLabels(enc string) map[string]string {
	if enc == "" {
		return nil
	}
	out := map[string]string{}
	for _, part := range strings.Split(enc, "\x01") {
		k, v, ok := strings.Cut(part, "\x00")
		if !ok {
			continue
		}
		out[k] = v
	}
	return out
}

func scrapeLabels(name, enc string) map[string]string {
	raw := decodeLabels(enc)
	if len(raw) == 0 {
		return raw
	}
	def, ok := LookupMetric(name)
	if !ok {
		return nil
	}
	if checkLabelsDef(def, raw) == nil {
		return raw
	}
	out := make(map[string]string, len(def.Labels))
	for _, k := range def.Labels {
		if ForbiddenLabel(k) {
			continue
		}
		if v, ok := raw[k]; ok {
			out[k] = v
		}
	}
	return out
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func promLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := sortedKeys(labels)
	var b strings.Builder
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(k)
		b.WriteString(`="`)
		b.WriteString(escapeLabel(labels[k]))
		b.WriteByte('"')
	}
	b.WriteByte('}')
	return b.String()
}

func escapeLabel(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

func mergeLe(labels map[string]string, le string) map[string]string {
	out := make(map[string]string, len(labels)+1)
	for k, v := range labels {
		out[k] = v
	}
	out["le"] = le
	return out
}

func trimFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
