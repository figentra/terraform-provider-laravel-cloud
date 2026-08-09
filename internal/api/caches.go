package api

import (
	"context"
	"errors"
	"fmt"
)

// Cache is a Valkey/Redis cache instance bound to one or more environments.
//
// v0.4.0 attribute expansion: added `type` (valkey/redis switch),
// `auto_upgrade_enabled`, `is_public`, `eviction_policy` — every knob the
// Cloud dashboard exposes for cache tuning.
type Cache struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	Name           string `json:"name"`
	Region         string `json:"region"`
	Size           string `json:"size"` // "valkey-pro.1gb", "valkey-pro.5gb", ...

	// Type selects the cache engine — "valkey" (default) or "redis". Added in v0.4.0.
	Type *string `json:"type,omitempty"`

	// AutoUpgradeEnabled toggles Cloud-managed engine upgrades. Added in v0.4.0.
	AutoUpgradeEnabled *bool `json:"auto_upgrade_enabled,omitempty"`

	// IsPublic exposes the cache to non-Cloud clients when true. Added in v0.4.0.
	IsPublic *bool `json:"is_public,omitempty"`

	// EvictionPolicy is Valkey/Redis eviction — "allkeys-lru", "volatile-lru",
	// "noeviction", etc. Added in v0.4.0.
	EvictionPolicy *string `json:"eviction_policy,omitempty"`

	Status    *string `json:"status,omitempty"`
	CreatedAt *string `json:"created_at,omitempty"`
	UpdatedAt *string `json:"updated_at,omitempty"`
}

// CreateCacheRequest is POST /caches.
type CreateCacheRequest struct {
	OrganizationID     string  `json:"organization_id,omitempty"`
	Name               string  `json:"name"`
	Region             string  `json:"region,omitempty"`
	Size               string  `json:"size"`
	Type               *string `json:"type,omitempty"`
	AutoUpgradeEnabled *bool   `json:"auto_upgrade_enabled,omitempty"`
	IsPublic           *bool   `json:"is_public,omitempty"`
	EvictionPolicy     *string `json:"eviction_policy,omitempty"`
}

// UpdateCacheRequest is PATCH /caches/:id — size + eviction policy + is_public
// + auto_upgrade mutable.
type UpdateCacheRequest struct {
	Size               *string `json:"size,omitempty"`
	AutoUpgradeEnabled *bool   `json:"auto_upgrade_enabled,omitempty"`
	IsPublic           *bool   `json:"is_public,omitempty"`
	EvictionPolicy     *string `json:"eviction_policy,omitempty"`
}

// CreateCache provisions a new cache.
func (c *Client) CreateCache(ctx context.Context, req CreateCacheRequest) (*Cache, error) {
	var env Envelope[Cache]
	if err := c.do(ctx, "POST", "/caches", req, &env); err != nil {
		return nil, fmt.Errorf("create cache: %w", err)
	}
	return &env.Data, nil
}

// GetCache reads a cache by ID.
func (c *Client) GetCache(ctx context.Context, id string) (*Cache, error) {
	if id == "" {
		return nil, errors.New("cache id is required")
	}
	var env Envelope[Cache]
	if err := c.do(ctx, "GET", "/caches/"+id, nil, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// UpdateCache PATCHes size only.
func (c *Client) UpdateCache(ctx context.Context, id string, req UpdateCacheRequest) (*Cache, error) {
	if id == "" {
		return nil, errors.New("cache id is required")
	}
	var env Envelope[Cache]
	if err := c.do(ctx, "PATCH", "/caches/"+id, req, &env); err != nil {
		return nil, fmt.Errorf("update cache: %w", err)
	}
	return &env.Data, nil
}

// DeleteCache destroys the cache.
func (c *Client) DeleteCache(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("cache id is required")
	}
	if err := c.do(ctx, "DELETE", "/caches/"+id, nil, nil); err != nil {
		return fmt.Errorf("delete cache: %w", err)
	}
	return nil
}
