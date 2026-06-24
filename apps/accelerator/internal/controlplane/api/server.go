package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/taeven/nance/accelerator/internal/controlplane/auth"
	"github.com/taeven/nance/accelerator/internal/controlplane/service"
	"github.com/taeven/nance/accelerator/internal/controlplane/api/handlers"
)

func NewServer(
	ts *service.TenantService,
	bs *service.BackendService,
	ps *service.PolicyService,
	toks *service.TokenService,
) http.Handler {
	r := chi.NewRouter()

	// Basic middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	h := handlers.NewHandlers(ts, bs, ps, toks)

	// Public / infra
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		// In Phase 0 we are ready as long as we can start (DB ping already happened)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})
	r.Handle("/metrics", promhttp.Handler())

	// Protected API
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(auth.AdminAuth)

		// Tenants
		r.Post("/tenants", h.CreateTenant)
		r.Get("/tenants", h.ListTenants)
		r.Get("/tenants/{tenantId}", h.GetTenant)

		// Backends
		r.Post("/tenants/{tenantId}/backend", h.SetBackend)
		r.Post("/tenants/{tenantId}/backend/test", h.TestBackend)

		// Policies
		r.Get("/tenants/{tenantId}/policy", h.GetPolicy)
		r.Put("/tenants/{tenantId}/policy/collections/{dbColl}", h.SetCollectionPolicy)
		r.Put("/tenants/{tenantId}/policy/defaults", h.SetDefaultTTL)

		// Tokens
		r.Post("/tenants/{tenantId}/tokens", h.IssueToken)
		r.Get("/tenants/{tenantId}/tokens", h.ListTokens)
		r.Delete("/tokens/{tokenId}", h.RevokeToken)
	})

	return r
}
