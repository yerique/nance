package pool

import (
	"context"
	"testing"
	"time"

	proxyconfig "github.com/taeven/nance/accelerator/internal/proxy/config"
)

func TestEvictIdle_RemovesUnused(t *testing.T) {
	m := NewManager(nil, nil, &proxyconfig.Config{
		BackendIdleTimeout:       50 * time.Millisecond,
		BackendIdleEvictInterval: time.Hour, // don't need loop
	}, nil)
	m.InjectClientForTest("t1", nil, time.Now().Add(-time.Minute), 0)
	m.InjectClientForTest("t2", nil, time.Now(), 0)                   // fresh
	m.InjectClientForTest("t3", nil, time.Now().Add(-time.Minute), 2) // in use

	if m.ClientCount() != 3 {
		t.Fatalf("count=%d", m.ClientCount())
	}
	m.EvictIdleForTest(context.Background())
	// t1 evicted (nil client path deletes), t2 kept (recent), t3 kept (refs)
	if m.ClientCount() != 2 {
		t.Fatalf("after evict count=%d want 2", m.ClientCount())
	}
	if m.RefsForTest("t3") != 2 {
		t.Fatalf("t3 refs=%d", m.RefsForTest("t3"))
	}
	if m.RefsForTest("t1") != -1 {
		t.Fatal("t1 should be gone")
	}
}

func TestRelease_DecrementsRefs(t *testing.T) {
	m := NewManager(nil, nil, &proxyconfig.Config{BackendIdleTimeout: time.Hour}, nil)
	m.InjectClientForTest("t1", nil, time.Now(), 1)
	m.Release("t1")
	if m.RefsForTest("t1") != 0 {
		t.Fatalf("refs=%d", m.RefsForTest("t1"))
	}
	m.Release("t1") // no underflow below 0 in logic - stays 0
	if m.RefsForTest("t1") != 0 {
		t.Fatalf("refs=%d", m.RefsForTest("t1"))
	}
}

func TestEvictDisabled(t *testing.T) {
	m := NewManager(nil, nil, &proxyconfig.Config{BackendIdleTimeout: 0}, nil)
	m.InjectClientForTest("t1", nil, time.Now().Add(-time.Hour), 0)
	m.EvictIdleForTest(context.Background())
	if m.ClientCount() != 1 {
		t.Fatal("should not evict when idle timeout disabled")
	}
}
