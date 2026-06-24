package savings

import (
	"sync"
	"sync/atomic"
	"time"
)

// Snapshot is a point-in-time view for dashboards / savings reports.
type Snapshot struct {
	TenantID       string    `json:"tenantId"`
	Hits           int64     `json:"hits"`
	Misses         int64     `json:"misses"`
	QueriesSaved   int64     `json:"queriesSaved"` // alias of hits
	BytesFromCache int64     `json:"bytesFromCache"`
	BytesFromBackend int64   `json:"bytesFromBackend"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// Tracker accumulates per-tenant cache economics in-process (Phase 4 platform metrics).
type Tracker struct {
	mu   sync.RWMutex
	data map[string]*counters
}

type counters struct {
	hits, misses, bytesCache, bytesBackend atomic.Int64
}

func NewTracker() *Tracker {
	return &Tracker{data: make(map[string]*counters)}
}

func (t *Tracker) ensure(tenantID string) *counters {
	t.mu.Lock()
	defer t.mu.Unlock()
	c, ok := t.data[tenantID]
	if !ok {
		c = &counters{}
		t.data[tenantID] = c
	}
	return c
}

func (t *Tracker) RecordHit(tenantID string, bytes int) {
	if t == nil {
		return
	}
	c := t.ensure(tenantID)
	c.hits.Add(1)
	c.bytesCache.Add(int64(bytes))
}

func (t *Tracker) RecordMiss(tenantID string, bytes int) {
	if t == nil {
		return
	}
	c := t.ensure(tenantID)
	c.misses.Add(1)
	c.bytesBackend.Add(int64(bytes))
}

func (t *Tracker) Snapshot(tenantID string) Snapshot {
	if t == nil {
		return Snapshot{TenantID: tenantID}
	}
	t.mu.RLock()
	c := t.data[tenantID]
	t.mu.RUnlock()
	if c == nil {
		return Snapshot{TenantID: tenantID, UpdatedAt: time.Now().UTC()}
	}
	hits := c.hits.Load()
	return Snapshot{
		TenantID:         tenantID,
		Hits:             hits,
		Misses:           c.misses.Load(),
		QueriesSaved:     hits,
		BytesFromCache:   c.bytesCache.Load(),
		BytesFromBackend: c.bytesBackend.Load(),
		UpdatedAt:        time.Now().UTC(),
	}
}

func (t *Tracker) All() []Snapshot {
	if t == nil {
		return nil
	}
	t.mu.RLock()
	ids := make([]string, 0, len(t.data))
	for id := range t.data {
		ids = append(ids, id)
	}
	t.mu.RUnlock()
	out := make([]Snapshot, 0, len(ids))
	for _, id := range ids {
		out = append(out, t.Snapshot(id))
	}
	return out
}
