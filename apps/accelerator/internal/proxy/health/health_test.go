package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthzReadyz(t *testing.T) {
	s := &Server{}
	h := s.Handler()

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != 200 {
		t.Fatalf("healthz %d", rr.Code)
	}
	var m map[string]string
	_ = json.NewDecoder(rr.Body).Decode(&m)
	if m["status"] != "ok" {
		t.Fatalf("%v", m)
	}

	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rr2.Code != 200 {
		t.Fatalf("readyz %d", rr2.Code)
	}

	s.ReadyFn = func(ctx context.Context) error { return errors.New("down") }
	rr3 := httptest.NewRecorder()
	h.ServeHTTP(rr3, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rr3.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 got %d", rr3.Code)
	}
}
