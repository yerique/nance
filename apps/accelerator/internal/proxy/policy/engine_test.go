package policy

import (
	"testing"
	"time"

	"github.com/taeven/nance/accelerator/internal/model"
)

func TestEngine_LookupExplicitEnable(t *testing.T) {
	e := NewEngine(nil, nil, time.Minute)
	maxB := 4096
	e.SetForTest("t1", &model.CachePolicy{
		TenantID:          "t1",
		DefaultTtlSeconds: 30,
		CacheKeyVersion:   2,
		Collections: map[string]model.CollectionPolicy{
			"mydb.orders": {Enabled: true, TTLSeconds: 10, MaxResultBytes: &maxB},
		},
	})
	d := e.Lookup("t1", "mydb", "orders")
	if !d.Enabled || d.TTL != 10*time.Second || d.MaxResultBytes != 4096 || d.CacheKeyVersion != 2 {
		t.Fatalf("unexpected decision: %+v", d)
	}
	// not configured => disabled
	d2 := e.Lookup("t1", "mydb", "other")
	if d2.Enabled {
		t.Fatal("expected disabled for unlisted collection")
	}
	// unknown tenant
	d3 := e.Lookup("nope", "mydb", "orders")
	if d3.Enabled {
		t.Fatal("unknown tenant must be disabled")
	}
}

func TestEngine_DefaultTTL(t *testing.T) {
	e := NewEngine(nil, nil, time.Minute)
	e.SetForTest("t1", &model.CachePolicy{
		TenantID:          "t1",
		DefaultTtlSeconds: 45,
		Collections: map[string]model.CollectionPolicy{
			"db.c": {Enabled: true, TTLSeconds: 0},
		},
	})
	d := e.Lookup("t1", "db", "c")
	if d.TTL != 45*time.Second {
		t.Fatalf("ttl=%v", d.TTL)
	}
}
