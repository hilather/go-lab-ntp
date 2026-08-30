package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"

	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/model"
)

type idempEntry struct {
	key   string
	fp    string
	plan  *Plan
	apply *ApplyResult
	prev  *idempEntry
	next  *idempEntry
}

type idempCache struct {
	mu      sync.Mutex
	max     int
	entries map[string]*idempEntry
	head    *idempEntry
	tail    *idempEntry
}

func newIdempCache(max int) *idempCache {
	if max <= 0 {
		max = defaultIdempotencyMax
	}
	return &idempCache{
		max:     max,
		entries: map[string]*idempEntry{},
	}
}

func (c *idempCache) lookup(key, fp string) (*idempEntry, error) {
	if c == nil || key == "" {
		return nil, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return nil, nil
	}
	if e.fp != fp {
		return nil, domainerr.IdempotencyConflict("idempotency key reused with a different request")
	}
	c.moveFrontLocked(e)
	return e, nil
}

func (c *idempCache) storePlan(key, fp string, p *Plan) {
	if c == nil || key == "" || p == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[key]; ok && e.fp == fp {
		e.plan = clonePlan(p)
		c.moveFrontLocked(e)
		return
	}
	c.insertFrontLocked(&idempEntry{key: key, fp: fp, plan: clonePlan(p)})
}

func (c *idempCache) storeApply(key, fp string, r *ApplyResult) {
	if c == nil || key == "" || r == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[key]; ok && e.fp == fp {
		e.apply = cloneApply(r)
		c.moveFrontLocked(e)
		return
	}
	c.insertFrontLocked(&idempEntry{key: key, fp: fp, apply: cloneApply(r)})
}

func (c *idempCache) evict(key string) {
	if c == nil || key == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[key]; ok {
		c.removeLocked(e)
	}
}

func (c *idempCache) clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = map[string]*idempEntry{}
	c.head = nil
	c.tail = nil
}

func (c *idempCache) insertFrontLocked(e *idempEntry) {
	if old, ok := c.entries[e.key]; ok {
		c.removeLocked(old)
	}
	c.entries[e.key] = e
	e.prev = nil
	e.next = c.head
	if c.head != nil {
		c.head.prev = e
	}
	c.head = e
	if c.tail == nil {
		c.tail = e
	}
	for len(c.entries) > c.max && c.tail != nil {
		c.removeLocked(c.tail)
	}
}

func (c *idempCache) moveFrontLocked(e *idempEntry) {
	if e == nil || e == c.head {
		return
	}
	c.unlinkLocked(e)
	e.prev = nil
	e.next = c.head
	if c.head != nil {
		c.head.prev = e
	}
	c.head = e
	if c.tail == nil {
		c.tail = e
	}
}

func (c *idempCache) removeLocked(e *idempEntry) {
	if e == nil {
		return
	}
	delete(c.entries, e.key)
	c.unlinkLocked(e)
}

func (c *idempCache) unlinkLocked(e *idempEntry) {
	if e.prev != nil {
		e.prev.next = e.next
	} else {
		c.head = e.next
	}
	if e.next != nil {
		e.next.prev = e.prev
	} else {
		c.tail = e.prev
	}
	e.prev = nil
	e.next = nil
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
