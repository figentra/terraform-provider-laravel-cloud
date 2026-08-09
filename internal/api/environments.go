package api

import (
	"context"
	"errors"
	"fmt"
)

// Environment represents a Cloud application environment (dev / stg / prd
// or a per-PR preview-*). Every environment carries a branch binding, an
// env-var map, and links to database schema / cache / websocket app /
// domains.
//
// v0.4.0 attribute expansion: added deploy runtime knobs (node_version,
// build_command, deploy_command), push-to-deploy toggles
// (uses_push_to_deploy, uses_deploy_hook, uses_octane, uses_hibernation),
// visual color, VanityDomain (Cloud-generated *.laravel.cloud fallback
// hostname), and the FK bindings for database / cache / websocket-app.
type Environment struct {
	ID                     string            `json:"id"`
	ApplicationID          string            `json:"application_id"`
	Name                   string            `json:"name"`
	Branch                 *string           `json:"branch,omitempty"`
	Variables              map[string]string `json:"variables,omitempty"`
	DatabaseSchemaID       *string           `json:"database_schema_id,omitempty"`
	CacheID                *string           `json:"cache_id,omitempty"`
	WebsocketApplicationID *string           `json:"websocket_application_id,omitempty"`
	InheritsID             *string           `json:"inherits_id,omitempty"`

	// Runtime + build config (v0.4.0)
	NodeVersion   *string `json:"node_version,omitempty"`
	BuildCommand  *string `json:"build_command,omitempty"`
	DeployCommand *string `json:"deploy_command,omitempty"`

	// Deploy toggles (v0.4.0)
	UsesPushToDeploy *bool `json:"uses_push_to_deploy,omitempty"`
	UsesDeployHook   *bool `json:"uses_deploy_hook,omitempty"`
	UsesOctane       *bool `json:"uses_octane,omitempty"`
	UsesHibernation  *bool `json:"uses_hibernation,omitempty"`

	// Visual identifier — "green"/"orange"/"red"/etc. Surfaced in Cloud
	// dashboard chip color. Nullable — Cloud picks a default when unset.
	Color *string `json:"color,omitempty"`

	// VanityDomain is the Cloud-generated `<app>-<env>.laravel.cloud`
	// fallback hostname. Read-only. Added in v0.4.0.
	VanityDomain *string `json:"vanity_domain,omitempty"`

	CreatedAt *string `json:"created_at,omitempty"`
	UpdatedAt *string `json:"updated_at,omitempty"`
}

// CreateEnvironmentRequest is the POST /applications/:id/environments body.
type CreateEnvironmentRequest struct {
	Name                   string            `json:"name"`
	Branch                 *string           `json:"branch,omitempty"`
	Variables              map[string]string `json:"variables,omitempty"`
	InheritsID             *string           `json:"inherits_id,omitempty"`
	DatabaseSchemaID       *string           `json:"database_schema_id,omitempty"`
	CacheID                *string           `json:"cache_id,omitempty"`
	WebsocketApplicationID *string           `json:"websocket_application_id,omitempty"`

	NodeVersion   *string `json:"node_version,omitempty"`
	BuildCommand  *string `json:"build_command,omitempty"`
	DeployCommand *string `json:"deploy_command,omitempty"`

	UsesPushToDeploy *bool `json:"uses_push_to_deploy,omitempty"`
	UsesDeployHook   *bool `json:"uses_deploy_hook,omitempty"`
	UsesOctane       *bool `json:"uses_octane,omitempty"`
	UsesHibernation  *bool `json:"uses_hibernation,omitempty"`

	Color *string `json:"color,omitempty"`
}

// UpdateEnvironmentRequest is PATCH /environments/:id — partial update.
// Every field is a pointer so operators can partial-update without wiping
// unset fields.
type UpdateEnvironmentRequest struct {
	Branch                 *string           `json:"branch,omitempty"`
	Variables              map[string]string `json:"variables,omitempty"`
	DatabaseSchemaID       *string           `json:"database_schema_id,omitempty"`
	CacheID                *string           `json:"cache_id,omitempty"`
	WebsocketApplicationID *string           `json:"websocket_application_id,omitempty"`

	NodeVersion   *string `json:"node_version,omitempty"`
	BuildCommand  *string `json:"build_command,omitempty"`
	DeployCommand *string `json:"deploy_command,omitempty"`

	UsesPushToDeploy *bool `json:"uses_push_to_deploy,omitempty"`
	UsesDeployHook   *bool `json:"uses_deploy_hook,omitempty"`
	UsesOctane       *bool `json:"uses_octane,omitempty"`
	UsesHibernation  *bool `json:"uses_hibernation,omitempty"`

	Color *string `json:"color,omitempty"`
}

// CreateEnvironment provisions a new environment under an application.
func (c *Client) CreateEnvironment(ctx context.Context, applicationID string, req CreateEnvironmentRequest) (*Environment, error) {
	if applicationID == "" {
		return nil, errors.New("application id is required")
	}
	path := fmt.Sprintf("/applications/%s/environments", applicationID)

	var env Envelope[Environment]
	if err := c.do(ctx, "POST", path, req, &env); err != nil {
		return nil, fmt.Errorf("create environment: %w", err)
	}
	return &env.Data, nil
}

// GetEnvironment reads an environment by ID.
func (c *Client) GetEnvironment(ctx context.Context, id string) (*Environment, error) {
	if id == "" {
		return nil, errors.New("environment id is required")
	}
	var env Envelope[Environment]
	if err := c.do(ctx, "GET", "/environments/"+id, nil, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// UpdateEnvironment PATCHes mutable fields.
func (c *Client) UpdateEnvironment(ctx context.Context, id string, req UpdateEnvironmentRequest) (*Environment, error) {
	if id == "" {
		return nil, errors.New("environment id is required")
	}
	var env Envelope[Environment]
	if err := c.do(ctx, "PATCH", "/environments/"+id, req, &env); err != nil {
		return nil, fmt.Errorf("update environment: %w", err)
	}
	return &env.Data, nil
}

// DeleteEnvironment tears down an environment.
func (c *Client) DeleteEnvironment(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("environment id is required")
	}
	if err := c.do(ctx, "DELETE", "/environments/"+id, nil, nil); err != nil {
		return fmt.Errorf("delete environment: %w", err)
	}
	return nil
}
