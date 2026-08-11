package api

import (
	"context"
	"errors"
	"fmt"
)

// EnvironmentColor is the canonical enum of colors Cloud accepts for the
// per-environment visual identifier (dashboard chip color). The set is
// aligned with the Cloud dashboard's color picker menu — 20 values covering
// warm, cool, and neutral hues. Consumers pin one of these constants to
// avoid drift with Cloud's server-side validator.
//
// If Cloud accepts additional colors, add them here; the provider's
// `laravelcloud_environment.color` `stringvalidator.OneOf(ValidColors...)`
// rejects unknown values at plan-time.
type EnvironmentColor string

const (
	ColorGray    EnvironmentColor = "gray"
	ColorSlate   EnvironmentColor = "slate"
	ColorZinc    EnvironmentColor = "zinc"
	ColorRed     EnvironmentColor = "red"
	ColorRose    EnvironmentColor = "rose"
	ColorOrange  EnvironmentColor = "orange"
	ColorAmber   EnvironmentColor = "amber"
	ColorYellow  EnvironmentColor = "yellow"
	ColorLime    EnvironmentColor = "lime"
	ColorGreen   EnvironmentColor = "green"
	ColorEmerald EnvironmentColor = "emerald"
	ColorTeal    EnvironmentColor = "teal"
	ColorCyan    EnvironmentColor = "cyan"
	ColorSky     EnvironmentColor = "sky"
	ColorBlue    EnvironmentColor = "blue"
	ColorIndigo  EnvironmentColor = "indigo"
	ColorViolet  EnvironmentColor = "violet"
	ColorPurple  EnvironmentColor = "purple"
	ColorFuchsia EnvironmentColor = "fuchsia"
	ColorPink    EnvironmentColor = "pink"
)

// ValidColors is every EnvironmentColor as a bare string, in the order
// Cloud's color picker displays. Consumed by the resource schema's
// `stringvalidator.OneOf(...)` so plan-time validation names every valid
// value in a stable order.
var ValidColors = []string{
	string(ColorGray), string(ColorSlate), string(ColorZinc),
	string(ColorRed), string(ColorRose),
	string(ColorOrange), string(ColorAmber),
	string(ColorYellow), string(ColorLime),
	string(ColorGreen), string(ColorEmerald), string(ColorTeal),
	string(ColorCyan), string(ColorSky), string(ColorBlue),
	string(ColorIndigo), string(ColorViolet), string(ColorPurple),
	string(ColorFuchsia), string(ColorPink),
}

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

	// PhpMajorVersion is the READ-SIDE PHP version field — Cloud's GET
	// /environments/:id response returns `php_major_version` (e.g. "8.4").
	// Added in v0.7.0.
	//
	// Cloud's WRITE-SIDE field is DIFFERENT: `php_version` with a mandatory
	// `:1` suffix (e.g. "8.4:1"). See `UpdateEnvironmentRequest.PhpVersion`.
	// The vendor SDK (redberry/laravel-cloud-sdk `UpdateEnvironmentData::toArray()`)
	// encodes the enum's backing value + `:1` on every write; the read
	// side returns the plain major version.
	//
	// Consumers see only the plain "8.4" shape — the provider handles the
	// write-side `:1` suffix internally in the resource's Create/Update.
	PhpMajorVersion *string `json:"php_major_version,omitempty"`

	// EnvironmentVariables is Cloud's READ-SIDE env-var list, returned as
	// `environment_variables: [{key, value}, ...]` by GET /environments/:id
	// (see the SDK's `EnvironmentData::$environment_variables`). Cloud
	// SILENTLY IGNORES a `variables` map on the env root PATCH — the
	// vendor exposes a dedicated `POST /environments/:id/variables`
	// endpoint that must be called separately to mutate this list.
	// Added in v0.8.0.
	//
	// The provider resource layer at
	// `internal/provider/resource_environment.go` orchestrates the two-call
	// sequence: PATCH the env root for non-var fields, then POST /variables
	// to reconcile the env-var list against the HCL `variables` map.
	EnvironmentVariables []EnvironmentVariable `json:"environment_variables,omitempty"`

	// Visual identifier — "green"/"orange"/"red"/etc. Surfaced in Cloud
	// dashboard chip color. Nullable — Cloud picks a default when unset.
	//
	// KNOWN LIMITATION: Cloud's API silently accepts `color` on
	// PATCH but does NOT persist nor return it. The dashboard color
	// picker uses a separate (undocumented) endpoint. Every write via
	// this provider is best-effort; visible drift is expected until
	// the vendor exposes the read side. Codified in v0.6.0's schema
	// enum validator to at least reject invalid values plan-time.
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
//
// PHP-VERSION CONTRACT ASYMMETRY (v0.7.0):
//
//	Cloud's PATCH endpoint accepts the WRITE field name `php_version`
//	with a mandatory `:1` suffix appended to the semver — e.g.
//	`{"php_version": "8.4:1"}` — NOT `php_major_version`.
//
//	The suffix is the vendor's internal sub-version index; the SDK
//	(redberry/laravel-cloud-sdk `UpdateEnvironmentData::toArray()`)
//	hardcodes `:1`. Sending `php_major_version` — or `php_version`
//	without the suffix — is silently ignored (HTTP 200, no-op).
//
//	The GET response returns the plain major version under
//	`php_major_version` (see Environment.PhpMajorVersion). The
//	resource layer at `internal/provider/resource_environment.go`
//	handles the encode / decode; upstream consumers see only the
//	plain "8.4" shape.
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

	// PhpVersion is the WRITE-SIDE PHP version field — see the
	// PHP-VERSION CONTRACT ASYMMETRY comment on UpdateEnvironmentRequest
	// above. Encoded shape is `"<major>:1"` (e.g. `"8.4:1"`); the
	// resource layer builds the suffix from the plain `php_major_version`
	// HCL input before firing the PATCH.
	//
	// Read-side value lives on Environment.PhpMajorVersion — Cloud's
	// GET response returns just the major version, without the suffix.
	PhpVersion *string `json:"php_version,omitempty"`
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

