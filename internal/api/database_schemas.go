package api

import (
	"context"
	"errors"
	"fmt"
)

// DatabaseSchema is a logical database inside a DatabaseCluster.
type DatabaseSchema struct {
	ID        string  `json:"id"`
	ClusterID string  `json:"cluster_id"`
	Name      string  `json:"name"`
	Status    *string `json:"status"`
	CreatedAt *string `json:"created_at"`
}

// CreateDatabaseSchemaRequest is the POST /databases/clusters/:clusterId/databases body.
type CreateDatabaseSchemaRequest struct {
	Name string `json:"name"`
}

// CreateDatabaseSchema provisions a schema in a cluster.
func (c *Client) CreateDatabaseSchema(ctx context.Context, clusterID string, req CreateDatabaseSchemaRequest) (*DatabaseSchema, error) {
	if clusterID == "" {
		return nil, errors.New("cluster id is required")
	}
	path := fmt.Sprintf("/databases/clusters/%s/databases", clusterID)

	var env Envelope[DatabaseSchema]
	if err := c.do(ctx, "POST", path, req, &env); err != nil {
		return nil, fmt.Errorf("create database schema: %w", err)
	}
	return &env.Data, nil
}

// GetDatabaseSchema reads a schema by ID.
func (c *Client) GetDatabaseSchema(ctx context.Context, id string) (*DatabaseSchema, error) {
	if id == "" {
		return nil, errors.New("schema id is required")
	}
	var env Envelope[DatabaseSchema]
	if err := c.do(ctx, "GET", "/databases/"+id, nil, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// DeleteDatabaseSchema drops a schema. Schemas are immutable post-create —
// no PATCH endpoint.
func (c *Client) DeleteDatabaseSchema(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("schema id is required")
	}
	if err := c.do(ctx, "DELETE", "/databases/"+id, nil, nil); err != nil {
		return fmt.Errorf("delete database schema: %w", err)
	}
	return nil
}
