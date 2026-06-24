package cache

import (
	"context"
	"testing"
)

func TestGenerationBump(t *testing.T) {
	g := NewGenerationTracker(nil)
	ctx := context.Background()
	if g.Get(ctx, "t", "d", "c") != 0 {
		t.Fatal()
	}
	n := g.Bump(ctx, "t", "d", "c")
	if n != 1 || g.KeySuffix(ctx, "t", "d", "c") != "g1" {
		t.Fatalf("n=%d suf=%s", n, g.KeySuffix(ctx, "t", "d", "c"))
	}
}
