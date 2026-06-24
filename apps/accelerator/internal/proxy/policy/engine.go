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
	Enabled        bool
	TTL            time.Duration
	MaxResultBytes int
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

// Lookup returns whether caching is enabled for tenant/db/coll.
// Collections must be explicitly enabled (disabled by default).
func (e *Engine) Lookup(tenantID, db, coll string) Decision {
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
	ttlSec := cp.TTLSeconds
	if ttlSec <= 0 {
		ttlSec = p.DefaultTtlSeconds
	}
	if ttlSec <= 0 {
		ttlSec = 60
	}
	maxBytes := defaultMaxResultBytes
	if cp.MaxResultBytes != nil && *cp.MaxResultBytes > 0 {
		maxBytes = *cp.MaxResultBytes
	}
	return Decision{
		Enabled:         true,
		TTL:             time.Duration(ttlSec) * time.Second,
		MaxResultBytes:  maxBytes,
		CacheKeyVersion: p.CacheKeyVersion,
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
