package api

import (
	"context"
	"errors"
	"fmt"
	"time"
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

// DeleteDatabaseCluster destroys the cluster. Cloud rejects with 422
// ("The database cluster has schemas attached and cannot be deleted.")
// when schemas remain — including schemas that live only on Cloud (Cloud-UI-
// authored, orphaned from stale terraform state).
//
// Cascade behaviour: before firing the cluster DELETE, list every schema
// attached to the cluster and delete each. Fail-open per-schema — a
// 404 on a single schema (already gone) is not fatal. Any other error
// propagates so the operator can inspect.
//
// Rationale: terraform's dependency graph destroys child-module schemas
// before the parent cluster, so the "clean" case never fires the cascade
// (list returns empty). The cascade matters in TWO scenarios:
//
//  1. Stale terraform state — a Cloud-UI-authored schema or an aborted
//     destroy that removed the schema from state but not from Cloud.
//
//  2. Cloud's eventual-consistency race — the DAG destroys child
//     schemas, then fires the cluster DELETE. The schema DELETE
//     endpoint returns 200 immediately, but Cloud's internal
//     `cluster.has_schemas` guard lags by a few seconds. The cluster
//     DELETE hits 422 even though our list said "0 schemas" a moment
//     earlier.
//
// Retry loop handles both: after each 422, drain lingering schemas +
// wait for Cloud to reconcile, then retry the cluster DELETE. Bounded
// at 5 attempts with exponential backoff (500ms → 8s cap, ≈ 15s
// total worst-case) so a genuinely-broken Cloud state fails loudly
// instead of stalling the plan forever.
func (c *Client) DeleteDatabaseCluster(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("cluster id is required")
	}

	const maxAttempts = 5
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Reap any lingering schemas Cloud still knows about.
		if drainErr := c.drainSchemas(ctx, id); drainErr != nil {
			return drainErr
		}

		if err := c.do(ctx, "DELETE", "/databases/clusters/"+id, nil, nil); err != nil {
			var apiErr *APIError
			// Race: Cloud still reports schemas attached even though
			// our drainSchemas call reported success. Back off, drain
			// again, retry.
			if errors.As(err, &apiErr) && apiErr.IsSchemasAttached() && attempt < maxAttempts {
				lastErr = err
				time.Sleep(backoff(attempt))
				continue
			}
			// 404 is idempotent — cluster already gone from a
			// previous partially-successful destroy.
			if errors.As(err, &apiErr) && apiErr.IsNotFound() {
				return nil
			}
			return fmt.Errorf("delete database cluster: %w", err)
		}
		return nil
	}
	return fmt.Errorf("delete database cluster: exhausted %d attempts, last error: %w", maxAttempts, lastErr)
}

// drainSchemas lists every schema attached to a cluster and deletes
// each. Fail-open per-schema on 404 (already gone). Surfaces any
// other list/delete error to the caller.
func (c *Client) drainSchemas(ctx context.Context, clusterID string) error {
	schemas, err := c.ListDatabaseSchemas(ctx, clusterID)
	if err != nil {
		// Cluster may already be half-deleted or the list endpoint may
		// 404 mid-cluster-teardown — treat 404 as "nothing to drain".
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return nil
		}
		return fmt.Errorf("cascade list schemas before cluster delete: %w", err)
	}
	for _, s := range schemas {
		if delErr := c.DeleteDatabaseSchema(ctx, clusterID, s.ID); delErr != nil {
			var apiErr *APIError
			if errors.As(delErr, &apiErr) && apiErr.IsNotFound() {
				continue // already gone; skip
			}
			return fmt.Errorf("cascade delete schema %s: %w", s.ID, delErr)
		}
	}
	return nil
}
