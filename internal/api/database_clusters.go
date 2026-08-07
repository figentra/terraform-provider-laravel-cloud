package api

import (
	"context"
	"errors"
	"fmt"
)

// DatabaseCluster is a shared Postgres or MySQL cluster hosting one or more
// per-service schemas.
type DatabaseCluster struct {
	ID                  string  `json:"id"`
	OrganizationID      string  `json:"organization_id"`
	Name                string  `json:"name"`
	Region              string  `json:"region"`
	Engine              string  `json:"engine"` // "postgres-16", "mysql-8", ...
	Size                string  `json:"size"`
	HighAvailability    bool    `json:"high_availability"`
	BackupRetentionDays int     `json:"backup_retention_days"`
	Status              *string `json:"status"`
	CreatedAt           *string `json:"created_at"`
	UpdatedAt           *string `json:"updated_at"`
}

// CreateDatabaseClusterRequest is the POST /databases/clusters body.
type CreateDatabaseClusterRequest struct {
	OrganizationID      string `json:"organization_id"`
	Name                string `json:"name"`
	Region              string `json:"region"`
	Engine              string `json:"engine"`
	Size                string `json:"size"`
	HighAvailability    bool   `json:"high_availability"`
	BackupRetentionDays int    `json:"backup_retention_days,omitempty"`
}

// UpdateDatabaseClusterRequest is PATCH /databases/clusters/:id.
type UpdateDatabaseClusterRequest struct {
	Name                *string `json:"name,omitempty"`
	Size                *string `json:"size,omitempty"`
	HighAvailability    *bool   `json:"high_availability,omitempty"`
	BackupRetentionDays *int    `json:"backup_retention_days,omitempty"`
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
