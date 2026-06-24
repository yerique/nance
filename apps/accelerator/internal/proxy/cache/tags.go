package cache

import (
	"context"
	"fmt"
)

func tagRegistryKey(tenantID, tag string) string {
	return fmt.Sprintf("nance:tenant:{%s}:tag:%s:keys", tenantID, tag)
}

// RegisterTags associates cache keys with logical tags (best effort).
func RegisterTags(ctx context.Context, s Store, tenantID, key string, tags []string) {
	if s == nil || len(tags) == 0 {
		return
	}
	// Only MemoryStore / RedisStore implement SADD via RegisterKey pattern.
	// Use InvalidateTags path through interface extension when available.
	if rs, ok := s.(tagStore); ok {
		for _, tag := range tags {
			_ = rs.SAddTag(ctx, tenantID, tag, key)
		}
	}
}

// InvalidateTags removes all keys for the given tags.
func InvalidateTags(ctx context.Context, s Store, tenantID string, tags []string) error {
	if s == nil {
		return nil
	}
	rs, ok := s.(tagStore)
	if !ok {
		return nil
	}
	for _, tag := range tags {
		if err := rs.InvalidateTag(ctx, tenantID, tag); err != nil {
			return err
		}
	}
	return nil
}

type tagStore interface {
	SAddTag(ctx context.Context, tenantID, tag, key string) error
	InvalidateTag(ctx context.Context, tenantID, tag string) error
}

func (s *RedisStore) SAddTag(ctx context.Context, tenantID, tag, key string) error {
	cctx, cancel := context.WithTimeout(ctx, s.setTO)
	defer cancel()
	return s.client.SAdd(cctx, tagRegistryKey(tenantID, tag), key).Err()
}

func (s *RedisStore) InvalidateTag(ctx context.Context, tenantID, tag string) error {
	cctx, cancel := context.WithTimeout(ctx, s.setTO*25)
	if cctx.Err() != nil {
		// fall through — timeout was constructed from setTO
	}
	defer cancel()
	rk := tagRegistryKey(tenantID, tag)
	members, err := s.client.SMembers(cctx, rk).Result()
	if err != nil {
		return err
	}
	pipe := s.client.Pipeline()
	for _, m := range members {
		pipe.Del(cctx, m)
	}
	pipe.Del(cctx, rk)
	_, err = pipe.Exec(cctx)
	return err
}

func (m *MemoryStore) SAddTag(_ context.Context, tenantID, tag, key string) error {
	rk := tagRegistryKey(tenantID, tag)
	if m.sets[rk] == nil {
		m.sets[rk] = make(map[string]struct{})
	}
	m.sets[rk][key] = struct{}{}
	return nil
}

func (m *MemoryStore) InvalidateTag(_ context.Context, tenantID, tag string) error {
	rk := tagRegistryKey(tenantID, tag)
	for k := range m.sets[rk] {
		delete(m.mu, k)
	}
	delete(m.sets, rk)
	return nil
}
