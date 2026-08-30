package querylog

import (
	"testing"
	"time"
)

func TestRingInsertListReset(t *testing.T) {
	r := New(2)
	if !r.TryInsert(Entry{Filter: "a", WhenHost: time.Now()}) {
		t.Fatal("insert a")
	}
	if !r.TryInsert(Entry{Filter: "b"}) {
		t.Fatal("insert b")
	}
	if !r.TryInsert(Entry{Filter: "c"}) {
		t.Fatal("insert c")
	}
	got := r.List()
	if len(got) != 2 || got[0].Filter != "c" || got[1].Filter != "b" {
		t.Fatalf("%+v", got)
	}
	r.Reset()
	if len(r.List()) != 0 {
		t.Fatal("reset")
	}
}
