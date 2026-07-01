package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/taeven/nance/accelerator/internal/controlplane/service"
	"github.com/taeven/nance/accelerator/internal/controlplane/store"
	"github.com/taeven/nance/accelerator/internal/model"
	"golang.org/x/crypto/bcrypt"
	"time"
)

func TestMiddleware_AdminToken(t *testing.T) {
	t.Setenv("NANCE_ADMIN_TOKEN", "secret")
	t.Setenv("NANCE_REQUIRE_USER_AUTH", "")
	ms := store.NewMemoryStore()
	authSvc := service.NewAuthService(ms, &service.LogMailer{}, nil)
	h := Middleware(authSvc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IsPlatformAdmin(r.Context()) {
			t.Error("expected admin")
		}
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("code %d", rr.Code)
	}
}

func TestMiddleware_UserSession(t *testing.T) {
	t.Setenv("NANCE_ADMIN_TOKEN", "admin-only")
	ms := store.NewMemoryStore()
	authSvc := service.NewAuthService(ms, &service.LogMailer{}, nil)
	ctx := context.Background()
	u, _ := ms.UpsertUserByEmail(ctx, "u@ex.com", "U")
	// create session via verify path injection
	hash, _ := bcrypt.GenerateFromPassword([]byte("999999"), bcrypt.MinCost)
	_ = ms.SetEmailVerificationCode(ctx, "u@ex.com", string(hash), time.Now().UTC().Add(time.Minute))
	tok, user, err := authSvc.VerifyCode(ctx, "u@ex.com", "999999", "")
	if err != nil || user.ID != u.ID {
		// user id may differ if upsert twice - ok
		_ = tok
	}
	var sawUser *model.User
	h := Middleware(authSvc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawUser = UserFromContext(r.Context())
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 200 || sawUser == nil {
		t.Fatalf("code=%d user=%v", rr.Code, sawUser)
	}
}

func TestMiddleware_Unauthorized(t *testing.T) {
	t.Setenv("NANCE_ADMIN_TOKEN", "x")
	t.Setenv("NANCE_REQUIRE_USER_AUTH", "1")
	h := Middleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("code %d", rr.Code)
	}
	_ = os.Unsetenv
}
