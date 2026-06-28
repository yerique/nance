package auth

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/taeven/nance/accelerator/internal/controlplane/service"
	"github.com/taeven/nance/accelerator/internal/model"
)

type ctxKey int

const (
	userCtxKey ctxKey = iota
	adminCtxKey
	rawTokenCtxKey
)

// Principal is either a dashboard user session or a platform admin token.
type Principal struct {
	User     *model.User
	IsAdmin  bool // NANCE_ADMIN_TOKEN superuser
	RawToken string
}

// UserFromContext returns the authenticated dashboard user, if any.
func UserFromContext(ctx context.Context) *model.User {
	if p, ok := ctx.Value(userCtxKey).(*Principal); ok && p != nil && p.User != nil {
		return p.User
	}
	return nil
}

// PrincipalFromContext returns the full principal.
func PrincipalFromContext(ctx context.Context) *Principal {
	if p, ok := ctx.Value(userCtxKey).(*Principal); ok {
		return p
	}
	return nil
}

// IsPlatformAdmin is true when the request used NANCE_ADMIN_TOKEN.
func IsPlatformAdmin(ctx context.Context) bool {
	p := PrincipalFromContext(ctx)
	return p != nil && p.IsAdmin
}

// Middleware authenticates either a user session token or platform admin bearer.
// Public routes should not use this middleware.
func Middleware(authSvc *service.AuthService) func(http.Handler) http.Handler {
	expectedAdmin := os.Getenv("NANCE_ADMIN_TOKEN")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authz := r.Header.Get("Authorization")
			if !strings.HasPrefix(authz, "Bearer ") {
				// Dev fallback: no admin token configured and no bearer — allow unauthenticated
				// only when NANCE_ADMIN_TOKEN is empty (legacy open mode). Prefer explicit login.
				if expectedAdmin == "" && os.Getenv("NANCE_REQUIRE_USER_AUTH") != "1" {
					next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userCtxKey, &Principal{IsAdmin: true})))
					return
				}
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			token := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
			if expectedAdmin != "" && token == expectedAdmin {
				ctx := context.WithValue(r.Context(), userCtxKey, &Principal{IsAdmin: true, RawToken: token})
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			if authSvc != nil {
				u, err := authSvc.SessionUser(r.Context(), token)
				if err == nil && u != nil {
					ctx := context.WithValue(r.Context(), userCtxKey, &Principal{User: u, RawToken: token})
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		})
	}
}

// RequireUser ensures a dashboard user is logged in (not just platform admin open mode without user).
// Platform admin may still access tenant APIs without being a member.
func RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := PrincipalFromContext(r.Context())
		if p == nil || (p.User == nil && !p.IsAdmin) {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		// For /me routes we need a real user
		if p.User == nil {
			http.Error(w, `{"error":"user session required"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
