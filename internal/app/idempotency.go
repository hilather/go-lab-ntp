package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"

	"github.com/hilather/go-lab-controlkit/idem"

	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/model"
)

// idempEntry is the value stored for one key. Plan and apply share the key,
// so a second store merges into the value already stored for that fingerprint.
type idempEntry struct {
	plan  *Plan
	apply *ApplyResult
}

type idempCache struct {
	mu sync.Mutex
	c  *idem.Cache[idempEntry]
}

func newIdempCache(max int) *idempCache {
	if max <= 0 {
		max = defaultIdempotencyMax
	}
	c, err := idem.New[idempEntry](max, idem.LRU)
	if err != nil {
		panic("idem.New: " + err.Error())
	}
	return &idempCache{c: c}
}

func (c *idempCache) lookup(key, fp string) (*idempEntry, error) {
	if c == nil || c.c == nil || key == "" {
		return nil, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok, err := c.c.Lookup(key, fp)
	if err != nil {
		if errors.Is(err, idem.ErrConflict) {
			return nil, domainerr.IdempotencyConflict("idempotency key reused with a different request")
		}
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	cp := v
	return &cp, nil
}

func (c *idempCache) storePlan(key, fp string, p *Plan) {
	if c == nil || c.c == nil || key == "" || p == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.storeMerged(key, fp, func(e idempEntry) idempEntry {
		e.plan = clonePlan(p)
		return e
	})
}

func (c *idempCache) storeApply(key, fp string, r *ApplyResult) {
	if c == nil || c.c == nil || key == "" || r == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.storeMerged(key, fp, func(e idempEntry) idempEntry {
		e.apply = cloneApply(r)
		return e
	})
}

// storeMerged loads the value for key, applies set, and stores the result.
// A miss or a different fingerprint starts from a zero entry, which drops
// the previous plan or apply. The caller holds c.mu.
func (c *idempCache) storeMerged(key, fp string, set func(idempEntry) idempEntry) {
	cur, ok, err := c.c.Lookup(key, fp)
	if err != nil || !ok {
		cur = idempEntry{}
	}
	c.c.Store(key, fp, set(cur))
}

func (c *idempCache) evict(key string) {
	if c == nil || c.c == nil || key == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.c.Evict(key)
}

func (c *idempCache) clear() {
	if c == nil || c.c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.c.Clear()
}

type changeFingerprint struct {
	Reason     string            `json:"reason"`
	Operations []model.Operation `json:"operations"`
}

func fingerprintChange(in ChangeIn) (string, error) {
	b, err := json.Marshal(changeFingerprint{
		Reason:     in.Reason,
		Operations: in.Operations,
	})
	if err != nil {
		return "", domainerr.Internal("idempotency fingerprint: " + err.Error())
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func clonePlan(p *Plan) *Plan {
	if p == nil {
		return nil
	}
	out := *p
	out.Diff = cloneDiff(p.Diff)
	if p.Warnings != nil {
		out.Warnings = append([]Warning(nil), p.Warnings...)
	}
	out.Operations = append([]model.Operation(nil), p.Operations...)
	return &out
}

func cloneDiff(diff []DiffEntry) []DiffEntry {
	if diff == nil {
		return nil
	}
	out := make([]DiffEntry, len(diff))
	for i, d := range diff {
		out[i] = d
		if d.Before != nil {
			out[i].Before = append(json.RawMessage(nil), d.Before...)
		}
		if d.After != nil {
			out[i].After = append(json.RawMessage(nil), d.After...)
		}
	}
	return out
}

func cloneApply(r *ApplyResult) *ApplyResult {
	if r == nil {
		return nil
	}
	out := *r
	out.Plan = *clonePlan(&r.Plan)
	return &out
}
