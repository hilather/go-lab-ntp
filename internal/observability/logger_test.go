package observability

import (
	"bytes"
	"strings"
	"testing"
)

func TestLoggerJSON(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf, LevelInfo)
	l.Log(Record{Event: EventStateApply, Component: "app", Result: "ok"})
	s := buf.String()
	if !strings.Contains(s, `"event":"state.apply"`) {
		t.Fatal(s)
	}
	if strings.Contains(s, "client_ip") || strings.Contains(s, "Authorization") {
		t.Fatal(s)
	}
}
