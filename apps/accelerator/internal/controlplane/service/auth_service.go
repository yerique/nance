package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/taeven/nance/accelerator/internal/controlplane/store"
	"github.com/taeven/nance/accelerator/internal/model"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidEmail       = errors.New("invalid email")
	ErrInvalidCode        = errors.New("invalid or expired verification code")
	ErrTooManyAttempts    = errors.New("too many verification attempts")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrInviteNotFound     = errors.New("invite not found")
	ErrInviteExpired      = errors.New("invite expired")
	ErrAlreadyMember      = errors.New("already a member")
	ErrNotMember          = errors.New("not a member")
	ErrLastOwner          = errors.New("cannot remove the last owner")
	ErrPasswordAuthOff    = errors.New("password authentication is disabled")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrWeakPassword       = errors.New("password must be at least 8 characters")
	ErrPasswordMismatch   = errors.New("current password is incorrect")
	ErrNoPasswordSet      = errors.New("no password is set for this account")
	ErrPasswordAlreadySet = errors.New("password is already set; use update with current password")
	ErrInvalidResetToken  = errors.New("invalid or expired reset link")
)

// AuthService handles email OTP login, optional password login, and sessions.
type AuthService struct {
	store               store.Store
	mailer              Mailer
	log                 *slog.Logger
	passwordAuthEnabled bool
	appPublicURL        string // e.g. https://app.oxella.com for reset links
}

func NewAuthService(s store.Store, mailer Mailer, log *slog.Logger) *AuthService {
	if log == nil {
		log = slog.Default()
	}
	if mailer == nil {
		mailer = &LogMailer{Log: log}
	}
	return &AuthService{store: s, mailer: mailer, log: log, appPublicURL: "https://app.oxella.com"}
}

// WithPasswordAuth enables or disables password login / set / reset.
func (s *AuthService) WithPasswordAuth(enabled bool) *AuthService {
	s.passwordAuthEnabled = enabled
	return s
}

// WithAppPublicURL sets the dashboard base URL used in password-reset emails.
func (s *AuthService) WithAppPublicURL(url string) *AuthService {
	url = strings.TrimRight(strings.TrimSpace(url), "/")
	if url != "" {
		s.appPublicURL = url
	}
	return s
}

// PasswordAuthEnabled reports whether password features are on.
func (s *AuthService) PasswordAuthEnabled() bool {
	return s.passwordAuthEnabled
}

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !emailRe.MatchString(email) {
		return "", ErrInvalidEmail
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return "", ErrInvalidEmail
	}
	return email, nil
}

// RequestCode sends a 6-digit verification code to the email (signup or login).
func (s *AuthService) RequestCode(ctx context.Context, email string) error {
	email, err := normalizeEmail(email)
	if err != nil {
		return err
	}
	code, err := randomDigits(6)
	if err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	expires := time.Now().UTC().Add(10 * time.Minute)
	if err := s.store.SetEmailVerificationCode(ctx, email, string(hash), expires); err != nil {
		return err
	}
	const expiryMinutes = 10
	_, htmlBody := otpEmailBodies(code, expiryMinutes)
	// Prefer HTML (Oxella branded). SMTP mailer detects HTML. Never log the OTP.
	if err := s.mailer.Send(ctx, email, otpEmailSubject, htmlBody); err != nil {
		s.log.Warn("failed to send verification email", "email", email, "error", err)
		// Still return success to avoid email enumeration timing.
	}
	return nil
}

// VerifyCode validates the OTP, creates the user if needed, and returns a session token.
func (s *AuthService) VerifyCode(ctx context.Context, email, code, name string) (rawToken string, user *model.User, err error) {
	email, err = normalizeEmail(email)
	if err != nil {
		return "", nil, err
	}
	code = strings.TrimSpace(code)
	if len(code) < 4 {
		return "", nil, ErrInvalidCode
	}
	hash, expires, attempts, err := s.store.GetEmailVerificationCode(ctx, email)
	if err != nil {
		return "", nil, ErrInvalidCode
	}
	if attempts >= 8 {
		return "", nil, ErrTooManyAttempts
	}
	if time.Now().UTC().After(expires) {
		_ = s.store.ClearEmailVerificationCode(ctx, email)
		return "", nil, ErrInvalidCode
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(code)) != nil {
		_ = s.store.IncrementEmailVerificationAttempts(ctx, email)
		return "", nil, ErrInvalidCode
	}
	_ = s.store.ClearEmailVerificationCode(ctx, email)

	user, err = s.store.UpsertUserByEmail(ctx, email, strings.TrimSpace(name))
	if err != nil {
		return "", nil, err
	}

	raw, err := s.issueSession(ctx, user.ID)
	if err != nil {
		return "", nil, err
	}
	return raw, user, nil
}

// SessionUser resolves a bearer session token to a user.
func (s *AuthService) SessionUser(ctx context.Context, rawToken string) (*model.User, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return nil, ErrUnauthorized
	}
	sid, uid, exp, err := s.store.GetSessionByTokenHash(ctx, hashToken(rawToken))
	if err != nil {
		return nil, ErrUnauthorized
	}
	if time.Now().UTC().After(exp) {
		_ = s.store.DeleteSession(ctx, sid)
		return nil, ErrUnauthorized
	}
	u, err := s.store.GetUserByID(ctx, uid)
	if err != nil {
		return nil, ErrUnauthorized
	}
	return u, nil
}

// Logout deletes the session for the raw token.
func (s *AuthService) Logout(ctx context.Context, rawToken string) error {
	return s.store.DeleteSessionByTokenHash(ctx, hashToken(rawToken))
}

// Me returns the user by id.
func (s *AuthService) Me(ctx context.Context, userID string) (*model.User, error) {
	u, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}
	return u, nil
}

