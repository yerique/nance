package policy

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/taeven/nance/accelerator/internal/controlplane/store"
	"github.com/taeven/nance/accelerator/internal/model"
)

const (
	// DefaultTTLSeconds is applied to every _cache-suffixed collection unless
	// the connection overrides defaultTtlSeconds or a per-collection ttlSeconds.
	DefaultTTLSeconds     = 60
	defaultMaxResultBytes = 1 << 20 // 1 MiB
)

// Decision is the outcome of a policy lookup for one command.
type Decision struct {
	Enabled         bool
	TTL             time.Duration
	MaxResultBytes  int
	CacheKeyVersion int
}

// Engine keeps an in-memory snapshot of per-connection cache policies.
type Engine struct {
	store    store.Store
	log      *slog.Logger
	interval time.Duration

	mu       sync.RWMutex
	policies map[string]*model.CachePolicy // keyed by connectionID
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
	list, err := e.store.ListAllCachePolicies(ctx)
	if err != nil {
		// Fallback: build from connections when table empty / older code paths
		conns, cerr := e.listAllConnections(ctx)
		if cerr != nil {
			e.log.Warn("policy refresh failed", "error", err, "connections_error", cerr)
			return
		}
		next := make(map[string]*model.CachePolicy, len(conns))
		for _, c := range conns {
			p, perr := e.store.GetCachePolicy(ctx, c.ID)
			if perr != nil {
				continue
			}
			next[c.ID] = p
		}
		e.mu.Lock()
		e.policies = next
		e.mu.Unlock()
		return
	}
	next := make(map[string]*model.CachePolicy, len(list))
	for _, p := range list {
		if p.ConnectionID == "" {
			continue
		}
		next[p.ConnectionID] = p
	}
	// Also ensure connections without a row still resolve defaults via Resolve
	e.mu.Lock()
	e.policies = next
	e.mu.Unlock()
}

func (e *Engine) listAllConnections(ctx context.Context) ([]*model.Connection, error) {
	tenants, err := e.store.ListTenants(ctx)
	if err != nil {
		return nil, err
	}
	var out []*model.Connection
	for _, t := range tenants {
		cs, err := e.store.ListConnections(ctx, t.ID)
		if err != nil {
			continue
		}
		out = append(out, cs...)
	}
	return out, nil
}

// Resolve returns cache settings for a connection + db.coll.
// Always Enabled=true: any collection accessed via the "_cache" suffix is cached.
func (e *Engine) Resolve(connectionID, db, coll string) Decision {
	e.mu.RLock()
	p := e.policies[connectionID]
	e.mu.RUnlock()

	ttlSec := DefaultTTLSeconds
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

// Lookup is a legacy helper; prefer Resolve(connectionID, ...).
func (e *Engine) Lookup(connectionID, db, coll string) Decision {
	return e.Resolve(connectionID, db, coll)
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

// SetForTest injects a policy without the store (unit tests). Keyed by connectionID.
func (e *Engine) SetForTest(connectionID string, p *model.CachePolicy) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.policies == nil {
		e.policies = make(map[string]*model.CachePolicy)
	}
	if p != nil && p.ConnectionID == "" {
		p.ConnectionID = connectionID
	}
	e.policies[connectionID] = p
}
