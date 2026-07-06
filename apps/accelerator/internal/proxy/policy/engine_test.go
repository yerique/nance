package policy

import (
	"testing"
	"time"

	"github.com/taeven/nance/accelerator/internal/model"
)

func TestResolve_DefaultsAndOverrides(t *testing.T) {
	e := NewEngine(nil, nil, time.Minute)
	e.SetForTest("conn1", &model.CachePolicy{
		ConnectionID:      "conn1",
		TenantID:          "t1",
		DefaultTtlSeconds: 30,
		CacheKeyVersion:   2,
		Collections: map[string]model.CollectionPolicy{
			"db.orders": {Enabled: true, TTLSeconds: 10},
		},
	})
	d := e.Resolve("conn1", "db", "orders")
	if !d.Enabled || d.TTL != 10*time.Second || d.CacheKeyVersion != 2 {
		t.Fatalf("%+v", d)
	}
	d2 := e.Resolve("conn1", "db", "other")
	if d2.TTL != 30*time.Second {
		t.Fatalf("default ttl: %+v", d2)
	}
	// Unknown connection still gets built-in default
	d3 := e.Resolve("missing", "db", "c")
	if !d3.Enabled || d3.TTL != 60*time.Second {
		t.Fatalf("%+v", d3)
	}
}
