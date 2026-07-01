package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/taeven/nance/accelerator/internal/controlplane/api/handlers"
	"github.com/taeven/nance/accelerator/internal/controlplane/service"
	"github.com/taeven/nance/accelerator/internal/controlplane/store"
)

func TestNewServer_PlatformAndHealth(t *testing.T) {
	ms := store.NewMemoryStore()
	ts := service.NewTenantService(ms)
	auth := service.NewAuthService(ms, &service.LogMailer{}, nil)
	orgs := service.NewOrgService(ms, nil).WithInviteOnly(true)
	h := NewServer(ts, nil, nil, nil, auth, orgs, handlers.PlatformPublic{
		InviteOnly: true, AllowOrgCreation: false, AllowAdminBootstrap: true,
	})

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != 200 {
		t.Fatalf("healthz %d", rr.Code)
	}

	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/api/v1/platform", nil))
	if rr2.Code != 200 {
		t.Fatalf("platform %d body %s", rr2.Code, rr2.Body.String())
	}
}
