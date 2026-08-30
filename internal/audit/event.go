package audit

import (
	"encoding/json"
	"time"

	"github.com/hilather/go-lab-ntp/internal/model"
)

// Result values on Event.
const (
	ResultOK     = "ok"
	ResultDenied = "denied"
	ResultError  = "error"
)

// Event is one mutation or security record. Payloads are redacted.
type Event struct {
	ID         string          `json:"id"`
	Time       time.Time       `json:"time"`
	ActorID    string          `json:"actorId,omitempty"`
	ActorClass string          `json:"actorClass,omitempty"`
	Transport  string          `json:"transport,omitempty"`
	Capability string          `json:"capability,omitempty"`
	Reason     string          `json:"reason,omitempty"`
	Previous   model.Revision  `json:"previous,omitempty"`
	Revision   model.Revision  `json:"revision,omitempty"`
	Result     string          `json:"result,omitempty"`
	ErrorCode  string          `json:"errorCode,omitempty"`
	Diff       []RedactedEntry `json:"diff,omitempty"`
}

// RedactedEntry is one canonical-path change after secret redaction.
type RedactedEntry struct {
	Path   string          `json:"path"`
	Op     string          `json:"op"`
	Before json.RawMessage `json:"before,omitempty"`
	After  json.RawMessage `json:"after,omitempty"`
}

// RedactEvent copies ev with secret material stripped from reason/diff.
func RedactEvent(ev Event) Event {
	out := ev
	out.Reason = redactText(out.Reason)
	if len(out.Diff) > 0 {
		diff := make([]RedactedEntry, len(out.Diff))
		for i, d := range out.Diff {
			before, after := redactJSON(d.Before), redactJSON(d.After)
			if secretPath(d.Path) {
				before, after = []byte(`"`+redacted+`"`), []byte(`"`+redacted+`"`)
			}
			diff[i] = RedactedEntry{
				Path:   d.Path,
				Op:     d.Op,
				Before: before,
				After:  after,
			}
		}
		out.Diff = diff
	}
	return out
}
