package auth

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/taeven/nance/accelerator/internal/controlplane/store"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrAuthFailed     = errors.New("authentication failed")
	ErrTenantInactive = errors.New("tenant is not active")
	ErrNoConnection   = errors.New("connection not configured for token")
)

// defaultAuthCacheTTL caches successful (tenant, raw-token) → context.
// On hit we re-check GetTokenByID for revoke/expiry (cheap indexed read, no bcrypt).
const defaultAuthCacheTTL = 60 * time.Second

// Validator resolves wire credentials (tenant id + raw token) against Postgres.
type Validator struct {
	store store.Store

	cacheTTL time.Duration
	mu       sync.RWMutex
	// positive cache: key = tenantID + "\x00" + lookupHash
	cache map[string]authCacheEntry
}

type authCacheEntry struct {
	tc  TenantContext
	exp time.Time
}

func NewValidator(s store.Store) *Validator {
	return &Validator{
		store:    s,
		cacheTTL: defaultAuthCacheTTL,
		cache:    make(map[string]authCacheEntry),
	}
}

// WithAuthCacheTTL sets how long successful auths are cached (0 disables).
func (v *Validator) WithAuthCacheTTL(d time.Duration) *Validator {
	if d < 0 {
		d = 0
	}
	v.cacheTTL = d
	return v
}

// InvalidateAuthCache drops all cached successful auths (tests / hot reload).
func (v *Validator) InvalidateAuthCache() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.cache = make(map[string]authCacheEntry)
}

// InvalidateToken removes cache entries for a token id.
func (v *Validator) InvalidateToken(tokenID string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	for k, e := range v.cache {
		if e.tc.TokenID == tokenID {
			delete(v.cache, k)
		}
	}
}

// TenantContext is attached after successful auth. ConnectionID selects the source Mongo URI.
type TenantContext struct {
	TenantID     string
	ConnectionID string
	TokenID      string
}

// Authenticate checks username (tenant id) + password (raw API token bound to one connection).
//
// Fast path:
//  1. in-memory cache (revalidated via GetTokenByID for revoke)
//  2. O(1) SHA-256 lookup_hash index
//
// Slow path (legacy tokens without lookup_hash): scan tenant tokens and bcrypt.
func (v *Validator) Authenticate(ctx context.Context, username, password string) (*TenantContext, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, ErrAuthFailed
	}

	lookup := store.ProxyTokenLookupHash(password)
	cacheKey := username + "\x00" + lookup

	if v.cacheTTL > 0 {
		if tc, ok := v.cacheGet(cacheKey); ok {
			if err := v.revalidateToken(ctx, &tc); err == nil {
				return &tc, nil
			}
			// stale (revoked/expired/missing) — drop and re-auth
			v.cacheDelete(cacheKey)
		}
	}

	tenant, err := v.store.GetTenant(ctx, username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrAuthFailed
		}
		return nil, err
	}
	if tenant.Status != "" && tenant.Status != "active" {
		return nil, ErrTenantInactive
	}

	// Fast store path: indexed lookup_hash (new tokens).
	if row, err := v.store.GetActiveTokenByLookup(ctx, username, lookup); err == nil && row != nil {
		tc, err := v.tenantContextFromRow(ctx, username, row)
		if err != nil {
			return nil, err
		}
		v.cachePut(cacheKey, *tc)
		return tc, nil
	} else if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}

	// Legacy path: tokens issued before lookup_hash migration.
	rows, err := v.store.ListActiveTokenHashes(ctx, username)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		// Skip rows that already have lookup_hash (would have matched above).
		if row.LookupHash != "" {
			continue
		}
		if bcrypt.CompareHashAndPassword([]byte(row.TokenHash), []byte(password)) != nil {
			continue
		}
		tc, err := v.tenantContextFromRow(ctx, username, &row)
		if err != nil {
			return nil, err
		}
		v.cachePut(cacheKey, *tc)
		return tc, nil
	}
	return nil, ErrAuthFailed
}

func (v *Validator) tenantContextFromRow(ctx context.Context, tenantID string, row *store.TokenHashRow) (*TenantContext, error) {
	if row.ConnectionID == "" {
		return nil, ErrNoConnection
	}
	if _, err := v.store.GetConnection(ctx, row.ConnectionID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNoConnection
		}
		return nil, err
	}
	return &TenantContext{
		TenantID:     tenantID,
		ConnectionID: row.ConnectionID,
		TokenID:      row.ID,
	}, nil
}

// revalidateToken ensures a cached token is still active (indexed PK read, no bcrypt).
func (v *Validator) revalidateToken(ctx context.Context, tc *TenantContext) error {
	tok, err := v.store.GetTokenByID(ctx, tc.TokenID)
	if err != nil {
		return err
	}
	if tok.TenantID != tc.TenantID {
		return ErrAuthFailed
	}
	if tok.RevokedAt != nil {
		return ErrAuthFailed
	}
	if tok.ExpiresAt != nil && tok.ExpiresAt.Before(time.Now().UTC()) {
		return ErrAuthFailed
	}
	if tok.ConnectionID == "" {
		return ErrNoConnection
	}
	// Keep connection id fresh if rotated (rare).
	tc.ConnectionID = tok.ConnectionID
	return nil
}

func (v *Validator) cacheGet(key string) (TenantContext, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	e, ok := v.cache[key]
	if !ok || time.Now().After(e.exp) {
		return TenantContext{}, false
	}
	return e.tc, true
}

func (v *Validator) cachePut(key string, tc TenantContext) {
	if v.cacheTTL <= 0 {
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	// opportunistic prune when map grows
	if len(v.cache) > 10_000 {
		now := time.Now()
		for k, e := range v.cache {
			if now.After(e.exp) {
				delete(v.cache, k)
			}
		}
	}
	v.cache[key] = authCacheEntry{tc: tc, exp: time.Now().Add(v.cacheTTL)}
}

func (v *Validator) cacheDelete(key string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.cache, key)
}

// ParsePLAINPayload parses SASL PLAIN message: [authzid]\0authcid\0passwd
// MongoDB clients typically send \0<username>\0<password>.
func ParsePLAINPayload(payload []byte) (username, password string, err error) {
	parts := bytes.Split(payload, []byte{0})
	switch len(parts) {
	case 2:
		return string(parts[0]), string(parts[1]), nil
	case 3:
		return string(parts[1]), string(parts[2]), nil
	default:
		if len(parts) == 4 && len(parts[3]) == 0 {
			return string(parts[1]), string(parts[2]), nil
		}
		return "", "", ErrAuthFailed
	}
}

// VerifyTokenHash is a small helper for tests / store implementations.
func VerifyTokenHash(raw, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(raw)) == nil
}

// TokenRow is used internally by store lookup.
type TokenRow struct {
	ID        string
	TenantID  string
	TokenHash string
	ExpiresAt *time.Time
	RevokedAt *time.Time
}
