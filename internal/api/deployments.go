package api

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Deployment mirrors Cloud's deployment resource — one row per `POST
// /environments/:id/deployments` firing. Terminal status is one of
// `deployment.succeeded`, `deployment.failed`, `build.failed`, `failed`,
// `cancelled`; every other status is progressing.
//
// Attribute mapping tracks the SDK's `DeploymentData` (see
// `.ref/laravel-cloud-sdk-main/src/Data/Deployments/DeploymentData.php`).
type Deployment struct {
	ID     string `json:"id"`
	Status string `json:"status"`

	BranchName    *string `json:"branch_name"`
	CommitHash    *string `json:"commit_hash"`
	CommitMessage *string `json:"commit_message"`
	CommitAuthor  *string `json:"commit_author"`
	FailureReason *string `json:"failure_reason"`

	PhpMajorVersion *string `json:"php_major_version"`
	NodeVersion     *string `json:"node_version"`
	BuildCommand    *string `json:"build_command"`

	UsesOctane      bool `json:"uses_octane"`
	UsesHibernation bool `json:"uses_hibernation"`

	StartedAt  *time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	CreatedAt  *time.Time `json:"created_at"`

	// Flattened relationships (populated by Envelope's JSON:API flatten
	// when `?include=environment,initiator` is passed). Never mandatory
	// — provider resources read the resolved `environment_id` field
	// directly.
	EnvironmentID string `json:"environment_id,omitempty"`
	InitiatorID   string `json:"initiator_id,omitempty"`
}

// IsTerminal reports whether the deployment has finished (successfully or
// otherwise). Progressing statuses return false — the poll loop keeps
// re-reading until this returns true or the caller's ctx expires.
//
// Cloud's status vocabulary spans two phases (build + deployment), so
// terminal detection matches every failing status + the two success
// statuses. Kept as a method on Deployment so callers don't hand-roll
// the match set at every poll site.
func (d Deployment) IsTerminal() bool {
	switch d.Status {
	case "deployment.succeeded",
		"deployment.failed",
		"build.failed",
		"failed",
		"cancelled":
		return true
	default:
		return false
	}
}

// IsFailure reports whether the terminal status is a failure. Callers use
// this to surface a Terraform diagnostic when the deploy exits non-happy.
func (d Deployment) IsFailure() bool {
	switch d.Status {
	case "deployment.failed",
		"build.failed",
		"failed",
		"cancelled":
		return true
	default:
		return false
	}
}

// CreateDeployment triggers a fresh deployment on an environment. Cloud
// enqueues the deploy immediately + returns the created record; the
// actual build + rollout progresses asynchronously via status updates.
//
// Wire: POST /environments/:id/deployments
// Success: HTTP 201 with the enveloped deployment record (initial status
// typically `pending` or `build.pending`).
// Failure: HTTP 422 when the environment is missing a repository binding
// or lacks a build cluster.
func (c *Client) CreateDeployment(ctx context.Context, environmentID string) (*Deployment, error) {
	if environmentID == "" {
		return nil, errors.New("environment id is required")
	}
	path := fmt.Sprintf("/environments/%s/deployments", environmentID)

	var env Envelope[Deployment]
	if err := c.do(ctx, "POST", path, struct{}{}, &env); err != nil {
		return nil, fmt.Errorf("create deployment: %w", err)
	}
	// Cloud may omit the FK in the POST response — synthesise it from the
	// URL so downstream state has a value.
	if env.Data.EnvironmentID == "" {
		env.Data.EnvironmentID = environmentID
	}
	return &env.Data, nil
}

// GetDeployment reads a deployment by ID. Passes
// `?include=environment,initiator` so the record carries the parent
// environment FK — otherwise Cloud strips it from the singleton response.
//
// Wire: GET /deployments/:id?include=environment,initiator
// Success: HTTP 200 with the enveloped record + included relationships.
// Failure: HTTP 404 when the deployment was purged (unusual — deployments
// are typically retained for the app's history retention window).
func (c *Client) GetDeployment(ctx context.Context, id string) (*Deployment, error) {
	if id == "" {
		return nil, errors.New("deployment id is required")
	}
	path := fmt.Sprintf("/deployments/%s?include=environment,initiator", id)

	var env Envelope[Deployment]
	if err := c.do(ctx, "GET", path, nil, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// PollDeployment polls Cloud until the deployment reaches a terminal
// status or the passed ctx expires. Poll cadence is fixed at 5 seconds —
// Cloud's deploys take 30-120 seconds under normal load, so a 5-second
// cadence produces ~10-25 polls per typical deploy without hammering the
// API.
//
// Returns the final terminal Deployment record + a boolean indicating
// success. On ctx expiry, returns the most recent read + false.
func (c *Client) PollDeployment(ctx context.Context, id string) (*Deployment, error) {
	const pollInterval = 5 * time.Second

	// Initial read — surface a 404 immediately so the caller can decide.
	last, err := c.GetDeployment(ctx, id)
	if err != nil {
		return nil, err
	}
	if last.IsTerminal() {
		return last, nil
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Return most recent snapshot so callers can surface partial
			// state (the deploy may still be running server-side).
			return last, fmt.Errorf("poll deployment: %w", ctx.Err())
		case <-ticker.C:
			next, err := c.GetDeployment(ctx, id)
			if err != nil {
				// Transient failures (500 / timeout) — surface the last
				// known state so the caller keeps context. Terminal
				// errors like 404 do surface.
				var apiErr *APIError
				if errors.As(err, &apiErr) && apiErr.IsNotFound() {
					return last, err
				}
				// Non-terminal read error — keep polling. Real transient
				// issues (rate limit, brief outage) recover on the next
				// tick.
				continue
			}
			last = next
			if next.IsTerminal() {
				return next, nil
			}
		}
	}
}

// DeploymentStatusHuman returns a human-friendly one-liner for terraform
// diagnostics. Cloud's status vocabulary is verbose (`deployment.succeeded`,
// `build.pending`) — this collapses to something readable in the CLI.
func DeploymentStatusHuman(status string) string {
	// Simple: replace the dot separator with " → " for readability.
	return strings.ReplaceAll(status, ".", " → ")
}
