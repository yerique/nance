package service

import (
	"context"
	"testing"
	"time"

	"github.com/taeven/nance/accelerator/internal/controlplane/store"
	"github.com/taeven/nance/accelerator/internal/model"
	"golang.org/x/crypto/bcrypt"
)

func TestOrgService_CreateAndInviteOnly(t *testing.T) {
	ms := store.NewMemoryStore()
	svc := NewOrgService(ms, &captureMailer{}).WithInviteOnly(true)
	ctx := context.Background()
	u, _ := ms.UpsertUserByEmail(ctx, "o@ex.com", "Owner")

	if _, err := svc.CreateOrganization(ctx, u.ID, "acme", "Acme"); err != ErrOrgCreationDisabled {
		t.Fatalf("want disabled, got %v", err)
	}

	svc2 := NewOrgService(ms, &captureMailer{})
	org, err := svc2.CreateOrganization(ctx, u.ID, "acme", "Acme")
	if err != nil || org.Role != model.RoleOwner {
		t.Fatalf("%+v %v", org, err)
	}
	if _, err := svc2.CreateOrganization(ctx, u.ID, "acme", "Dup"); err == nil {
		t.Fatal("duplicate id should fail")
	}
}

func TestOrgService_RolesAndDelete(t *testing.T) {
	ms := store.NewMemoryStore()
	mail := &captureMailer{}
	svc := NewOrgService(ms, mail)
	ctx := context.Background()

	owner, _ := ms.UpsertUserByEmail(ctx, "owner@ex.com", "O")
	admin, _ := ms.UpsertUserByEmail(ctx, "admin@ex.com", "A")
	member, _ := ms.UpsertUserByEmail(ctx, "mem@ex.com", "M")
	org, _ := svc.CreateOrganization(ctx, owner.ID, "org1", "Org One")

	// invite admin as admin role by owner
	inv, err := svc.InviteMember(ctx, org.ID, owner.ID, "admin@ex.com", model.RoleAdmin, model.RoleOwner)
	if err != nil || inv == nil {
		t.Fatal(err)
	}
	if _, err := svc.AcceptInvite(ctx, admin, inv.ID); err != nil {
		t.Fatal(err)
	}
	// admin cannot invite owner
	if _, err := svc.InviteMember(ctx, org.ID, admin.ID, "x@ex.com", model.RoleOwner, model.RoleAdmin); err == nil {
		t.Fatal("admin should not invite owner")
	}
	// member invite fails for inviter role member
	if _, err := svc.InviteMember(ctx, org.ID, member.ID, "y@ex.com", model.RoleMember, model.RoleMember); err != ErrForbidden {
		t.Fatalf("got %v", err)
	}

	// add member directly
	_ = ms.AddMember(ctx, org.ID, member.ID, model.RoleMember)
	if _, err := svc.RequireAdmin(ctx, org.ID, member.ID); err != ErrForbidden {
		t.Fatal("member is not admin")
	}
	if _, err := svc.RequireOwner(ctx, org.ID, admin.ID); err != ErrForbidden {
		t.Fatal("admin is not owner")
	}

	// delete org: request + confirm
	if err := svc.RequestDeleteOrganization(ctx, org.ID, admin); err != ErrForbidden {
		t.Fatalf("admin delete request: %v", err)
	}
	if err := svc.RequestDeleteOrganization(ctx, org.ID, owner); err != nil {
		t.Fatal(err)
	}
	if mail.n < 1 {
		t.Fatal("expected delete email")
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("654321"), bcrypt.MinCost)
	key := orgDeleteCodeKey(org.ID, owner.Email)
	_ = ms.SetEmailVerificationCode(ctx, key, string(hash), time.Now().UTC().Add(time.Minute))
	if err := svc.ConfirmDeleteOrganization(ctx, org.ID, owner, "654321"); err != nil {
		t.Fatal(err)
	}
	if _, err := ms.GetTenant(ctx, org.ID); err != store.ErrNotFound {
		t.Fatal("tenant should be gone")
	}
}

func TestOrgService_RemoveLastOwner(t *testing.T) {
	ms := store.NewMemoryStore()
	svc := NewOrgService(ms, nil)
	ctx := context.Background()
	owner, _ := ms.UpsertUserByEmail(ctx, "o@ex.com", "")
	org, _ := svc.CreateOrganization(ctx, owner.ID, "solo", "Solo")
	if err := svc.RemoveMember(ctx, org.ID, owner.ID, owner.ID, model.RoleOwner); err != ErrLastOwner {
		t.Fatalf("got %v", err)
	}
}

func TestCanManageSettings(t *testing.T) {
	if !CanManageSettings(model.RoleOwner) || !CanManageSettings(model.RoleAdmin) || CanManageSettings(model.RoleMember) {
		t.Fatal("role matrix wrong")
	}
}
