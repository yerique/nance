package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/taeven/nance/accelerator/internal/controlplane/store"
	"github.com/taeven/nance/accelerator/internal/crypto"
	"github.com/taeven/nance/accelerator/internal/model"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrTenantNotFound     = errors.New("tenant not found")
	ErrBackendNotFound    = errors.New("backend not configured for tenant")
	ErrInvalidToken       = errors.New("invalid token")
	ErrPolicyNotFound     = errors.New("policy not found")
)

// TenantService handles tenant lifecycle.
type TenantService struct {
	store store.Store
}

func NewTenantService(s store.Store) *TenantService {
	return &TenantService{store: s}
}

func (s *TenantService) Create(ctx context.Context, id, name string) (*model.Tenant, error) {
	if id == "" || name == "" {
		return nil, errors.New("id and name are required")
	}
	t := &model.Tenant{
		ID:        id,
		Name:      name,
		Status:    "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.store.CreateTenant(ctx, t); err != nil {
		return nil, err
	}
	_ = s.store.RecordAudit(ctx, id, "system", "create_tenant", map[string]string{"name": name})
	return t, nil
}

func (s *TenantService) Get(ctx context.Context, id string) (*model.Tenant, error) {
	t, err := s.store.GetTenant(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrTenantNotFound
		}
		return nil, err
	}
	return t, nil
}

func (s *TenantService) List(ctx context.Context) ([]*model.Tenant, error) {
	return s.store.ListTenants(ctx)
}

// BackendService manages encrypted connection strings + validation.
type BackendService struct {
	store  store.Store
	crypto *crypto.Config
}

func NewBackendService(s store.Store, c *crypto.Config) *BackendService {
	return &BackendService{store: s, crypto: c}
}

// SetBackend encrypts and stores the real MongoDB URI for a tenant.
func (s *BackendService) SetBackend(ctx context.Context, tenantID, plaintextURI string) error {
	if plaintextURI == "" {
		return errors.New("uri is required")
	}
	ct, nonce, dekVer, err := s.crypto.Encrypt([]byte(plaintextURI), tenantID)
	if err != nil {
		return fmt.Errorf("encryption failed: %w", err)
	}
	if err := s.store.SetBackend(ctx, tenantID, ct, nonce, dekVer); err != nil {
		return err
	}
	_ = s.store.RecordAudit(ctx, tenantID, "system", "set_backend", nil)
	return nil
}

// TestConnection decrypts the URI (temporarily), connects to the real Mongo, runs ping + listCollections, then disconnects.
// Never logs or returns the plaintext URI.
func (s *BackendService) TestConnection(ctx context.Context, tenantID string) error {
	be, err := s.store.GetBackend(ctx, tenantID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrBackendNotFound
		}
		return err
	}

	plaintext, err := s.crypto.Decrypt(be.URICiphertext, be.Nonce, tenantID)
	if err != nil {
		return fmt.Errorf("failed to decrypt backend uri: %w", err)
	}
	uri := string(plaintext)

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return fmt.Errorf("failed to connect to real MongoDB: %w", err)
	}
	defer client.Disconnect(ctx)

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	// Light validation: list collections on any DB to prove auth/permissions work
	// We don't care which DB; just that the connection is healthy.
	dbs, err := client.ListDatabases(ctx, nil)
	if err != nil {
		return fmt.Errorf("listDatabases failed: %w", err)
	}
	_ = dbs // we don't return DB list for security

	if err := s.store.UpdateBackendValidated(ctx, tenantID); err != nil {
		// Non-fatal
	}

	_ = s.store.RecordAudit(ctx, tenantID, "system", "test_backend_connection", map[string]any{"success": true})
	return nil
}

// PolicyService manages cache policies.
type PolicyService struct {
	store store.Store
}

func NewPolicyService(s store.Store) *PolicyService {
	return &PolicyService{store: s}
}

func (s *PolicyService) Get(ctx context.Context, tenantID string) (*model.CachePolicy, error) {
	p, err := s.store.GetCachePolicy(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (s *PolicyService) SetCollectionPolicy(ctx context.Context, tenantID, dbColl string, pol model.CollectionPolicy) error {
	current, err := s.store.GetCachePolicy(ctx, tenantID)
	if err != nil {
		return err
	}
	if current.Collections == nil {
		current.Collections = make(map[string]model.CollectionPolicy)
	}
	current.Collections[dbColl] = pol
	current.TenantID = tenantID
	if err := s.store.UpsertCachePolicy(ctx, current); err != nil {
		return err
	}
	_ = s.store.RecordAudit(ctx, tenantID, "system", "update_collection_policy", map[string]string{"collection": dbColl})
	return nil
}

func (s *PolicyService) SetDefaults(ctx context.Context, tenantID string, defaultTTL int) error {
	current, err := s.store.GetCachePolicy(ctx, tenantID)
	if err != nil {
		return err
	}
	current.DefaultTtlSeconds = defaultTTL
	current.TenantID = tenantID
	if err := s.store.UpsertCachePolicy(ctx, current); err != nil {
		return err
	}
	_ = s.store.RecordAudit(ctx, tenantID, "system", "update_default_ttl", map[string]int{"ttl": defaultTTL})
	return nil
}

// TokenService issues and manages data-plane tokens.
type TokenService struct {
	store store.Store
}

func NewTokenService(s store.Store) *TokenService {
	return &TokenService{store: s}
}

// Issue creates a new token, returns the **raw secret only once**.
func (s *TokenService) Issue(ctx context.Context, tenantID, description string) (rawToken string, tok *model.Token, err error) {
	// Generate 32 bytes of entropy
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, err
	}
	rawToken = base64.RawURLEncoding.EncodeToString(buf)

	// Hash for storage (bcrypt is fine for control plane tokens)
	hash, err := bcrypt.GenerateFromPassword([]byte(rawToken), bcrypt.DefaultCost)
	if err != nil {
		return "", nil, err
	}

	now := time.Now().UTC()
	tok = &model.Token{
		ID:          "tok_" + base64.RawURLEncoding.EncodeToString(buf[:12]), // short opaque id
		TenantID:    tenantID,
		Description: description,
		CreatedAt:   now,
	}

	if err := s.store.CreateToken(ctx, tok, string(hash)); err != nil {
		return "", nil, err
	}
	_ = s.store.RecordAudit(ctx, tenantID, "system", "issue_token", map[string]string{"token_id": tok.ID})
	return rawToken, tok, nil
}

func (s *TokenService) List(ctx context.Context, tenantID string) ([]*model.Token, error) {
	return s.store.ListTokensForTenant(ctx, tenantID)
}

func (s *TokenService) Revoke(ctx context.Context, tokenID string) error {
	return s.store.RevokeToken(ctx, tokenID)
}

// Note: actual token validation (for proxy use) will live in Phase 1.
// For Phase 0 we only need issuance + listing/revocation from control plane.
