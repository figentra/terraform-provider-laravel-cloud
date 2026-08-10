package api

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Command is one artisan / shell command execution inside an env. Cloud
// runs the command inside a fresh short-lived container attached to the
// env's database + cache + secrets. Common uses: `migrate --force`,
// `db:seed`, `tinker`, `cache:clear`.
//
// Terminal statuses match Cloud's CommandStatus enum — the SDK enum lives
// in `.ref/laravel-cloud-sdk-main/src/Enums/CommandStatus.php`. Reproduced
// here as method literals rather than an enum type because Go doesn't
// need the ceremony.
type Command struct {
	ID            string  `json:"id"`
	Command       string  `json:"command"`
	Output        *string `json:"output,omitempty"`
	Status        string  `json:"status"`
	ExitCode      *int    `json:"exit_code,omitempty"`
	FailureReason *string `json:"failure_reason,omitempty"`

	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	CreatedAt  *time.Time `json:"created_at,omitempty"`

	EnvironmentID string `json:"environment_id,omitempty"`
	DeploymentID  string `json:"deployment_id,omitempty"`
}

// IsTerminal reports whether the command finished.
func (c Command) IsTerminal() bool {
	switch c.Status {
	case "completed", "failed", "cancelled", "timeout":
		return true
	default:
		return false
	}
}

// IsFailure reports whether the terminal status is a failure.
func (c Command) IsFailure() bool {
	switch c.Status {
	case "failed", "cancelled", "timeout":
		return true
	default:
		return false
	}
}

// RunCommandRequest is the POST body.
type RunCommandRequest struct {
	Command string `json:"command"`
}

// RunCommand fires a command on an env.
//
// Wire: POST /environments/:id/commands
// Success: HTTP 201 with the enqueued command record (status typically
// `pending` or `running`).
func (c *Client) RunCommand(ctx context.Context, environmentID string, req RunCommandRequest) (*Command, error) {
	if environmentID == "" {
		return nil, errors.New("environment id is required")
	}
	path := fmt.Sprintf("/environments/%s/commands", environmentID)

	var env Envelope[Command]
	if err := c.do(ctx, "POST", path, req, &env); err != nil {
		return nil, fmt.Errorf("run command: %w", err)
	}
	if env.Data.EnvironmentID == "" {
		env.Data.EnvironmentID = environmentID
	}
	return &env.Data, nil
}

// GetCommand reads a command by ID.
//
// Wire: GET /commands/:id?include=environment,deployment,initiator
func (c *Client) GetCommand(ctx context.Context, id string) (*Command, error) {
	if id == "" {
		return nil, errors.New("command id is required")
	}
	path := fmt.Sprintf("/commands/%s?include=environment,deployment,initiator", id)

	var env Envelope[Command]
	if err := c.do(ctx, "GET", path, nil, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// PollCommand polls until terminal status. Same shape as PollDeployment.
func (c *Client) PollCommand(ctx context.Context, id string) (*Command, error) {
	const pollInterval = 3 * time.Second

	last, err := c.GetCommand(ctx, id)
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
			return last, fmt.Errorf("poll command: %w", ctx.Err())
		case <-ticker.C:
			next, err := c.GetCommand(ctx, id)
			if err != nil {
				var apiErr *APIError
				if errors.As(err, &apiErr) && apiErr.IsNotFound() {
					return last, err
				}
				continue
			}
			last = next
			if next.IsTerminal() {
				return next, nil
			}
		}
	}
}
