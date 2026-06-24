package auth

import (
	"net/http"
	"os"
	"strings"
)

// AdminAuth is a very simple bearer token middleware for the control plane admin API.
// In production this will be replaced by proper OIDC/JWT or mTLS + service accounts.
func AdminAuth(next http.Handler) http.Handler {
	expected := os.Getenv("NANCE_ADMIN_TOKEN")
	if expected == "" {
		// In dev we allow everything with a warning (DO NOT do this in real prod)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		token := strings.TrimPrefix(auth, "Bearer ")
		if token != expected {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
