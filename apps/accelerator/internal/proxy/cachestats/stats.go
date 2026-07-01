package cachestats

import (
	"sync"
	"sync/atomic"
	"time"
)

// CollectionSnapshot is hit/miss accounting for one tenant + real collection.
type CollectionSnapshot struct {
	TenantID   string    `json:"tenantId"`
	DB         string    `json:"db"`
	Collection string    `json:"collection"`
	Hits       int64     `json:"hits"`
	Misses     int64     `json:"misses"`
	Total      int64     `json:"total"`
	HitRatio   float64   `json:"hitRatio"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// TenantSnapshot aggregates all collections for a tenant on this proxy process.
type TenantSnapshot struct {
	TenantID    string               `json:"tenantId"`
	Hits        int64                `json:"hits"`
	Misses      int64                `json:"misses"`
	HitRatio    float64              `json:"hitRatio"`
	Collections []CollectionSnapshot `json:"collections"`
	UpdatedAt   time.Time            `json:"updatedAt"`
}

type counters struct {
	hits   atomic.Int64
	misses atomic.Int64
}

// Tracker records cache hits/misses per collection using atomics + sync.Map only.
// Hot path does no I/O and no tenant Mongo access (no impact on DB latency).
type Tracker struct {
	byColl sync.Map // tenant\x00db\x00coll -> *counters
}

func NewTracker() *Tracker { return &Tracker{} }

func makeKey(tenantID, db, coll string) string {
	return tenantID + "\x00" + db + "\x00" + coll
}

func (t *Tracker) get(tenantID, db, coll string) *counters {
	if t == nil {
		return &counters{}
	}
	k := makeKey(tenantID, db, coll)
	if v, ok := t.byColl.Load(k); ok {
		return v.(*counters)
	}
	c := &counters{}
	actual, _ := t.byColl.LoadOrStore(k, c)
	return actual.(*counters)
}

func (t *Tracker) RecordHit(tenantID, db, coll string) {
	if t == nil || tenantID == "" || coll == "" {
		return
	}
	t.get(tenantID, db, coll).hits.Add(1)
}

func (t *Tracker) RecordMiss(tenantID, db, coll string) {
	if t == nil || tenantID == "" || coll == "" {
		return
	}
	t.get(tenantID, db, coll).misses.Add(1)
}

func ratio(hits, misses int64) float64 {
	tot := hits + misses
	if tot == 0 {
		return 0
	}
	return float64(hits) / float64(tot)
}

func (t *Tracker) SnapshotCollection(tenantID, db, coll string) CollectionSnapshot {
	now := time.Now().UTC()
	out := CollectionSnapshot{TenantID: tenantID, DB: db, Collection: coll, UpdatedAt: now}
	if t == nil {
		return out
	}
	v, ok := t.byColl.Load(makeKey(tenantID, db, coll))
	if !ok {
		return out
	}
	c := v.(*counters)
	h, m := c.hits.Load(), c.misses.Load()
	out.Hits, out.Misses, out.Total = h, m, h+m
	out.HitRatio = ratio(h, m)
	return out
}

func (t *Tracker) SnapshotTenant(tenantID string) TenantSnapshot {
	now := time.Now().UTC()
	out := TenantSnapshot{TenantID: tenantID, Collections: make([]CollectionSnapshot, 0), UpdatedAt: now}
	if t == nil || tenantID == "" {
		return out
	}
	prefix := tenantID + "\x00"
	t.byColl.Range(func(k, v any) bool {
		ks, ok := k.(string)
		if !ok || len(ks) <= len(prefix) || ks[:len(prefix)] != prefix {
			return true
		}
		rest := ks[len(prefix):]
		db, coll := "", ""
		for i := 0; i < len(rest); i++ {
			if rest[i] == 0 {
				db = rest[:i]
				coll = rest[i+1:]
				break
			}
		}
		if coll == "" {
			return true
		}
		c := v.(*counters)
		h, m := c.hits.Load(), c.misses.Load()
		out.Hits += h
		out.Misses += m
		out.Collections = append(out.Collections, CollectionSnapshot{
			TenantID: tenantID, DB: db, Collection: coll,
			Hits: h, Misses: m, Total: h + m, HitRatio: ratio(h, m), UpdatedAt: now,
		})
		return true
	})
	out.HitRatio = ratio(out.Hits, out.Misses)
	return out
}

func (t *Tracker) SnapshotAll() []CollectionSnapshot {
	now := time.Now().UTC()
	out := make([]CollectionSnapshot, 0)
	if t == nil {
		return out
	}
	t.byColl.Range(func(k, v any) bool {
		ks, _ := k.(string)
		parts := splitNull(ks)
		if len(parts) != 3 {
			return true
		}
		c := v.(*counters)
		h, m := c.hits.Load(), c.misses.Load()
		out = append(out, CollectionSnapshot{
			TenantID: parts[0], DB: parts[1], Collection: parts[2],
			Hits: h, Misses: m, Total: h + m, HitRatio: ratio(h, m), UpdatedAt: now,
		})
		return true
	})
	return out
}

func splitNull(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == 0 {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}
