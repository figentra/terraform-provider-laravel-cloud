package api

import (
	"context"
	"errors"
	"fmt"
)

// DatabaseCluster is a shared Postgres or MySQL cluster hosting one or more
// per-service schemas.
//
// v0.4.0 attribute expansion: added `Type` (Cloud's v2 cluster-type slug
// such as `neon_serverless_postgres_18`) + `Config` (a free-form map of
// type-specific tuning knobs — cu_min, cu_max, suspend_seconds, ...) so
// consumers on Cloud v2 can provision the newer serverless clusters. The
// pre-v0.4 (`Engine` + `Size` + `HighAvailability`) fields are kept as
// Optional/Computed for backward compatibility.
type DatabaseCluster struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	Name           string `json:"name"`
	Region         string `json:"region"`

	// Type is the Cloud v2 cluster type slug — e.g.
	// `neon_serverless_postgres_18`, `postgres_16`, `mysql_8`. Added
	// in v0.4.0.
	Type *string `json:"type,omitempty"`

	// Config is the free-form v2 tuning bag — cu_min, cu_max,
	// suspend_seconds, retention_days, etc. Added in v0.4.0.
	Config map[string]any `json:"config,omitempty"`

	// Pre-v0.4 attributes (kept for backward compat)
	Engine              string `json:"engine,omitempty"`
	Size                string `json:"size,omitempty"`
	HighAvailability    bool   `json:"high_availability,omitempty"`
	BackupRetentionDays int    `json:"backup_retention_days,omitempty"`

	Status    *string `json:"status,omitempty"`
	CreatedAt *string `json:"created_at,omitempty"`
	UpdatedAt *string `json:"updated_at,omitempty"`
}

// CreateDatabaseClusterRequest is the POST /databases/clusters body.
type CreateDatabaseClusterRequest struct {
	OrganizationID string `json:"organization_id,omitempty"`
	Name           string `json:"name"`
	Region         string `json:"region,omitempty"`

	// v0.4 canonical
	Type   *string        `json:"type,omitempty"`
	Config map[string]any `json:"config,omitempty"`

	// pre-v0.4
	Engine              string `json:"engine,omitempty"`
	Size                string `json:"size,omitempty"`
	HighAvailability    bool   `json:"high_availability,omitempty"`
	BackupRetentionDays int    `json:"backup_retention_days,omitempty"`
}

// UpdateDatabaseClusterRequest is PATCH /databases/clusters/:id.
type UpdateDatabaseClusterRequest struct {
	Name                *string        `json:"name,omitempty"`
	Config              map[string]any `json:"config,omitempty"`
	Size                *string        `json:"size,omitempty"`
	HighAvailability    *bool          `json:"high_availability,omitempty"`
	BackupRetentionDays *int           `json:"backup_retention_days,omitempty"`
}

// CreateDatabaseCluster provisions a new shared database cluster.
func (c *Client) CreateDatabaseCluster(ctx context.Context, req CreateDatabaseClusterRequest) (*DatabaseCluster, error) {
	var env Envelope[DatabaseCluster]
	if err := c.do(ctx, "POST", "/databases/clusters", req, &env); err != nil {
		return nil, fmt.Errorf("create database cluster: %w", err)
	}
	return &env.Data, nil
}

// GetDatabaseCluster reads by ID.
func (c *Client) GetDatabaseCluster(ctx context.Context, id string) (*DatabaseCluster, error) {
	if id == "" {
		return nil, errors.New("cluster id is required")
	}
	var env Envelope[DatabaseCluster]
	if err := c.do(ctx, "GET", "/databases/clusters/"+id, nil, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// UpdateDatabaseCluster PATCHes mutable fields (size, ha, backup retention).
func (c *Client) UpdateDatabaseCluster(ctx context.Context, id string, req UpdateDatabaseClusterRequest) (*DatabaseCluster, error) {
	if id == "" {
		return nil, errors.New("cluster id is required")
	}
	var env Envelope[DatabaseCluster]
	if err := c.do(ctx, "PATCH", "/databases/clusters/"+id, req, &env); err != nil {
		return nil, fmt.Errorf("update database cluster: %w", err)
	}
	return &env.Data, nil
}

// DeleteDatabaseCluster destroys the cluster. Cloud rejects with 409 when
// schemas still exist — Terraform's dependency graph should destroy schemas
// first.
func (c *Client) DeleteDatabaseCluster(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("cluster id is required")
	}
	if err := c.do(ctx, "DELETE", "/databases/clusters/"+id, nil, nil); err != nil {
		return fmt.Errorf("delete database cluster: %w", err)
	}
	return nil
}
