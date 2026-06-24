package model

import "time"

// Tenant represents a customer / project that uses the accelerator.
type Tenant struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CollectionPolicy configures caching for a specific db.collection.
type CollectionPolicy struct {
	Enabled        bool `json:"enabled"`
	TTLSeconds     int  `json:"ttlSeconds"`
	MaxResultBytes *int `json:"maxResultBytes,omitempty"`
}

// CachePolicy is the declarative caching configuration for a tenant.
type CachePolicy struct {
	TenantID          string                      `json:"tenantId"`
	DefaultTtlSeconds int                         `json:"defaultTtlSeconds"`
	Collections       map[string]CollectionPolicy `json:"collections"`
	CacheKeyVersion   int                         `json:"cacheKeyVersion"`
	UpdatedAt         time.Time                   `json:"updatedAt"`
}

// Token represents an issued credential for the data plane (mongodb+nance://).
type Token struct {
	ID          string     `json:"id"`
	TenantID    string     `json:"tenantId"`
	Description string     `json:"description,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
}

// TenantBackend holds the encrypted real MongoDB connection info (never returned over API).
type TenantBackend struct {
	TenantID        string    `json:"-"`
	URICiphertext   []byte    `json:"-"`
	Nonce           []byte    `json:"-"`
	DEKVersion      string    `json:"-"`
	LastValidatedAt *time.Time `json:"-"`
	CreatedAt       time.Time `json:"-"`
	UpdatedAt       time.Time `json:"-"`
}
