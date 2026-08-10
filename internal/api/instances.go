package api

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Instance is one Cloud compute unit. An environment carries at least
// one Instance (type `app`) plus optionally a `queue` / `service`
// Instance for horizon workers, custom services, or a serverless-queue
// runtime. Every Instance is sized by an `InstanceSize` slug (see the
// SDK's InstanceSize enum for the canonical set) + carries autoscaling
// + Octane + hibernation toggles.
type Instance struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Type            string  `json:"type"`
	Size            string  `json:"size"`
	ScalingType     string  `json:"scaling_type"`
	MinReplicas     int     `json:"min_replicas"`
	MaxReplicas     int     `json:"max_replicas"`
	UsesScheduler   bool    `json:"uses_scheduler"`
	UsesSleepMode   bool    `json:"uses_sleep_mode"`
	SleepTimeout    *int    `json:"sleep_timeout,omitempty"`
	UsesOctane      bool    `json:"uses_octane"`
	UsesInertiaSsr  bool    `json:"uses_inertia_ssr"`

	// Nullable — Cloud omits these when scalingType != auto/custom.
	ScalingCpuThresholdPercentage    *int `json:"scaling_cpu_threshold_percentage"`
	ScalingMemoryThresholdPercentage *int `json:"scaling_memory_threshold_percentage"`

	CreatedAt *time.Time `json:"created_at,omitempty"`

	// Flattened relationships when the caller passes `?include=environment`.
	EnvironmentID string `json:"environment_id,omitempty"`
}

// CreateInstanceRequest is the POST /environments/:envId/instances body.
type CreateInstanceRequest struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Size        string `json:"size"`
	ScalingType string `json:"scaling_type"`
	MinReplicas int    `json:"min_replicas"`
	MaxReplicas int    `json:"max_replicas"`

	UsesScheduler                    *bool `json:"uses_scheduler,omitempty"`
	ScalingCpuThresholdPercentage    *int  `json:"scaling_cpu_threshold_percentage,omitempty"`
	ScalingMemoryThresholdPercentage *int  `json:"scaling_memory_threshold_percentage,omitempty"`
}

// UpdateInstanceRequest is the PATCH /instances/:id body. Every field
// is a pointer so operators can partial-update without wiping other
// fields.
type UpdateInstanceRequest struct {
	Name                             *string `json:"name,omitempty"`
	Size                             *string `json:"size,omitempty"`
	ScalingType                      *string `json:"scaling_type,omitempty"`
	MinReplicas                      *int    `json:"min_replicas,omitempty"`
	MaxReplicas                      *int    `json:"max_replicas,omitempty"`
	UsesSleepMode                    *bool   `json:"uses_sleep_mode,omitempty"`
	SleepTimeout                     *int    `json:"sleep_timeout,omitempty"`
	UsesScheduler                    *bool   `json:"uses_scheduler,omitempty"`
	UsesOctane                       *bool   `json:"uses_octane,omitempty"`
	UsesInertiaSsr                   *bool   `json:"uses_inertia_ssr,omitempty"`
	ScalingCpuThresholdPercentage    *int    `json:"scaling_cpu_threshold_percentage,omitempty"`
	ScalingMemoryThresholdPercentage *int    `json:"scaling_memory_threshold_percentage,omitempty"`
}

// CreateInstance provisions a compute unit on an environment.
//
// Wire: POST /environments/:envId/instances
// Success: HTTP 201 with the enveloped instance record.
func (c *Client) CreateInstance(ctx context.Context, environmentID string, req CreateInstanceRequest) (*Instance, error) {
	if environmentID == "" {
		return nil, errors.New("environment id is required")
	}
	path := fmt.Sprintf("/environments/%s/instances", environmentID)

	var env Envelope[Instance]
	if err := c.do(ctx, "POST", path, req, &env); err != nil {
		return nil, fmt.Errorf("create instance: %w", err)
	}
	if env.Data.EnvironmentID == "" {
		env.Data.EnvironmentID = environmentID
	}
	return &env.Data, nil
}

// GetInstance reads an instance by ID.
//
// Wire: GET /instances/:id
// Failure: HTTP 404 when the instance was deleted out-of-band.
func (c *Client) GetInstance(ctx context.Context, id string) (*Instance, error) {
	if id == "" {
		return nil, errors.New("instance id is required")
	}
	path := fmt.Sprintf("/instances/%s?include=environment", id)

	var env Envelope[Instance]
	if err := c.do(ctx, "GET", path, nil, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// UpdateInstance PATCHes mutable fields.
//
// Wire: PATCH /instances/:id
func (c *Client) UpdateInstance(ctx context.Context, id string, req UpdateInstanceRequest) (*Instance, error) {
	if id == "" {
		return nil, errors.New("instance id is required")
	}
	path := fmt.Sprintf("/instances/%s", id)

	var env Envelope[Instance]
	if err := c.do(ctx, "PATCH", path, req, &env); err != nil {
		return nil, fmt.Errorf("update instance: %w", err)
	}
	return &env.Data, nil
}

// DeleteInstance tears down a compute unit. Cascade rules match Cloud:
// deleting an instance destroys every attached background process.
//
// Wire: DELETE /instances/:id
func (c *Client) DeleteInstance(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("instance id is required")
	}
	path := fmt.Sprintf("/instances/%s", id)

	if err := c.do(ctx, "DELETE", path, nil, nil); err != nil {
		return fmt.Errorf("delete instance: %w", err)
	}
	return nil
}
