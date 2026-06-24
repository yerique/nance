package ratelimit

import "testing"

func TestLimiterBurstThenBlock(t *testing.T) {
	l := New(1, 2) // 1 qps, burst 2
	if !l.Allow("t") || !l.Allow("t") {
		t.Fatal("burst should allow 2")
	}
	if l.Allow("t") {
		t.Fatal("third should block")
	}
}

func TestLimiterIndependentTenants(t *testing.T) {
	l := New(1, 1)
	if !l.Allow("a") {
		t.Fatal("a")
	}
	if !l.Allow("b") {
		t.Fatal("b should have own bucket")
	}
}
