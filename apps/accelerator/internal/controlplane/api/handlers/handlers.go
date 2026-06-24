package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/taeven/nance/accelerator/internal/controlplane/service"
	"github.com/taeven/nance/accelerator/internal/model"
)

type Handlers struct {
	tenants  *service.TenantService
	backends *service.BackendService
	policies *service.PolicyService
	tokens   *service.TokenService
}

func NewHandlers(
	ts *service.TenantService,
	bs *service.BackendService,
	ps *service.PolicyService,
	toks *service.TokenService,
) *Handlers {
	return &Handlers{
		tenants:  ts,
		backends: bs,
		policies: ps,
		tokens:   toks,
	}
}

// writeJSON is a small helper.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// ===== Tenant handlers =====

func (h *Handlers) CreateTenant(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	t, err := h.tenants.Create(r.Context(), req.ID, req.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (h *Handlers) GetTenant(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "tenantId")
	t, err := h.tenants.Get(r.Context(), id)
	if err != nil {
		if err == service.ErrTenantNotFound {
			writeError(w, http.StatusNotFound, "tenant not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *Handlers) ListTenants(w http.ResponseWriter, r *http.Request) {
	list, err := h.tenants.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// ===== Backend handlers =====

func (h *Handlers) SetBackend(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantId")
	var req struct {
		URI string `json:"uri"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := h.backends.SetBackend(r.Context(), tenantID, req.URI); err != nil {
		if err == service.ErrTenantNotFound {
			writeError(w, http.StatusNotFound, "tenant not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "stored (encrypted)"})
}

func (h *Handlers) TestBackend(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantId")
	if err := h.backends.TestConnection(r.Context(), tenantID); err != nil {
		if err == service.ErrBackendNotFound || err == service.ErrTenantNotFound {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "connection successful"})
}

// ===== Policy handlers =====

func (h *Handlers) GetPolicy(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantId")
	p, err := h.policies.Get(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *Handlers) SetCollectionPolicy(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantId")
	dbColl := chi.URLParam(r, "dbColl")

	var pol model.CollectionPolicy
	if err := json.NewDecoder(r.Body).Decode(&pol); err != nil {
		writeError(w, http.StatusBadRequest, "invalid policy json")
		return
	}
	if err := h.policies.SetCollectionPolicy(r.Context(), tenantID, dbColl, pol); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *Handlers) SetDefaultTTL(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantId")
	var req struct {
		DefaultTTL int `json:"defaultTtlSeconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := h.policies.SetDefaults(r.Context(), tenantID, req.DefaultTTL); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// ===== Token handlers =====

type issueTokenResponse struct {
	TokenID     string `json:"tokenId"`
	RawToken    string `json:"rawToken"` // ONLY returned at issuance time
	TenantID    string `json:"tenantId"`
	Description string `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (h *Handlers) IssueToken(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantId")
	var req struct {
		Description string `json:"description"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req) // description is optional

	raw, tok, err := h.tokens.Issue(r.Context(), tenantID, req.Description)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := issueTokenResponse{
		TokenID:     tok.ID,
		RawToken:    raw,
		TenantID:    tok.TenantID,
		Description: tok.Description,
		CreatedAt:   tok.CreatedAt,
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handlers) ListTokens(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantId")
	toks, err := h.tokens.List(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toks)
}

func (h *Handlers) RevokeToken(w http.ResponseWriter, r *http.Request) {
	tokenID := chi.URLParam(r, "tokenId")
	if err := h.tokens.Revoke(r.Context(), tokenID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// Invalidate clears cache entries for a tenant namespace and/or tags (Phase 3).
// Body: { "db": "mydb", "coll": "orders", "tags": ["user:1"] }
func (h *Handlers) Invalidate(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantId")
	var req struct {
		DB   string   `json:"db"`
		Coll string   `json:"coll"`
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		// allow empty body with query params
	}
	if req.DB == "" {
		req.DB = r.URL.Query().Get("db")
	}
	if req.Coll == "" {
		req.Coll = r.URL.Query().Get("coll")
	}
	if err := h.policies.Invalidate(r.Context(), tenantID, req.DB, req.Coll, req.Tags); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "invalidated",
		"tenantId": tenantID,
		"db":       req.DB,
		"coll":     req.Coll,
		"tags":     req.Tags,
	})
}
