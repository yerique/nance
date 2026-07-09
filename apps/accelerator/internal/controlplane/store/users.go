package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/taeven/nance/accelerator/internal/model"
)

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func newID(prefix string) string {
	return prefix + "_" + cryptoRandHex(12)
}

func cryptoRandHex(nBytes int) string {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		// extremely unlikely; fall back to time-based
		return hex.EncodeToString([]byte(time.Now().Format("150405.000000000")))[:nBytes*2]
	}
	return hex.EncodeToString(b)
}

// ===== Users =====

func (s *PostgresStore) UpsertUserByEmail(ctx context.Context, email, name string) (*model.User, error) {
	email = normalizeEmail(email)
	if email == "" {
		return nil, errors.New("email required")
	}
	existing, err := s.GetUserByEmail(ctx, email)
	if err == nil {
		if name != "" && existing.Name == "" {
			_ = s.UpdateUserName(ctx, existing.ID, name)
			existing.Name = name
		}
		return existing, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	id := newID("usr")
	now := time.Now().UTC()
	u := &model.User{ID: id, Email: email, Name: name, CreatedAt: now, UpdatedAt: now}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO users (id, email, name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`, u.ID, u.Email, u.Name, u.CreatedAt, u.UpdatedAt)
	if err != nil {
		return s.GetUserByEmail(ctx, email)
	}
	return u, nil
}

func (s *PostgresStore) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, email, name, created_at, updated_at,
		       (password_hash IS NOT NULL AND password_hash <> '')
		FROM users WHERE id = $1
	`, id)
	var u model.User
	if err := row.Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt, &u.UpdatedAt, &u.HasPassword); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (s *PostgresStore) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	email = normalizeEmail(email)
	row := s.pool.QueryRow(ctx, `
		SELECT id, email, name, created_at, updated_at,
		       (password_hash IS NOT NULL AND password_hash <> '')
		FROM users WHERE LOWER(email) = $1
	`, email)
	var u model.User
	if err := row.Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt, &u.UpdatedAt, &u.HasPassword); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (s *PostgresStore) UpdateUserName(ctx context.Context, id, name string) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET name = $2, updated_at = NOW() WHERE id = $1`, id, name)
	return err
}

func (s *PostgresStore) SetUserPasswordHash(ctx context.Context, userID, passwordHash string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE users SET password_hash = $2, updated_at = NOW() WHERE id = $1
	`, userID, passwordHash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) GetUserPasswordHash(ctx context.Context, userID string) (string, error) {
	row := s.pool.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, userID)
	var hash *string
	if err := row.Scan(&hash); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	if hash == nil || *hash == "" {
		return "", ErrNotFound
	}
	return *hash, nil
}

func (s *PostgresStore) ClearUserPassword(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET password_hash = NULL, updated_at = NOW() WHERE id = $1`, userID)
	return err
}

func (s *PostgresStore) CreatePasswordResetToken(ctx context.Context, id, userID, tokenHash string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO password_reset_tokens (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, NOW())
	`, id, userID, tokenHash, expiresAt.UTC())
	return err
}

func (s *PostgresStore) ConsumePasswordResetToken(ctx context.Context, tokenHash string) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
		SELECT id, user_id, expires_at, used_at
		FROM password_reset_tokens
		WHERE token_hash = $1
		FOR UPDATE
	`, tokenHash)
	var id, userID string
	var expires time.Time
	var usedAt *time.Time
	if err := row.Scan(&id, &userID, &expires, &usedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	if usedAt != nil || time.Now().UTC().After(expires) {
		return "", ErrNotFound
	}
	if _, err := tx.Exec(ctx, `UPDATE password_reset_tokens SET used_at = NOW() WHERE id = $1`, id); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return userID, nil
}

func (s *PostgresStore) SetEmailVerificationCode(ctx context.Context, email, codeHash string, expiresAt time.Time) error {
	email = normalizeEmail(email)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO email_verification_codes (email, code_hash, expires_at, attempts, created_at)
		VALUES ($1, $2, $3, 0, NOW())
		ON CONFLICT (email) DO UPDATE SET
			code_hash = EXCLUDED.code_hash,
			expires_at = EXCLUDED.expires_at,
			attempts = 0,
			created_at = NOW()
	`, email, codeHash, expiresAt.UTC())
	return err
}

