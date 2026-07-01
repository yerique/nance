package store

import (
	"context"
	"testing"
	"time"

	"github.com/taeven/nance/accelerator/internal/model"
)

func TestMemoryStore_TenantUserMemberRoundTrip(t *testing.T) {
	m := NewMemoryStore()
	ctx := context.Background()
	now := time.Now().UTC()
	if err := m.CreateTenant(ctx, &model.Tenant{ID: "t", Name: "T", Status: "active", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	u, err := m.UpsertUserByEmail(ctx, "A@B.C", "Ada")
	if err != nil || u.Email != "a@b.c" {
		t.Fatal(err)
	}
	if err := m.AddMember(ctx, "t", u.ID, model.RoleOwner); err != nil {
		t.Fatal(err)
	}
	mem, err := m.GetMember(ctx, "t", u.ID)
	if err != nil || mem.Role != model.RoleOwner {
		t.Fatal(err)
	}
	orgs, err := m.ListOrganizationsForUser(ctx, u.ID)
	if err != nil || len(orgs) != 1 {
		t.Fatal(err)
	}
	if err := m.DeleteTenant(ctx, "t"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.GetTenant(ctx, "t"); err != ErrNotFound {
		t.Fatal(err)
	}
}
