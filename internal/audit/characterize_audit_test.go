package audit

import (
	"encoding/json"
	"testing"
	"time"
)

func TestCharacterizeAuditRow(t *testing.T) {
	if DefaultMax != 128 {
		t.Fatalf("DefaultMax %d", DefaultMax)
	}
	capped := NewRing(0)
	for i := 0; i < 129; i++ {
		capped.Append(Event{Time: time.Unix(int64(i), 0).UTC(), Capability: "changes.apply"})
	}
	if capped.Len() != 128 {
		t.Fatalf("len %d", capped.Len())
	}
	if _, ok := capped.Get("aud-1"); ok {
		t.Fatal("129th append must drop the oldest")
	}
	second, ok := capped.Get("aud-2")
	if !ok || second.ID != "aud-2" {
		t.Fatalf("aud-2 %+v %v", second, ok)
	}
	newest := capped.List(1)
	if len(newest) != 1 || newest[0].ID != "aud-129" {
		t.Fatalf("newest %+v", newest)
	}

	ring := NewRing(0)
	for i := 0; i < 101; i++ {
		ring.Append(Event{Time: time.Unix(int64(i), 0).UTC(), Capability: "changes.apply"})
	}
	if ring.Len() != 101 {
		t.Fatalf("len %d", ring.Len())
	}
	for _, limit := range []int{0, -1, 101, 1000} {
		page := ring.List(limit)
		if len(page) != 100 {
			t.Fatalf("limit %d -> %d", limit, len(page))
		}
		if page[0].ID != "aud-101" || page[99].ID != "aud-2" {
			t.Fatalf("limit %d newest %s oldest-in-page %s", limit, page[0].ID, page[99].ID)
		}
	}
	one := ring.List(1)
	if len(one) != 1 || one[0].ID != "aud-101" {
		t.Fatalf("newest %+v", one)
	}
	got, ok := ring.Get("aud-1")
	if !ok || got.ID != "aud-1" {
		t.Fatalf("get aud-1 %+v %v", got, ok)
	}

	raw, err := json.Marshal(map[string]any{
		"secret":        "s",
		"secretRef":     "s",
		"secretFile":    "s",
		"token":         "s",
		"password":      "s",
		"authorization": "s",
		"bearer":        "s",
		"credential":    "s",
		"credentials":   "s",
		"apiKey":        "s",
		"api_key":       "s",
		"privateKey":    "s",
		"private_key":   "s",
		"cookie":        "s",
		"keep":          "visible",
		"note":          "-----BEGIN RSA PRIVATE KEY-----\nabc",
	})
	if err != nil {
		t.Fatal(err)
	}
	ev := RedactEvent(Event{
		Reason: "-----BEGIN OPENSSH PRIVATE KEY-----\nxyz",
		Diff: []RedactedEntry{{
			Path:   "spec.note",
			Op:     "replace",
			Before: raw,
			After:  []byte(`{"token":"abc","keep":"yes"}`),
		}},
	})
	if ev.Reason != "[redacted]" {
		t.Fatalf("reason %q", ev.Reason)
	}
	var before map[string]any
	if err := json.Unmarshal(ev.Diff[0].Before, &before); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"secret", "secretRef", "secretFile", "token", "password", "authorization", "bearer", "credential", "credentials", "apiKey", "api_key", "privateKey", "private_key", "cookie", "note"} {
		if before[key] != "[redacted]" {
			t.Fatalf("key %s = %#v", key, before[key])
		}
	}
	if before["keep"] != "visible" {
		t.Fatalf("keep %v", before["keep"])
	}
	var after map[string]any
	if err := json.Unmarshal(ev.Diff[0].After, &after); err != nil {
		t.Fatal(err)
	}
	if after["token"] != "[redacted]" || after["keep"] != "yes" {
		t.Fatalf("after %#v", after)
	}

	pathSecret := RedactEvent(Event{Diff: []RedactedEntry{{
		Path:   "spec.auth.tokens.0.secret",
		Op:     "replace",
		Before: []byte(`"visible"`),
		After:  []byte(`"also"`),
	}}})
	if string(pathSecret.Diff[0].Before) != `"[redacted]"` || string(pathSecret.Diff[0].After) != `"[redacted]"` {
		t.Fatalf("path redaction %s %s", pathSecret.Diff[0].Before, pathSecret.Diff[0].After)
	}
}
