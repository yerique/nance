package policy

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/taeven/nance/accelerator/internal/controlplane/store"
	"github.com/taeven/nance/accelerator/internal/model"
)

const defaultMaxResultBytes = 1 << 20 // 1 MiB

// Decision is the outcome of a policy lookup for one command.
type Decision struct {
	Enabled         bool
	TTL             time.Duration
	MaxResultBytes  int
	CacheKeyVersion int
}

// Engine keeps an in-memory snapshot of tenant cache policies.
type Engine struct {
	store    store.Store
	log      *slog.Logger
	interval time.Duration

	mu       sync.RWMutex
	policies map[string]*model.CachePolicy
}

func NewEngine(s store.Store, log *slog.Logger, refreshInterval time.Duration) *Engine {
	if log == nil {
		log = slog.Default()
	}
	if refreshInterval <= 0 {
		refreshInterval = 30 * time.Second
	}
	return &Engine{
		store:    s,
		log:      log,
		interval: refreshInterval,
		policies: make(map[string]*model.CachePolicy),
	}
}

// Start runs the refresh loop until ctx is done.
func (e *Engine) Start(ctx context.Context) {
	e.refresh(ctx)
	t := time.NewTicker(e.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			e.refresh(ctx)
		}
	}
}

func (e *Engine) refresh(ctx context.Context) {
	tenants, err := e.store.ListTenants(ctx)
	if err != nil {
		e.log.Warn("policy refresh: list tenants", "error", err)
		return
	}
	next := make(map[string]*model.CachePolicy, len(tenants))
	for _, t := range tenants {
		p, err := e.store.GetCachePolicy(ctx, t.ID)
		if err != nil {
			continue
		}
		next[t.ID] = p
	}
	e.mu.Lock()
	e.policies = next
	e.mu.Unlock()
}

// Lookup returns whether caching is enabled for tenant/db/coll via control-plane policy.
// Prefer Resolve for the data-plane path: clients opt in with a "_cache" collection suffix;
// policy only supplies TTL / size / key-version overrides.
func (e *Engine) Lookup(tenantID, db, coll string) Decision {
	d := e.Resolve(tenantID, db, coll)
	e.mu.RLock()
	p := e.policies[tenantID]
	e.mu.RUnlock()
	if p == nil {
		return Decision{Enabled: false}
	}
	ns := db + "." + coll
	cp, ok := p.Collections[ns]
	if !ok || !cp.Enabled {
		return Decision{Enabled: false, CacheKeyVersion: p.CacheKeyVersion}
	}
	d.Enabled = true
	return d
}

// Resolve returns cache settings (TTL, max bytes, key version) for tenant/db/coll.
// Always Enabled=true with sensible defaults so the "_cache" suffix opt-in can use the cache
// without requiring a control-plane collection policy. Per-collection policy still overrides TTL/size.
func (e *Engine) Resolve(tenantID, db, coll string) Decision {
	e.mu.RLock()
	p := e.policies[tenantID]
	e.mu.RUnlock()

	ttlSec := 60
	maxBytes := defaultMaxResultBytes
	keyVer := 1
	if p != nil {
		keyVer = p.CacheKeyVersion
		if keyVer <= 0 {
			keyVer = 1
		}
		if p.DefaultTtlSeconds > 0 {
			ttlSec = p.DefaultTtlSeconds
		}
		ns := db + "." + coll
		if cp, ok := p.Collections[ns]; ok {
			if cp.TTLSeconds > 0 {
				ttlSec = cp.TTLSeconds
			}
			if cp.MaxResultBytes != nil && *cp.MaxResultBytes > 0 {
				maxBytes = *cp.MaxResultBytes
			}
		}
	}
	return Decision{
		Enabled:         true,
		TTL:             time.Duration(ttlSec) * time.Second,
		MaxResultBytes:  maxBytes,
		CacheKeyVersion: keyVer,
	}
}

// Snapshot returns a copy of the current policy map (for tests / debug).
func (e *Engine) Snapshot() map[string]*model.CachePolicy {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make(map[string]*model.CachePolicy, len(e.policies))
	for k, v := range e.policies {
		out[k] = v
	}
	return out
}

// SetForTest injects a policy without the store (unit tests).
func (e *Engine) SetForTest(tenantID string, p *model.CachePolicy) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.policies == nil {
		e.policies = make(map[string]*model.CachePolicy)
	}
	e.policies[tenantID] = p
}
