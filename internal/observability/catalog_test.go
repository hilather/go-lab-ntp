package observability

import (
	"strings"
	"testing"
)

func TestCatalogNoClientIPLabels(t *testing.T) {
	for _, m := range Metrics() {
		for _, l := range m.Labels {
			if ForbiddenLabel(l) || strings.Contains(strings.ToLower(l), "ip") {
				t.Errorf("metric %s has forbidden label %s", m.Name, l)
			}
		}
	}
	for _, f := range ForbiddenLabels {
		if f == "client_ip" {
			return
		}
	}
	t.Fatal("ForbiddenLabels must include client_ip")
}

func TestOversizeDecisionLabel(t *testing.T) {
	if PacketDecision("oversize") != "oversize" {
		t.Fatal("oversize")
	}
	if PacketDecision("serve") != "serve" {
		t.Fatal("serve")
	}
}

func TestRegistryOpenMetrics(t *testing.T) {
	r := NewRegistry()
	r.Inc(MetricPacketsTotal, map[string]string{"version": "4", "decision": "oversize"}, 1)
	r.Inc(MetricPacketsTotal, map[string]string{"version": "4", "decision": "serve"}, 2)
	r.Set(MetricUDPInflight, nil, 3)
	var b strings.Builder
	if err := r.WriteOpenMetrics(&b); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, "labntp_packets_total") {
		t.Fatal(out)
	}
	if !strings.Contains(out, `decision="oversize"`) {
		t.Fatal(out)
	}
	if !strings.HasSuffix(out, "# EOF\n") {
		t.Fatal(out)
	}
	if strings.Contains(out, "client_ip") {
		t.Fatal("client IP leaked into scrape")
	}
}

func TestEvaluateReady(t *testing.T) {
	p := Evaluate(Facts{NTPBound: true, SnapshotUp: true, MgmtOff: true})
	if !p.Live || !p.Ready {
		t.Fatalf("%+v", p)
	}
	p = Evaluate(Facts{NTPBound: true, SnapshotUp: true})
	if p.Ready {
		t.Fatal("mgmt unbound should not be ready")
	}
}
