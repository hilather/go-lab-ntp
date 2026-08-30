package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/hilather/go-lab-ntp/internal/config"
	"github.com/hilather/go-lab-ntp/internal/model"
)

func diffStates(before, after *model.State) ([]DiffEntry, string, error) {
	bt, err := jsonTree(before)
	if err != nil {
		return nil, "", err
	}
	at, err := jsonTree(after)
	if err != nil {
		return nil, "", err
	}
	var out []DiffEntry
	walkDiff(bt, at, "", &out)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, formatHumanDiff(out), nil
}

func jsonTree(st *model.State) (any, error) {
	if st == nil {
		return map[string]any{}, nil
	}
	raw, err := config.CanonicalJSON(st)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var tree any
	if err := dec.Decode(&tree); err != nil {
		return nil, err
	}
	return tree, nil
}

func walkDiff(before, after any, path string, out *[]DiffEntry) {
	if deepEqualJSON(before, after) {
		return
	}
	bm, bMap := before.(map[string]any)
	am, aMap := after.(map[string]any)
	if bMap && aMap {
		keys := map[string]struct{}{}
		for k := range bm {
			keys[k] = struct{}{}
		}
		for k := range am {
			keys[k] = struct{}{}
		}
		ks := make([]string, 0, len(keys))
		for k := range keys {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		for _, k := range ks {
			bv, bOk := bm[k]
			av, aOk := am[k]
			p := joinJSONPath(path, k)
			switch {
			case !bOk:
				*out = append(*out, DiffEntry{Path: p, Op: "add", After: mustJSON(av)})
			case !aOk:
				*out = append(*out, DiffEntry{Path: p, Op: "remove", Before: mustJSON(bv)})
			default:
				walkDiff(bv, av, p, out)
			}
		}
		return
	}
	bl, bArr := before.([]any)
	al, aArr := after.([]any)
	if bArr && aArr {
		n := len(bl)
		if len(al) > n {
			n = len(al)
		}
		for i := 0; i < n; i++ {
			p := path + "[" + strconv.Itoa(i) + "]"
			switch {
			case i >= len(bl):
				*out = append(*out, DiffEntry{Path: p, Op: "add", After: mustJSON(al[i])})
			case i >= len(al):
				*out = append(*out, DiffEntry{Path: p, Op: "remove", Before: mustJSON(bl[i])})
			default:
				walkDiff(bl[i], al[i], p, out)
			}
		}
		return
	}
	*out = append(*out, DiffEntry{Path: path, Op: "replace", Before: mustJSON(before), After: mustJSON(after)})
}

func deepEqualJSON(a, b any) bool {
	return bytes.Equal(mustJSON(a), mustJSON(b))
}

func joinJSONPath(parent, key string) string {
	if parent == "" {
		return key
	}
	return parent + "." + key
}

func formatHumanDiff(diff []DiffEntry) string {
	if len(diff) == 0 {
		return ""
	}
	var b strings.Builder
	for _, d := range diff {
		fmt.Fprintf(&b, "%s %s\n", d.Op, d.Path)
	}
	return b.String()
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("null")
	}
	return b
}
