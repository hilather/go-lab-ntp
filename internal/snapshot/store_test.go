package snapshot

import "testing"

func TestStoreSwap(t *testing.T) {
	st := NewStore()
	if st.Load() != nil {
		t.Fatal("empty")
	}
	a := &Snapshot{Generation: 1}
	b := &Snapshot{Generation: 2}
	st.InstallBootstrap(a)
	if st.Load() != a || st.Bootstrap() != a {
		t.Fatal("bootstrap")
	}
	prev := st.Swap(b)
	if prev != a || st.Load() != b || st.Previous() != a {
		t.Fatal("swap")
	}
}
