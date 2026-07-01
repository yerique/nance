package service

import (
	"context"
	"testing"

	"github.com/taeven/nance/accelerator/internal/controlplane/store"
	"github.com/taeven/nance/accelerator/internal/model"
)

func TestTenantService_CRUD(t *testing.T) {
	ms := store.NewMemoryStore()
	svc := NewTenantService(ms)
	ctx := context.Background()
	if _, err := svc.Create(ctx, "", "x"); err == nil {
		t.Fatal("empty id")
	}
	ten, err := svc.Create(ctx, "t1", "Tenant")
	if err != nil || ten.ID != "t1" {
		t.Fatal(err)
	}
	got, err := svc.Get(ctx, "t1")
	if err != nil || got.Name != "Tenant" {
		t.Fatal(err)
	}
	if _, err := svc.Get(ctx, "missing"); err != ErrTenantNotFound {
		t.Fatalf("got %v", err)
	}
	list, err := svc.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("%d %v", len(list), err)
	}
}

func TestPolicyService_DefaultsAndCollection(t *testing.T) {
	ms := store.NewMemoryStore()
	svc := NewPolicyService(ms)
	ctx := context.Background()
	if err := svc.SetDefaults(ctx, "t1", 30); err != nil {
		t.Fatal(err)
	}
	p, err := svc.Get(ctx, "t1")
	if err != nil || p.DefaultTtlSeconds != 30 {
		t.Fatalf("%+v %v", p, err)
	}
	pol := model.CollectionPolicy{Enabled: true, TTLSeconds: 10}
	if err := svc.SetCollectionPolicy(ctx, "t1", "db.c", pol); err != nil {
		t.Fatal(err)
	}
	p2, _ := svc.Get(ctx, "t1")
	if p2.Collections["db.c"].TTLSeconds != 10 {
		t.Fatalf("%+v", p2.Collections)
	}
	if err := svc.Invalidate(ctx, "t1", "db", "c", nil); err != nil {
		t.Fatal(err)
	}
}

func TestTokenService_IssueListRevoke(t *testing.T) {
	ms := store.NewMemoryStore()
	svc := NewTokenService(ms)
	ctx := context.Background()
	raw, tok, err := svc.Issue(ctx, "t1", "ci")
	if err != nil || raw == "" || tok.ID == "" {
		t.Fatal(err)
	}
	list, err := svc.List(ctx, "t1")
	if err != nil || len(list) != 1 {
		t.Fatal(err)
	}
	if err := svc.Revoke(ctx, tok.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := ms.GetTokenByID(ctx, tok.ID)
	if got.RevokedAt == nil {
		t.Fatal("expected revoked")
	}
}
