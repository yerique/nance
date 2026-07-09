package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/taeven/nance/accelerator/internal/controlplane/store"
	"golang.org/x/crypto/bcrypt"
)

type captureMailer struct {
	lastTo, lastSubject, lastBody string
	n                             int
}

func (c *captureMailer) Send(_ context.Context, to, subject, body string) error {
	c.n++
	c.lastTo, c.lastSubject, c.lastBody = to, subject, body
	return nil
}

func TestAuthService_RequestAndVerify(t *testing.T) {
	ms := store.NewMemoryStore()
	mail := &captureMailer{}
	svc := NewAuthService(ms, mail, nil)
	ctx := context.Background()

	if err := svc.RequestCode(ctx, "not-an-email"); err != ErrInvalidEmail {
		t.Fatalf("want invalid email, got %v", err)
	}
	if err := svc.RequestCode(ctx, "Ada@Example.COM"); err != nil {
		t.Fatal(err)
	}
	if mail.n != 1 || mail.lastSubject != "Your Oxella sign-in code" {
		t.Fatalf("mail not sent: %+v", mail)
	}
	if !strings.Contains(mail.lastBody, "Verification code") && !strings.Contains(strings.ToLower(mail.lastBody), "verification code") {
		t.Fatalf("mail body missing code label: %+v", mail)
	}
	// Pull code from store by brute forcing is hard; inject known code
	hash, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.MinCost)
	_ = ms.SetEmailVerificationCode(ctx, "ada@example.com", string(hash), time.Now().UTC().Add(10*time.Minute))

	tok, user, err := svc.VerifyCode(ctx, "ada@example.com", "123456", "")
	if err != nil || tok == "" || user == nil || user.Email != "ada@example.com" {
		t.Fatalf("verify: tok=%q user=%+v err=%v", tok, user, err)
	}
	u2, err := svc.SessionUser(ctx, tok)
	if err != nil || u2.ID != user.ID {
		t.Fatalf("session: %+v %v", u2, err)
	}
	_ = svc.Logout(ctx, tok)
	if _, err := svc.SessionUser(ctx, tok); err != ErrUnauthorized {
		t.Fatalf("expected unauthorized after logout, got %v", err)
	}
}

func TestAuthService_UpdateProfile(t *testing.T) {
	ms := store.NewMemoryStore()
	svc := NewAuthService(ms, &captureMailer{}, nil)
	ctx := context.Background()
	u, _ := ms.UpsertUserByEmail(ctx, "a@b.co", "")
	if _, err := svc.UpdateProfile(ctx, u.ID, "  "); err == nil {
		t.Fatal("empty name should fail")
	}
	out, err := svc.UpdateProfile(ctx, u.ID, "Ada")
	if err != nil || out.Name != "Ada" {
		t.Fatalf("%+v %v", out, err)
	}
}

func TestAuthService_BadCode(t *testing.T) {
	ms := store.NewMemoryStore()
	svc := NewAuthService(ms, &LogMailer{}, nil)
	ctx := context.Background()
	hash, _ := bcrypt.GenerateFromPassword([]byte("000000"), bcrypt.MinCost)
	_ = ms.SetEmailVerificationCode(ctx, "x@y.z", string(hash), time.Now().UTC().Add(time.Minute))
	if _, _, err := svc.VerifyCode(ctx, "x@y.z", "111111", ""); err != ErrInvalidCode {
		t.Fatalf("got %v", err)
	}
}
