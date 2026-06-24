package cachedcursor

import (
	"sync"
	"sync/atomic"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// Entry is an in-memory snapshot cursor for cache-hit pagination.
type Entry struct {
	ID        int64
	TenantID  string
	ConnKey   string
	NS        string
	Docs      []bson.Raw
	Pos       int
	CreatedAt time.Time
	LastUsed  time.Time
	Bytes     int
}

// Store holds short-lived emulated cursors for cache hits.
type Store struct {
	mu      sync.Mutex
	byID    map[int64]*Entry
	nextID  atomic.Int64
	idleTTL time.Duration
	maxBytes int64
	curBytes atomic.Int64
}

func NewStore(idleTTL time.Duration, maxBytes int64) *Store {
	if idleTTL <= 0 {
		idleTTL = 10 * time.Minute
	}
	if maxBytes <= 0 {
		maxBytes = 64 << 20 // 64 MiB global default
	}
	s := &Store{byID: make(map[int64]*Entry), idleTTL: idleTTL, maxBytes: maxBytes}
	s.nextID.Store(1_000_000_000) // separate range from backend cursor registry ids
	return s
}

// Register stores docs and returns a synthetic cursor id. firstBatchSize controls how many
// docs the caller should place in firstBatch (we only track pos for subsequent getMore).
func (s *Store) Register(tenantID, connKey, ns string, docs []bson.Raw, firstBatchSize int) (id int64, first []bson.Raw, restPos int) {
	if firstBatchSize <= 0 {
		firstBatchSize = 101
	}
	if firstBatchSize >= len(docs) {
		// No need to retain cursor
		return 0, docs, len(docs)
	}
	bytes := 0
	for _, d := range docs {
		bytes += len(d)
	}
	// Evict oldest if over budget
	for s.curBytes.Load()+int64(bytes) > s.maxBytes {
		if !s.evictOldest() {
			break
		}
	}
	id = s.nextID.Add(1)
	now := time.Now()
	cp := make([]bson.Raw, len(docs))
	copy(cp, docs)
	ent := &Entry{
		ID: id, TenantID: tenantID, ConnKey: connKey, NS: ns,
		Docs: cp, Pos: firstBatchSize, CreatedAt: now, LastUsed: now, Bytes: bytes,
	}
	s.mu.Lock()
	s.byID[id] = ent
	s.mu.Unlock()
	s.curBytes.Add(int64(bytes))
	return id, docs[:firstBatchSize], firstBatchSize
}

func (s *Store) evictOldest() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	var oldestID int64
	var oldestTime time.Time
	for id, e := range s.byID {
		if oldestID == 0 || e.LastUsed.Before(oldestTime) {
			oldestID = id
			oldestTime = e.LastUsed
		}
	}
	if oldestID == 0 {
		return false
	}
	e := s.byID[oldestID]
	delete(s.byID, oldestID)
	s.curBytes.Add(-int64(e.Bytes))
	return true
}

// NextBatch returns up to batchSize docs and whether the cursor is exhausted (id should be 0).
func (s *Store) NextBatch(id int64, tenantID, connKey string, batchSize int) (ns string, batch []bson.Raw, exhausted bool, ok bool) {
	if batchSize <= 0 {
		batchSize = 101
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, found := s.byID[id]
	if !found || e.TenantID != tenantID || e.ConnKey != connKey {
		return "", nil, true, false
	}
	e.LastUsed = time.Now()
	end := e.Pos + batchSize
	if end > len(e.Docs) {
		end = len(e.Docs)
	}
	batch = e.Docs[e.Pos:end]
	e.Pos = end
	exhausted = e.Pos >= len(e.Docs)
	ns = e.NS
	if exhausted {
		delete(s.byID, id)
		s.curBytes.Add(-int64(e.Bytes))
	}
	return ns, batch, exhausted, true
}

func (s *Store) Kill(id int64, tenantID, connKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.byID[id]
	if !ok || e.TenantID != tenantID || e.ConnKey != connKey {
		return
	}
	delete(s.byID, id)
	s.curBytes.Add(-int64(e.Bytes))
}

func (s *Store) KillMany(ids []int64, tenantID, connKey string) {
	for _, id := range ids {
		s.Kill(id, tenantID, connKey)
	}
}

func (s *Store) CleanupConn(connKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, e := range s.byID {
		if e.ConnKey == connKey {
			s.curBytes.Add(-int64(e.Bytes))
			delete(s.byID, id)
		}
	}
}

func (s *Store) PruneIdle() int {
	cutoff := time.Now().Add(-s.idleTTL)
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for id, e := range s.byID {
		if e.LastUsed.Before(cutoff) {
			s.curBytes.Add(-int64(e.Bytes))
			delete(s.byID, id)
			n++
		}
	}
	return n
}

func (s *Store) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.byID)
}
