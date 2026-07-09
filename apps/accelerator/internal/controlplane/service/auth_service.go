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
	ErrInvalidEmail    = errors.New("invalid email")
	ErrInvalidCode     = errors.New("invalid or expired verification code")
	ErrTooManyAttempts = errors.New("too many verification attempts")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrForbidden       = errors.New("forbidden")
	ErrInviteNotFound  = errors.New("invite not found")
	ErrInviteExpired   = errors.New("invite expired")
	ErrAlreadyMember   = errors.New("already a member")
	ErrNotMember       = errors.New("not a member")
	ErrLastOwner       = errors.New("cannot remove the last owner")
)

// AuthService handles email OTP login and sessions.
type AuthService struct {
	store  store.Store
	mailer Mailer
	log    *slog.Logger
}

func NewAuthService(s store.Store, mailer Mailer, log *slog.Logger) *AuthService {
	if log == nil {
		log = slog.Default()
	}
	if mailer == nil {
		mailer = &LogMailer{Log: log}
	}
	return &AuthService{store: s, mailer: mailer, log: log}
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

	raw, err := randomToken(32)
	if err != nil {
		return "", nil, err
	}
	th := hashToken(raw)
	sid := "ses_" + cryptoRandHex(12)
	exp := time.Now().UTC().Add(30 * 24 * time.Hour)
	if err := s.store.CreateSession(ctx, sid, user.ID, th, exp); err != nil {
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
