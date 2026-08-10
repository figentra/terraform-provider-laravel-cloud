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

// GetDatabaseSchema reads a schema by ID. Cloud's schema endpoints are
// cluster-scoped — the bare `/databases/<id>` path 404s ("No query
// results for model [App\Models\Database]") because Cloud routes
// database schema lookups through the cluster hierarchy. The caller
// MUST supply the cluster ID.
//
// See DeleteDatabaseSchema for the matching path shape.
func (c *Client) GetDatabaseSchema(ctx context.Context, clusterID, id string) (*DatabaseSchema, error) {
	if clusterID == "" {
		return nil, errors.New("cluster id is required")
	}
	if id == "" {
		return nil, errors.New("schema id is required")
	}
	path := fmt.Sprintf("/databases/clusters/%s/databases/%s", clusterID, id)
	var env Envelope[DatabaseSchema]
	if err := c.do(ctx, "GET", path, nil, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// DeleteDatabaseSchema drops a schema. Schemas are immutable post-create —
// no PATCH endpoint. Same cluster-scoped path shape as GetDatabaseSchema.
func (c *Client) DeleteDatabaseSchema(ctx context.Context, clusterID, id string) error {
	if clusterID == "" {
		return errors.New("cluster id is required")
	}
	if id == "" {
		return errors.New("schema id is required")
	}
	path := fmt.Sprintf("/databases/clusters/%s/databases/%s", clusterID, id)
	if err := c.do(ctx, "DELETE", path, nil, nil); err != nil {
		return fmt.Errorf("delete database schema: %w", err)
	}
	return nil
}