// UpdateProfile sets display name (and future profile fields) for the user.
func (s *AuthService) UpdateProfile(ctx context.Context, userID, name string) (*model.User, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("name is required")
	}
	if err := s.store.UpdateUserName(ctx, userID, name); err != nil {
		return nil, err
	}
	return s.store.GetUserByID(ctx, userID)
}

// LoginWithPassword authenticates with email + password and returns a session token.
func (s *AuthService) LoginWithPassword(ctx context.Context, email, password string) (rawToken string, user *model.User, err error) {
	if !s.passwordAuthEnabled {
		return "", nil, ErrPasswordAuthOff
	}
	email, err = normalizeEmail(email)
	if err != nil {
		return "", nil, ErrInvalidCredentials
	}
	password = strings.TrimSpace(password)
	if password == "" {
		return "", nil, ErrInvalidCredentials
	}
	user, err = s.store.GetUserByEmail(ctx, email)
	if err != nil {
		// uniform error — no email enumeration
		return "", nil, ErrInvalidCredentials
	}
	hash, err := s.store.GetUserPasswordHash(ctx, user.ID)
	if err != nil {
		return "", nil, ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return "", nil, ErrInvalidCredentials
	}
	raw, err := s.issueSession(ctx, user.ID)
	if err != nil {
		return "", nil, err
	}
	user.HasPassword = true
	return raw, user, nil
}

// SetPassword sets a password for the first time (account already exists via OTP).
// currentPassword is ignored when no password is set yet.
func (s *AuthService) SetPassword(ctx context.Context, userID, currentPassword, newPassword string) (*model.User, error) {
	if !s.passwordAuthEnabled {
		return nil, ErrPasswordAuthOff
	}
	if err := validatePassword(newPassword); err != nil {
		return nil, err
	}
	_, err := s.store.GetUserPasswordHash(ctx, userID)
	if err == nil {
		// already has password — require current
		return s.UpdatePassword(ctx, userID, currentPassword, newPassword)
	}
	if !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	if err := s.store.SetUserPasswordHash(ctx, userID, string(hash)); err != nil {
		return nil, err
	}
	return s.store.GetUserByID(ctx, userID)
}

// UpdatePassword changes an existing password (requires current password).
func (s *AuthService) UpdatePassword(ctx context.Context, userID, currentPassword, newPassword string) (*model.User, error) {
	if !s.passwordAuthEnabled {
		return nil, ErrPasswordAuthOff
	}
	if err := validatePassword(newPassword); err != nil {
		return nil, err
	}
	hash, err := s.store.GetUserPasswordHash(ctx, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNoPasswordSet
		}
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(currentPassword)) != nil {
		return nil, ErrPasswordMismatch
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	if err := s.store.SetUserPasswordHash(ctx, userID, string(newHash)); err != nil {
		return nil, err
	}
	return s.store.GetUserByID(ctx, userID)
}

// RequestPasswordReset emails a reset link if the account has a password. Always succeeds (no enumeration).
func (s *AuthService) RequestPasswordReset(ctx context.Context, email string) error {
	if !s.passwordAuthEnabled {
		return ErrPasswordAuthOff
	}
	email, err := normalizeEmail(email)
	if err != nil {
		return nil // still ok
	}
	user, err := s.store.GetUserByEmail(ctx, email)
	if err != nil {
		return nil
	}
	if _, err := s.store.GetUserPasswordHash(ctx, user.ID); err != nil {
		return nil // no password set — no email
	}
	raw, err := randomToken(32)
	if err != nil {
		return err
	}
	id := "pwr_" + cryptoRandHex(12)
	exp := time.Now().UTC().Add(60 * time.Minute)
	if err := s.store.CreatePasswordResetToken(ctx, id, user.ID, hashToken(raw), exp); err != nil {
		return err
	}
	link := s.appPublicURL + "/reset-password?token=" + raw
	_, htmlBody := passwordResetEmailBodies(link, 60)
	if err := s.mailer.Send(ctx, email, passwordResetEmailSubject, htmlBody); err != nil {
		s.log.Warn("failed to send password reset email", "email", email, "error", err)
	}
	return nil
}

// ResetPassword consumes a reset token and sets a new password.
func (s *AuthService) ResetPassword(ctx context.Context, rawToken, newPassword string) error {
	if !s.passwordAuthEnabled {
		return ErrPasswordAuthOff
	}
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return ErrInvalidResetToken
	}
	if err := validatePassword(newPassword); err != nil {
		return err
	}
	userID, err := s.store.ConsumePasswordResetToken(ctx, hashToken(rawToken))
	if err != nil {
		return ErrInvalidResetToken
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.store.SetUserPasswordHash(ctx, userID, string(hash))
}

func (s *AuthService) issueSession(ctx context.Context, userID string) (string, error) {
	raw, err := randomToken(32)
	if err != nil {
		return "", err
	}
	th := hashToken(raw)
	sid := "ses_" + cryptoRandHex(12)
	exp := time.Now().UTC().Add(30 * 24 * time.Hour)
	if err := s.store.CreateSession(ctx, sid, userID, th, exp); err != nil {
		return "", err
	}
	return raw, nil
}

func validatePassword(pw string) error {
	pw = strings.TrimSpace(pw)
	if len(pw) < 8 {
		return ErrWeakPassword
	}
	if len(pw) > 128 {
		return errors.New("password is too long")
	}
	return nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func randomDigits(n int) (string, error) {
	const digits = "0123456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	out := make([]byte, n)
	for i := range b {
		out[i] = digits[int(b[i])%10]
	}
	return string(out), nil
}

func randomToken(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func cryptoRandHex(nBytes int) string {
	b := make([]byte, nBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
