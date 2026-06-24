package ratelimit

import (
	"sync"
	"time"
)

// Limiter is a simple per-tenant token bucket.
type Limiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     float64 // tokens per second
	burst    float64
}

type bucket struct {
	tokens float64
	last   time.Time
}

func New(qps, burst int) *Limiter {
	if qps <= 0 {
		qps = 1000
	}
	if burst <= 0 {
		burst = qps
	}
	return &Limiter{
		buckets: make(map[string]*bucket),
		rate:    float64(qps),
		burst:   float64(burst),
	}
}

// Allow reports whether tenant may proceed (consumes 1 token).
func (l *Limiter) Allow(tenantID string) bool {
	if l == nil || tenantID == "" {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[tenantID]
	now := time.Now()
	if !ok {
		l.buckets[tenantID] = &bucket{tokens: l.burst - 1, last: now}
		return true
	}
	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * l.rate
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// Prune removes idle buckets (optional maintenance).
func (l *Limiter) Prune(idle time.Duration) {
	if l == nil {
		return
	}
	cutoff := time.Now().Add(-idle)
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, b := range l.buckets {
		if b.last.Before(cutoff) {
			delete(l.buckets, k)
		}
	}
}