func (s *PostgresStore) GetEmailVerificationCode(ctx context.Context, email string) (string, time.Time, int, error) {
	email = normalizeEmail(email)
	row := s.pool.QueryRow(ctx, `
		SELECT code_hash, expires_at, attempts FROM email_verification_codes WHERE email = $1
	`, email)
	var hash string
	var exp time.Time
	var attempts int
	if err := row.Scan(&hash, &exp, &attempts); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", time.Time{}, 0, ErrNotFound
		}
		return "", time.Time{}, 0, err
	}
	return hash, exp, attempts, nil
}

func (s *PostgresStore) IncrementEmailVerificationAttempts(ctx context.Context, email string) error {
	email = normalizeEmail(email)
	_, err := s.pool.Exec(ctx, `
		UPDATE email_verification_codes SET attempts = attempts + 1 WHERE email = $1
	`, email)
	return err
}

func (s *PostgresStore) ClearEmailVerificationCode(ctx context.Context, email string) error {
	email = normalizeEmail(email)
	_, err := s.pool.Exec(ctx, `DELETE FROM email_verification_codes WHERE email = $1`, email)
	return err
}

func (s *PostgresStore) CreateSession(ctx context.Context, id, userID, tokenHash string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO user_sessions (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, NOW())
	`, id, userID, tokenHash, expiresAt.UTC())
	return err
}

func (s *PostgresStore) GetSessionByTokenHash(ctx context.Context, tokenHash string) (string, string, time.Time, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, user_id, expires_at FROM user_sessions WHERE token_hash = $1
	`, tokenHash)
	var sid, uid string
	var exp time.Time
	if err := row.Scan(&sid, &uid, &exp); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", time.Time{}, ErrNotFound
		}
		return "", "", time.Time{}, err
	}
	return sid, uid, exp, nil
}

func (s *PostgresStore) DeleteSession(ctx context.Context, sessionID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM user_sessions WHERE id = $1`, sessionID)
	return err
}

func (s *PostgresStore) DeleteSessionByTokenHash(ctx context.Context, tokenHash string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM user_sessions WHERE token_hash = $1`, tokenHash)
	return err
}

// ===== Membership =====

func (s *PostgresStore) AddMember(ctx context.Context, tenantID, userID string, role model.MemberRole) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO organization_members (tenant_id, user_id, role, created_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (tenant_id, user_id) DO UPDATE SET role = EXCLUDED.role
	`, tenantID, userID, string(role))
	return err
}

func (s *PostgresStore) RemoveMember(ctx context.Context, tenantID, userID string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM organization_members WHERE tenant_id = $1 AND user_id = $2
	`, tenantID, userID)
	return err
}

