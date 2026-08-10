package api

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// BackgroundProcessConfig mirrors Laravel's queue-worker knobs. Every
// field is a pointer so partial updates work without overwriting other
// tuning. Semantic values match `php artisan queue:work` flag names.
type BackgroundProcessConfig struct {
	Connection *string `json:"connection,omitempty"`
	Queue      *string `json:"queue,omitempty"`
	Tries      *int    `json:"tries,omitempty"`
	Backoff    *int    `json:"backoff,omitempty"`
	Sleep      *int    `json:"sleep,omitempty"`
	Rest       *int    `json:"rest,omitempty"`
	Timeout    *int    `json:"timeout,omitempty"`
	Force      *bool   `json:"force,omitempty"`
}

// BackgroundProcess is a daemon running on an Instance. Two types:
// `worker` (Laravel queue worker → Horizon-managed) or `custom` (any
// long-lived process; requires the `command` field).
type BackgroundProcess struct {
	ID         string                    `json:"id"`
	InstanceID string                    `json:"instance_id,omitempty"`
	Type       string                    `json:"type"`
	Processes  int                       `json:"processes"`
	Command    *string                   `json:"command"`
	Config     *BackgroundProcessConfig  `json:"config"`

	StrategyType      *string `json:"strategy_type,omitempty"`
	StrategyThreshold *int    `json:"strategy_threshold,omitempty"`

	CreatedAt *time.Time `json:"created_at,omitempty"`
}

// CreateBackgroundProcessRequest is the POST /instances/:id/background-processes body.
type CreateBackgroundProcessRequest struct {
	Type      string                   `json:"type"`
	Processes int                      `json:"processes"`
	Command   *string                  `json:"command,omitempty"`
	Config    *BackgroundProcessConfig `json:"config,omitempty"`
}

// UpdateBackgroundProcessRequest is the PATCH /background-processes/:id body.
type UpdateBackgroundProcessRequest struct {
	Type      *string                  `json:"type,omitempty"`
	Processes *int                     `json:"processes,omitempty"`
	Command   *string                  `json:"command,omitempty"`
	Config    *BackgroundProcessConfig `json:"config,omitempty"`
}

// CreateBackgroundProcess provisions a daemon on an Instance.
//
// Wire: POST /instances/:id/background-processes
// Success: HTTP 201.
func (c *Client) CreateBackgroundProcess(ctx context.Context, instanceID string, req CreateBackgroundProcessRequest) (*BackgroundProcess, error) {
	if instanceID == "" {
		return nil, errors.New("instance id is required")
	}
	path := fmt.Sprintf("/instances/%s/background-processes", instanceID)

	var env Envelope[BackgroundProcess]
	if err := c.do(ctx, "POST", path, req, &env); err != nil {
		return nil, fmt.Errorf("create background process: %w", err)
	}
	if env.Data.InstanceID == "" {
		env.Data.InstanceID = instanceID
	}
	return &env.Data, nil
}

// GetBackgroundProcess reads by ID.
//
// Wire: GET /background-processes/:id
// Failure: HTTP 404 when the daemon was deleted out-of-band.
func (c *Client) GetBackgroundProcess(ctx context.Context, id string) (*BackgroundProcess, error) {
	if id == "" {
		return nil, errors.New("background process id is required")
	}
	path := fmt.Sprintf("/background-processes/%s?include=instance", id)

	var env Envelope[BackgroundProcess]
	if err := c.do(ctx, "GET", path, nil, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// UpdateBackgroundProcess PATCHes mutable fields.
//
// Wire: PATCH /background-processes/:id
func (c *Client) UpdateBackgroundProcess(ctx context.Context, id string, req UpdateBackgroundProcessRequest) (*BackgroundProcess, error) {
	if id == "" {
		return nil, errors.New("background process id is required")
	}
	path := fmt.Sprintf("/background-processes/%s", id)

	var env Envelope[BackgroundProcess]
	if err := c.do(ctx, "PATCH", path, req, &env); err != nil {
		return nil, fmt.Errorf("update background process: %w", err)
	}
	return &env.Data, nil
}

// DeleteBackgroundProcess removes a daemon.
//
// Wire: DELETE /background-processes/:id
func (c *Client) DeleteBackgroundProcess(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("background process id is required")
	}
	path := fmt.Sprintf("/background-processes/%s", id)

	if err := c.do(ctx, "DELETE", path, nil, nil); err != nil {
		return fmt.Errorf("delete background process: %w", err)
	}
	return nil
}
