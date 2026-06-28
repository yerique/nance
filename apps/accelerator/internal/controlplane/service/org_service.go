package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/taeven/nance/accelerator/internal/controlplane/store"
	"github.com/taeven/nance/accelerator/internal/model"
)

// OrgService manages organizations (tenants), membership, and invites for dashboard users.
type OrgService struct {
	store  store.Store
	mailer Mailer
}

func NewOrgService(s store.Store, mailer Mailer) *OrgService {
	return &OrgService{store: s, mailer: mailer}
}

var slugRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9_-]{1,62}[a-z0-9])?$`)

// CreateOrganization creates a tenant and adds the user as owner.
func (s *OrgService) CreateOrganization(ctx context.Context, userID, id, name string) (*model.OrganizationSummary, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("name is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		id = slugify(name)
	}
	id = strings.ToLower(id)
	if !slugRe.MatchString(id) {
		return nil, errors.New("id must be 3-64 chars: lowercase letters, digits, _ or -")
	}
	if _, err := s.store.GetTenant(ctx, id); err == nil {
		return nil, errors.New("organization id already exists")
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	now := time.Now().UTC()
	t := &model.Tenant{ID: id, Name: name, Status: "active", CreatedAt: now, UpdatedAt: now}
	if err := s.store.CreateTenant(ctx, t); err != nil {
		return nil, err
	}
	if err := s.store.AddMember(ctx, id, userID, model.RoleOwner); err != nil {
		return nil, err
	}
	_ = s.store.RecordAudit(ctx, id, userID, "create_organization", map[string]string{"name": name})
	return &model.OrganizationSummary{Tenant: *t, Role: model.RoleOwner}, nil
}

func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
		} else if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "org-" + cryptoRandHex(4)
	}
	if len(out) > 48 {
		out = out[:48]
	}
	// ensure uniqueness-ish
	return out + "-" + cryptoRandHex(3)
}

// ListOrganizations returns orgs the user belongs to.
func (s *OrgService) ListOrganizations(ctx context.Context, userID string) ([]*model.OrganizationSummary, error) {
	return s.store.ListOrganizationsForUser(ctx, userID)
}

// RequireMember returns membership or ErrForbidden / ErrNotMember.
func (s *OrgService) RequireMember(ctx context.Context, tenantID, userID string) (*model.OrganizationMember, error) {
	m, err := s.store.GetMember(ctx, tenantID, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrForbidden
		}
		return nil, err
	}
	return m, nil
}

// RequireAdmin requires owner or admin role.
func (s *OrgService) RequireAdmin(ctx context.Context, tenantID, userID string) (*model.OrganizationMember, error) {
	m, err := s.RequireMember(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	if m.Role != model.RoleOwner && m.Role != model.RoleAdmin {
		return nil, ErrForbidden
	}
	return m, nil
}

// ListMembers lists members of an org (caller must be a member).
func (s *OrgService) ListMembers(ctx context.Context, tenantID string) ([]*model.OrganizationMember, error) {
	return s.store.ListMembers(ctx, tenantID)
}

// ListPendingInvitesForTenant lists outstanding invites for an org.
func (s *OrgService) ListPendingInvitesForTenant(ctx context.Context, tenantID string) ([]*model.OrganizationInvite, error) {
	return s.store.ListPendingInvitesForTenant(ctx, tenantID)
}

// ListPendingInvitesForUser lists invites addressed to the user's email.
func (s *OrgService) ListPendingInvitesForUser(ctx context.Context, user *model.User) ([]*model.OrganizationInvite, error) {
	return s.store.ListPendingInvitesForEmail(ctx, user.Email)
}

// InviteMember creates an invite and emails the invitee.
func (s *OrgService) InviteMember(ctx context.Context, tenantID, inviterID, email string, role model.MemberRole) (*model.OrganizationInvite, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return nil, err
	}
	if role == "" {
		role = model.RoleMember
	}
	if role != model.RoleMember && role != model.RoleAdmin && role != model.RoleOwner {
		return nil, errors.New("invalid role")
	}
	// If user already member, reject
	if u, err := s.store.GetUserByEmail(ctx, email); err == nil {
		if _, merr := s.store.GetMember(ctx, tenantID, u.ID); merr == nil {
			return nil, ErrAlreadyMember
		}
	}

	raw, err := randomToken(24)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	inv := &model.OrganizationInvite{
		ID:        "inv_" + cryptoRandHex(12),
		TenantID:  tenantID,
		Email:     email,
		Role:      role,
		InvitedBy: inviterID,
		ExpiresAt: now.Add(7 * 24 * time.Hour),
		CreatedAt: now,
		RawToken:  raw,
	}
	if t, err := s.store.GetTenant(ctx, tenantID); err == nil {
		inv.TenantName = t.Name
	}
	// Replace any prior pending invite for same email (delete then insert)
	if existing, err := s.store.ListPendingInvitesForTenant(ctx, tenantID); err == nil {
		for _, e := range existing {
			if strings.EqualFold(e.Email, email) {
				_ = s.store.DeleteInvite(ctx, e.ID)
			}
		}
	}
	if err := s.store.CreateInvite(ctx, inv, hashToken(raw)); err != nil {
		return nil, err
	}
	body := fmt.Sprintf(
		"You have been invited to join organization %q on Nance.\n\nSign in with this email and accept the invite from your organizations page.\nInvite id: %s\n",
		inv.TenantName, inv.ID,
	)
	if s.mailer != nil {
		_ = s.mailer.Send(ctx, email, "Nance organization invite", body)
	}
	_ = s.store.RecordAudit(ctx, tenantID, inviterID, "invite_member", map[string]string{"email": email, "role": string(role)})
	return inv, nil
}

// AcceptInvite accepts a pending invite for the logged-in user (email must match).
func (s *OrgService) AcceptInvite(ctx context.Context, user *model.User, inviteID string) (*model.OrganizationSummary, error) {
	inv, err := s.store.GetInviteByID(ctx, inviteID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrInviteNotFound
		}
		return nil, err
	}
	if inv.AcceptedAt != nil {
		return nil, ErrInviteNotFound
	}
	if time.Now().UTC().After(inv.ExpiresAt) {
		return nil, ErrInviteExpired
	}
	if !strings.EqualFold(inv.Email, user.Email) {
		return nil, ErrForbidden
	}
	if err := s.store.AddMember(ctx, inv.TenantID, user.ID, inv.Role); err != nil {
		return nil, err
	}
	_ = s.store.MarkInviteAccepted(ctx, inv.ID)
	t, err := s.store.GetTenant(ctx, inv.TenantID)
	if err != nil {
		return nil, err
	}
	_ = s.store.RecordAudit(ctx, inv.TenantID, user.ID, "accept_invite", map[string]string{"inviteId": inv.ID})
	return &model.OrganizationSummary{Tenant: *t, Role: inv.Role}, nil
}

// RemoveMember removes a member; cannot remove last owner.
func (s *OrgService) RemoveMember(ctx context.Context, tenantID, actorID, targetUserID string) error {
	target, err := s.store.GetMember(ctx, tenantID, targetUserID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrNotMember
		}
		return err
	}
	if target.Role == model.RoleOwner {
		members, err := s.store.ListMembers(ctx, tenantID)
		if err != nil {
			return err
		}
		owners := 0
		for _, m := range members {
			if m.Role == model.RoleOwner {
				owners++
			}
		}
		if owners <= 1 {
			return ErrLastOwner
		}
	}
	if err := s.store.RemoveMember(ctx, tenantID, targetUserID); err != nil {
		return err
	}
	_ = s.store.RecordAudit(ctx, tenantID, actorID, "remove_member", map[string]string{"userId": targetUserID})
	return nil
}

// RevokeInvite deletes a pending invite.
func (s *OrgService) RevokeInvite(ctx context.Context, tenantID, actorID, inviteID string) error {
	inv, err := s.store.GetInviteByID(ctx, inviteID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrInviteNotFound
		}
		return err
	}
	if inv.TenantID != tenantID {
		return ErrInviteNotFound
	}
	if err := s.store.DeleteInvite(ctx, inviteID); err != nil {
		return err
	}
	_ = s.store.RecordAudit(ctx, tenantID, actorID, "revoke_invite", map[string]string{"inviteId": inviteID})
	return nil
}
