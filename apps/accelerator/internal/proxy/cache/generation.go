package cache

import (
	"context"
	"fmt"
	"strconv"
	"sync"
)

// GenerationTracker stores per-tenant per-namespace cache generation counters.
// Bumping generation changes effective cache keys without scanning Redis.
type GenerationTracker struct {
	mu   sync.RWMutex
	gens map[string]int64 // key: tenant|db.coll
	store Store           // optional redis persistence
}

func NewGenerationTracker(s Store) *GenerationTracker {
	return &GenerationTracker{gens: make(map[string]int64), store: s}
}

func genKey(tenantID, db, coll string) string {
	return tenantID + "|" + db + "." + coll
}

func redisGenKey(tenantID, db, coll string) string {
	return fmt.Sprintf("nance:tenant:{%s}:ns:%s.%s:generation", tenantID, db, coll)
}

func (g *GenerationTracker) Get(ctx context.Context, tenantID, db, coll string) int64 {
	k := genKey(tenantID, db, coll)
	g.mu.RLock()
	v, ok := g.gens[k]
	g.mu.RUnlock()
	if ok {
		return v
	}
	if g.store != nil {
		if rs, ok := g.store.(*RedisStore); ok {
			cctx, cancel := context.WithTimeout(ctx, rs.getTO)
			defer cancel()
			s, err := rs.client.Get(cctx, redisGenKey(tenantID, db, coll)).Result()
			if err == nil {
				n, _ := strconv.ParseInt(s, 10, 64)
				g.mu.Lock()
				g.gens[k] = n
				g.mu.Unlock()
				return n
			}
		}
	}
	return 0
}

// Bump increments generation (invalidates all prior keys for that ns logically).
func (g *GenerationTracker) Bump(ctx context.Context, tenantID, db, coll string) int64 {
	k := genKey(tenantID, db, coll)
	g.mu.Lock()
	g.gens[k]++
	n := g.gens[k]
	g.mu.Unlock()
	if g.store != nil {
		if rs, ok := g.store.(*RedisStore); ok {
			cctx, cancel := context.WithTimeout(ctx, rs.setTO)
			_ = rs.client.Set(cctx, redisGenKey(tenantID, db, coll), strconv.FormatInt(n, 10), 0).Err()
			cancel()
		}
	}
	return n
}

// KeySuffix returns a segment to append into cache keys.
func (g *GenerationTracker) KeySuffix(ctx context.Context, tenantID, db, coll string) string {
	return fmt.Sprintf("g%d", g.Get(ctx, tenantID, db, coll))
}
