package api

import (
	"context"
	"errors"
	"fmt"
)

// Cache is a Valkey/Redis cache instance bound to one or more environments.
type Cache struct {
	ID             string  `json:"id"`
	OrganizationID string  `json:"organization_id"`
	Name           string  `json:"name"`
	Region         string  `json:"region"`
	Size           string  `json:"size"` // "valkey-pro.1gb", "valkey-pro.5gb", ...
	Status         *string `json:"status"`
	CreatedAt      *string `json:"created_at"`
}

// CreateCacheRequest is POST /caches.
type CreateCacheRequest struct {
	OrganizationID string `json:"organization_id"`
	Name           string `json:"name"`
	Region         string `json:"region"`
	Size           string `json:"size"`
}

// UpdateCacheRequest is PATCH /caches/:id — only size is mutable.
type UpdateCacheRequest struct {
	Size *string `json:"size,omitempty"`
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
