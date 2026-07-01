package config

import (
	"testing"
	"time"
)

func TestLoad_DefaultsAndIdleZero(t *testing.T) {
	t.Setenv("NANCE_PROXY_LISTEN", "")
	t.Setenv("NANCE_PROXY_BACKEND_IDLE_TIMEOUT", "")
	cfg := Load()
	if cfg.ListenAddr != ":27018" {
		t.Fatalf("listen %s", cfg.ListenAddr)
	}
	if cfg.BackendIdleTimeout != 15*time.Minute {
		t.Fatalf("idle default %v", cfg.BackendIdleTimeout)
	}
	t.Setenv("NANCE_PROXY_BACKEND_IDLE_TIMEOUT", "0")
	cfg2 := Load()
	if cfg2.BackendIdleTimeout != 0 {
		t.Fatalf("want 0 got %v", cfg2.BackendIdleTimeout)
	}
	t.Setenv("NANCE_PROXY_BACKEND_IDLE_TIMEOUT", "2m")
	cfg3 := Load()
	if cfg3.BackendIdleTimeout != 2*time.Minute {
		t.Fatalf("got %v", cfg3.BackendIdleTimeout)
	}
}