func (s *PostgresStore) GetMember(ctx context.Context, tenantID, userID string) (*model.OrganizationMember, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT m.tenant_id, m.user_id, COALESCE(u.email, ''), COALESCE(u.name, ''), m.role, m.created_at
		FROM organization_members m
		LEFT JOIN users u ON u.id = m.user_id
		WHERE m.tenant_id = $1 AND m.user_id = $2
	`, tenantID, userID)
	var m model.OrganizationMember
	var role string
	if err := row.Scan(&m.TenantID, &m.UserID, &m.Email, &m.Name, &role, &m.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	m.Role = model.MemberRole(role)
	return &m, nil
}

func (s *PostgresStore) ListMembers(ctx context.Context, tenantID string) ([]*model.OrganizationMember, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT m.tenant_id, m.user_id, COALESCE(u.email, ''), COALESCE(u.name, ''), m.role, m.created_at
		FROM organization_members m
		LEFT JOIN users u ON u.id = m.user_id
		WHERE m.tenant_id = $1
		ORDER BY m.created_at ASC
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*model.OrganizationMember, 0)
	for rows.Next() {
		var m model.OrganizationMember
		var role string
		if err := rows.Scan(&m.TenantID, &m.UserID, &m.Email, &m.Name, &role, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.Role = model.MemberRole(role)
		out = append(out, &m)
	}
	return out, rows.Err()
}

func (s *PostgresStore) ListOrganizationsForUser(ctx context.Context, userID string) ([]*model.OrganizationSummary, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT t.id, t.name, t.status, t.created_at, t.updated_at, m.role
		FROM organization_members m
		JOIN tenants t ON t.id = m.tenant_id
		WHERE m.user_id = $1
		ORDER BY t.name ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*model.OrganizationSummary, 0)
	for rows.Next() {
		var o model.OrganizationSummary
		var role string
		if err := rows.Scan(&o.ID, &o.Name, &o.Status, &o.CreatedAt, &o.UpdatedAt, &role); err != nil {
			return nil, err
		}
		o.Role = model.MemberRole(role)
		out = append(out, &o)
	}
	return out, rows.Err()
}

func (s *PostgresStore) CreateInvite(ctx context.Context, inv *model.OrganizationInvite, tokenHash string) error {
	email := normalizeEmail(inv.Email)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO organization_invites (id, tenant_id, email, role, token_hash, invited_by, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, inv.ID, inv.TenantID, email, string(inv.Role), tokenHash, nullIfEmpty(inv.InvitedBy), inv.ExpiresAt.UTC(), inv.CreatedAt.UTC())
	return err
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

type scannable interface {
	Scan(dest ...any) error
}

func scanInvite(row scannable) (*model.OrganizationInvite, error) {
	var inv model.OrganizationInvite
	var role string
	var accepted *time.Time
	if err := row.Scan(&inv.ID, &inv.TenantID, &inv.TenantName, &inv.Email, &role, &inv.InvitedBy,
		&inv.ExpiresAt, &accepted, &inv.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	inv.Role = model.MemberRole(role)
	inv.AcceptedAt = accepted
	return &inv, nil
}

type rowser interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanInviteRows(rows rowser) ([]*model.OrganizationInvite, error) {
	out := make([]*model.OrganizationInvite, 0)
	for rows.Next() {
		var inv model.OrganizationInvite
		var role string
		var accepted *time.Time
		if err := rows.Scan(&inv.ID, &inv.TenantID, &inv.TenantName, &inv.Email, &role, &inv.InvitedBy,
			&inv.ExpiresAt, &accepted, &inv.CreatedAt); err != nil {
			return nil, err
		}
		inv.Role = model.MemberRole(role)
		inv.AcceptedAt = accepted
		out = append(out, &inv)
	}
	return out, rows.Err()
}

func (s *PostgresStore) GetInviteByID(ctx context.Context, id string) (*model.OrganizationInvite, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT i.id, i.tenant_id, COALESCE(t.name, ''), i.email, i.role, COALESCE(i.invited_by, ''),
		       i.expires_at, i.accepted_at, i.created_at
		FROM organization_invites i
		LEFT JOIN tenants t ON t.id = i.tenant_id
		WHERE i.id = $1
	`, id)
	return scanInvite(row)
}

func (s *PostgresStore) GetPendingInviteByTokenHash(ctx context.Context, tokenHash string) (*model.OrganizationInvite, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT i.id, i.tenant_id, COALESCE(t.name, ''), i.email, i.role, COALESCE(i.invited_by, ''),
		       i.expires_at, i.accepted_at, i.created_at
		FROM organization_invites i
		LEFT JOIN tenants t ON t.id = i.tenant_id
		WHERE i.token_hash = $1 AND i.accepted_at IS NULL
	`, tokenHash)
	return scanInvite(row)
}

func (s *PostgresStore) ListPendingInvitesForEmail(ctx context.Context, email string) ([]*model.OrganizationInvite, error) {
	email = normalizeEmail(email)
	rows, err := s.pool.Query(ctx, `
		SELECT i.id, i.tenant_id, COALESCE(t.name, ''), i.email, i.role, COALESCE(i.invited_by, ''),
		       i.expires_at, i.accepted_at, i.created_at
		FROM organization_invites i
		LEFT JOIN tenants t ON t.id = i.tenant_id
		WHERE LOWER(i.email) = $1 AND i.accepted_at IS NULL AND i.expires_at > NOW()
		ORDER BY i.created_at DESC
	`, email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanInviteRows(rows)
}

func (s *PostgresStore) ListPendingInvitesForTenant(ctx context.Context, tenantID string) ([]*model.OrganizationInvite, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT i.id, i.tenant_id, COALESCE(t.name, ''), i.email, i.role, COALESCE(i.invited_by, ''),
		       i.expires_at, i.accepted_at, i.created_at
		FROM organization_invites i
		LEFT JOIN tenants t ON t.id = i.tenant_id
		WHERE i.tenant_id = $1 AND i.accepted_at IS NULL AND i.expires_at > NOW()
		ORDER BY i.created_at DESC
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanInviteRows(rows)
}

func (s *PostgresStore) MarkInviteAccepted(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE organization_invites SET accepted_at = NOW() WHERE id = $1
	`, id)
	return err
}

func (s *PostgresStore) DeleteInvite(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM organization_invites WHERE id = $1`, id)
	return err
}
