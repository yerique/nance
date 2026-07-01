package cache

import (
	"context"
	"testing"
)

func TestMemoryStore_TagsInvalidate(t *testing.T) {
	m := NewMemoryStore()
	ctx := context.Background()
	key := "k1"
	_ = m.Set(ctx, key, []byte("x"), 0)
	_ = m.SAddTag(ctx, "t1", "tagA", key)
	if err := m.InvalidateTag(ctx, "t1", "tagA"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Get(ctx, key); err != ErrMiss {
		t.Fatalf("want miss got %v", err)
	}
}

func TestRegisterAndInvalidateTags(t *testing.T) {
	m := NewMemoryStore()
	ctx := context.Background()
	RegisterTags(ctx, m, "ten", "key1", []string{"a"})
	_ = m.Set(ctx, "key1", []byte("v"), 0)
	if err := InvalidateTags(ctx, m, "ten", []string{"a"}); err != nil {
		t.Fatal(err)
	}
}
