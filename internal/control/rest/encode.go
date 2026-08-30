package rest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/hilather/go-lab-ntp/internal/config"
)

func (s *Server) writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := marshalAPI(v)
	if err != nil {
		http.Error(w, `{"type":"urn:labntp:error:internal-error","title":"Internal error","status":500,"code":"internal_error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func (s *Server) writeBytes(w http.ResponseWriter, status int, contentType string, body []byte) {
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func marshalAPI(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var tree any
	if err := dec.Decode(&tree); err != nil {
		return nil, err
	}
	config.FormatWireTree(tree)
	return json.Marshal(tree)
}

func rfc3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}
