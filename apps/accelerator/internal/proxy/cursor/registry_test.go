package cursor

import (
	"testing"
	"time"
)

// minimal stand-in — Registry uses *mongo.Cursor; test ID allocation via Register with nil is unsafe.
// Test prune of empty map and KillMany no-op.

func TestRegistry_KillManyEmpty(t *testing.T) {
	r := NewRegistry(time.Minute)
	r.KillMany(nil, "t", "c")
	r.KillMany([]int64{1, 2}, "t", "c")
}

func TestRegistry_GetMissing(t *testing.T) {
	r := NewRegistry(time.Minute)
	if _, ok := r.Get(99, "t", "c"); ok {
		t.Fatal("expected miss")
	}
}
