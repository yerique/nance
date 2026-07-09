package service

import (
	"context"
	"testing"
	"time"

	"github.com/taeven/nance/accelerator/internal/controlplane/store"
)

func TestPasswordAuth_SetLoginReset(t *testing.T) {
	ctx := context.Background()
	ms := store.NewMemoryStore()
	auth := NewAuthService(ms, &LogMailer{}, nil).WithPasswordAuth(true).WithAppPublicURL("https://app.test")

	u, err := ms.UpsertUserByEmail(ctx, "a@example.com", "A")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := auth.SetPassword(ctx, u.ID, "", "short"); err != ErrWeakPassword {
		t.Fatalf("weak: %v", err)
	}
	if _, err := auth.SetPassword(ctx, u.ID, "", "goodpass1"); err != nil {
		t.Fatal(err)
	}
	tok, user, err := auth.LoginWithPassword(ctx, "a@example.com", "goodpass1")
	if err != nil || tok == "" || !user.HasPassword {
		t.Fatalf("%v %q %+v", err, tok, user)
	}
	if _, _, err := auth.LoginWithPassword(ctx, "a@example.com", "wrong"); err != ErrInvalidCredentials {
		t.Fatalf("want invalid creds, got %v", err)
	}

	if _, err := auth.UpdatePassword(ctx, u.ID, "goodpass1", "betterpass2"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := auth.LoginWithPassword(ctx, "a@example.com", "betterpass2"); err != nil {
		t.Fatal(err)
	}

	off := NewAuthService(ms, &LogMailer{}, nil).WithPasswordAuth(false)
	if _, _, err := off.LoginWithPassword(ctx, "a@example.com", "betterpass2"); err != ErrPasswordAuthOff {
		t.Fatalf("want off, got %v", err)
	}

	raw := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	if err := ms.CreatePasswordResetToken(ctx, "pwr_t", u.ID, hashToken(raw), time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := auth.ResetPassword(ctx, raw, "newestpass9"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := auth.LoginWithPassword(ctx, "a@example.com", "newestpass9"); err != nil {
		t.Fatal(err)
	}
}
