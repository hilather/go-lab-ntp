package ntpserver

import (
	"sync"
	"time"
)

type queryLimiter struct {
	rate  float64
	burst float64
	now   func() time.Time

	mu      sync.Mutex
	buckets map[string]*qbucket
}

type qbucket struct {
	tokens float64
	last   time.Time
}

// maxQueryBuckets matches DefaultMaxInflight and the regression ceiling.
const maxQueryBuckets = 1024

func newQueryLimiter(rate, burst float64, now func() time.Time) *queryLimiter {
	if now == nil {
		now = time.Now
	}
	if rate <= 0 {
		rate = 1
	}
	if burst <= 0 {
		burst = rate * 2
	}
	return &queryLimiter{rate: rate, burst: burst, now: now, buckets: map[string]*qbucket{}}
}

func (l *queryLimiter) setRate(rate, burst float64) {
	if l == nil {
		return
	}
	if rate <= 0 {
		rate = 1
	}
	if burst <= 0 {
		burst = rate * 2
	}
	l.mu.Lock()
	l.rate = rate
	l.burst = burst
	l.mu.Unlock()
}

func (l *queryLimiter) allow(key string) bool {
	if l == nil {
		return true
	}
	if key == "" {
		key = "unknown"
	}
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.evictIdleLocked(now)
	b := l.buckets[key]
	if b == nil {
		if len(l.buckets) >= maxQueryBuckets {
			l.evictOldestLocked()
		}
		b = &qbucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}
	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * l.rate
		if b.tokens > l.burst {
			b.tokens = l.burst
		}
		b.last = now
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (l *queryLimiter) evictIdleLocked(now time.Time) {
	if l == nil || len(l.buckets) == 0 {
		return
	}
	idleFor := 30 * time.Second
	if l.rate > 0 {
		refill := time.Duration(float64(time.Second) * (l.burst / l.rate) * 4)
		if refill > idleFor {
			idleFor = refill
		}
	}
	for k, b := range l.buckets {
		if now.Sub(b.last) > idleFor {
			delete(l.buckets, k)
		}
	}
}

func (l *queryLimiter) evictOldestLocked() {
	var oldestKey string
	var oldest time.Time
	first := true
	for k, b := range l.buckets {
		if first || b.last.Before(oldest) {
			first = false
			oldest = b.last
			oldestKey = k
		}
	}
	if oldestKey != "" {
		delete(l.buckets, oldestKey)
	}
}