// EnvironmentVariable is one `{key, value}` pair returned by GET
// /environments/:id under `environment_variables` and accepted by
// POST /environments/:id/variables under `variables`. Added in v0.8.0.
type EnvironmentVariable struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// EnvironmentVariableMethod names the merge semantics Cloud applies when
// POST /environments/:id/variables lands. Matches the SDK's
// `EnvironmentVariableMethod` enum.
type EnvironmentVariableMethod string

const (
	// EnvVarMethodAppend merges the incoming list on top of Cloud's
	// current set — same-key entries overwrite; missing keys survive.
	EnvVarMethodAppend EnvironmentVariableMethod = "append"

	// EnvVarMethodSet replaces the entire list — missing keys are
	// wiped, incoming keys become the sole set.
	EnvVarMethodSet EnvironmentVariableMethod = "set"
)

// SetEnvironmentVariablesRequest is the POST /environments/:id/variables body.
// Added in v0.8.0.
type SetEnvironmentVariablesRequest struct {
	Method    EnvironmentVariableMethod `json:"method"`
	Variables []EnvironmentVariable     `json:"variables"`
}

// DeleteEnvironmentVariablesRequest is the POST /environments/:id/variables/delete
// body. Added in v0.8.0.
type DeleteEnvironmentVariablesRequest struct {
	Keys []string `json:"keys"`
}

// SetEnvironmentVariables writes env vars to Cloud via the dedicated
// endpoint. `method` picks the merge semantics — pass EnvVarMethodAppend
// to overlay on top of Cloud's auto-generated APP_KEY etc., or
// EnvVarMethodSet to replace the entire list.
//
// Added in v0.8.0 to fix the silent-drop bug where PATCHing `variables`
// on the env root returned HTTP 200 but never persisted the map. The
// vendor SDK's `SetEnvironmentVariablesRequest` (path
// `/environments/{id}/variables`) is the authoritative write path.
func (c *Client) SetEnvironmentVariables(ctx context.Context, id string, req SetEnvironmentVariablesRequest) (*Environment, error) {
	if id == "" {
		return nil, errors.New("environment id is required")
	}
	if len(req.Variables) == 0 && req.Method == EnvVarMethodAppend {
		// No-op — nothing to append. Return current state so callers
		// see the fresh env unmodified.
		return c.GetEnvironment(ctx, id)
	}
	var env Envelope[Environment]
	path := fmt.Sprintf("/environments/%s/variables", id)
	if err := c.do(ctx, "POST", path, req, &env); err != nil {
		return nil, fmt.Errorf("set environment variables: %w", err)
	}
	return &env.Data, nil
}

// DeleteEnvironmentVariables removes a set of env var keys from Cloud
// via POST /environments/:id/variables/delete. Added in v0.8.0.
func (c *Client) DeleteEnvironmentVariables(ctx context.Context, id string, keys []string) (*Environment, error) {
	if id == "" {
		return nil, errors.New("environment id is required")
	}
	if len(keys) == 0 {
		// Nothing to delete — return current state.
		return c.GetEnvironment(ctx, id)
	}
	var env Envelope[Environment]
	path := fmt.Sprintf("/environments/%s/variables/delete", id)
	if err := c.do(ctx, "POST", path, DeleteEnvironmentVariablesRequest{Keys: keys}, &env); err != nil {
		return nil, fmt.Errorf("delete environment variables: %w", err)
	}
	return &env.Data, nil
}
