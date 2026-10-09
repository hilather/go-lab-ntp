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
