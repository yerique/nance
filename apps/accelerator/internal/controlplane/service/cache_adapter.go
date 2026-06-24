package service

import (
	"context"

	"github.com/taeven/nance/accelerator/internal/proxy/cache"
)

// RedisInvalidator adapts proxy/cache.Store for control-plane explicit invalidation.
type RedisInvalidator struct {
	Store cache.Store
}

func (r *RedisInvalidator) InvalidateNamespace(ctx context.Context, tenantID, db, coll string) error {
	if r == nil || r.Store == nil {
		return nil
	}
	return r.Store.InvalidateNamespace(ctx, tenantID, db, coll)
}

func (r *RedisInvalidator) InvalidateTags(ctx context.Context, tenantID string, tags []string) error {
	if r == nil || r.Store == nil {
		return nil
	}
	return cache.InvalidateTags(ctx, r.Store, tenantID, tags)
}
