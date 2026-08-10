package api

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// DatabaseSnapshot is one point-in-time backup on a database cluster.
// Two flavours: `manual` (operator-created) and `automated` (Cloud's
// scheduled snapshots). Terraform manages `manual` snapshots; the
// `automated` ones are visible via the list data source.
type DatabaseSnapshot struct {
	ID          string  `json:"id"`
	ClusterID   string  `json:"cluster_id,omitempty"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	Type        string  `json:"type"`
	Status      string  `json:"status"`

	StorageBytes *int  `json:"storage_bytes,omitempty"`
	PitrEnabled  *bool `json:"pitr_enabled,omitempty"`

	PitrEndsAt  *time.Time `json:"pitr_ends_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
}

// CreateDatabaseSnapshotRequest is the POST body.
type CreateDatabaseSnapshotRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

// CreateDatabaseSnapshot triggers a manual snapshot on a cluster.
//
// Wire: POST /databases/clusters/:id/snapshots
// Success: HTTP 201.
func (c *Client) CreateDatabaseSnapshot(ctx context.Context, clusterID string, req CreateDatabaseSnapshotRequest) (*DatabaseSnapshot, error) {
	if clusterID == "" {
		return nil, errors.New("cluster id is required")
	}
	path := fmt.Sprintf("/databases/clusters/%s/snapshots", clusterID)

	var env Envelope[DatabaseSnapshot]
	if err := c.do(ctx, "POST", path, req, &env); err != nil {
		return nil, fmt.Errorf("create database snapshot: %w", err)
	}
	if env.Data.ClusterID == "" {
		env.Data.ClusterID = clusterID
	}
	return &env.Data, nil
}

// GetDatabaseSnapshot reads a snapshot by ID.
//
// Wire: GET /databases/clusters/:clusterId/snapshots/:id
func (c *Client) GetDatabaseSnapshot(ctx context.Context, clusterID, id string) (*DatabaseSnapshot, error) {
	if clusterID == "" {
		return nil, errors.New("cluster id is required")
	}
	if id == "" {
		return nil, errors.New("snapshot id is required")
	}
	path := fmt.Sprintf("/databases/clusters/%s/snapshots/%s", clusterID, id)

	var env Envelope[DatabaseSnapshot]
	if err := c.do(ctx, "GET", path, nil, &env); err != nil {
		return nil, err
	}
	if env.Data.ClusterID == "" {
		env.Data.ClusterID = clusterID
	}
	return &env.Data, nil
}

// DeleteDatabaseSnapshot removes a snapshot.
//
// Wire: DELETE /databases/clusters/:clusterId/snapshots/:id
func (c *Client) DeleteDatabaseSnapshot(ctx context.Context, clusterID, id string) error {
	if clusterID == "" {
		return errors.New("cluster id is required")
	}
	if id == "" {
		return errors.New("snapshot id is required")
	}
	path := fmt.Sprintf("/databases/clusters/%s/snapshots/%s", clusterID, id)

	if err := c.do(ctx, "DELETE", path, nil, nil); err != nil {
		return fmt.Errorf("delete database snapshot: %w", err)
	}
	return nil
}

// ListDatabaseSnapshots returns every snapshot attached to a cluster.
// Used by the list data source.
//
// Wire: GET /databases/clusters/:id/snapshots
func (c *Client) ListDatabaseSnapshots(ctx context.Context, clusterID string) ([]DatabaseSnapshot, error) {
	if clusterID == "" {
		return nil, errors.New("cluster id is required")
	}
	path := fmt.Sprintf("/databases/clusters/%s/snapshots", clusterID)

	var env Envelope[[]DatabaseSnapshot]
	if err := c.do(ctx, "GET", path, nil, &env); err != nil {
		return nil, fmt.Errorf("list database snapshots: %w", err)
	}
	return env.Data, nil
}
