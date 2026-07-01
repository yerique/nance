package server

import (
	"testing"

	proxyconfig "github.com/taeven/nance/accelerator/internal/proxy/config"
)

// New requires live dependencies; cover config defaults used by server construction.
func TestConfigListenDefault(t *testing.T) {
	cfg := proxyconfig.Load()
	if cfg.ListenAddr == "" || cfg.HealthAddr == "" {
		t.Fatal("expected listen addresses")
	}
}
