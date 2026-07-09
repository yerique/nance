package auth

import (
	"context"
	"testing"
	"time"

	"github.com/taeven/nance/accelerator/internal/controlplane/store"
	"github.com/taeven/nance/accelerator/internal/model"
	"golang.org/x/crypto/bcrypt"
)

func TestParsePLAINPayload(t *testing.T) {
	p := []byte{0, 'u', 's', 'e', 'r', 0, 'p', 'a', 's', 's'}
	u, pw, err := ParsePLAINPayload(p)
	if err != nil || u != "user" || pw != "pass" {
		t.Fatalf("got %q %q err=%v", u, pw, err)
	}

	p2 := []byte{'d', 'e', 'm', 'o', 0, 't', 'o', 'k'}
	u, pw, err = ParsePLAINPayload(p2)
	if err != nil || u != "demo" || pw != "tok" {
		t.Fatalf("got %q %q err=%v", u, pw, err)
	}
}

func seedTenantConn(t *testing.T, ms *store.MemoryStore, tenantID, connID string) {
	t.Helper()
	ctx := context.Background()
	if err := ms.CreateTenant(ctx, &model.Tenant{ID: tenantID, Name: "T", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if err := ms.CreateConnection(ctx, &model.Connection{
		ID: connID, TenantID: tenantID, Name: "prod",
		URICiphertext: []byte("ct"), Nonce: []byte("n"), DEKVersion: "v1",
	}); err != nil {
		t.Fatal(err)
	}
}

func TestAuthenticate_LookupHashFastPath(t *testing.T) {
	ctx := context.Background()
	ms := store.NewMemoryStore()
	seedTenantConn(t, ms, "org1", "conn1")

	raw := "super-secret-token-value-xyz"
	bcryptHash, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	lookup := store.ProxyTokenLookupHash(raw)
	if err := ms.CreateToken(ctx, &model.Token{
		ID: "tok_fast", TenantID: "org1", ConnectionID: "conn1", CreatedAt: time.Now().UTC(),
	}, string(bcryptHash), lookup); err != nil {
		t.Fatal(err)
	}

	v := NewValidator(ms).WithAuthCacheTTL(0) // disable cache for pure lookup test
	tc, err := v.Authenticate(ctx, "org1", raw)
	if err != nil {
		t.Fatal(err)
	}
	if tc.TokenID != "tok_fast" || tc.ConnectionID != "conn1" {
		t.Fatalf("%+v", tc)
	}
	if _, err := v.Authenticate(ctx, "org1", "wrong"); err != ErrAuthFailed {
		t.Fatalf("want auth failed, got %v", err)
	}
}

func TestAuthenticate_LegacyBcryptFallback(t *testing.T) {
	ctx := context.Background()
	ms := store.NewMemoryStore()
	seedTenantConn(t, ms, "org1", "conn1")

	raw := "legacy-token"
	bcryptHash, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	// empty lookup_hash → legacy path
	if err := ms.CreateToken(ctx, &model.Token{
		ID: "tok_leg", TenantID: "org1", ConnectionID: "conn1", CreatedAt: time.Now().UTC(),
	}, string(bcryptHash), ""); err != nil {
		t.Fatal(err)
	}

	v := NewValidator(ms).WithAuthCacheTTL(0)
	tc, err := v.Authenticate(ctx, "org1", raw)
	if err != nil || tc.TokenID != "tok_leg" {
		t.Fatalf("%v %+v", err, tc)
	}
}

func TestAuthenticate_CacheAndRevokeRevalidate(t *testing.T) {
	ctx := context.Background()
	ms := store.NewMemoryStore()
	seedTenantConn(t, ms, "org1", "conn1")

	raw := "cached-token"
	bcryptHash, _ := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.MinCost)
	lookup := store.ProxyTokenLookupHash(raw)
	if err := ms.CreateToken(ctx, &model.Token{
		ID: "tok_c", TenantID: "org1", ConnectionID: "conn1", CreatedAt: time.Now().UTC(),
	}, string(bcryptHash), lookup); err != nil {
		t.Fatal(err)
	}

	v := NewValidator(ms).WithAuthCacheTTL(time.Minute)
	if _, err := v.Authenticate(ctx, "org1", raw); err != nil {
		t.Fatal(err)
	}
	// second call should hit cache + revalidate
	if _, err := v.Authenticate(ctx, "org1", raw); err != nil {
		t.Fatal(err)
	}

	if err := ms.RevokeToken(ctx, "tok_c"); err != nil {
		t.Fatal(err)
	}
	// cache hit must revalidate and fail
	if _, err := v.Authenticate(ctx, "org1", raw); err == nil {
		t.Fatal("expected fail after revoke")
	}
}

func TestProxyTokenLookupHashStable(t *testing.T) {
	a := store.ProxyTokenLookupHash("abc")
	b := store.ProxyTokenLookupHash("abc")
	if a != b || len(a) != 64 {
		t.Fatalf("%q %q", a, b)
	}
}
