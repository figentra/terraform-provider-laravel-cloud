package api

import (
	"context"
	"errors"
	"fmt"
)

// WebsocketApp is a Reverb app registered under a WebsocketCluster.
//
// v0.4.0 attribute expansion: named apps (via `Name`) instead of the
// pre-v0.4 environment-bound pattern. Environments now reference apps by
// ID via `Environment.WebsocketApplicationID`, letting operators share
// one WS app across multiple envs when appropriate.
type WebsocketApp struct {
	ID        string `json:"id"`
	ClusterID string `json:"cluster_id"`

	// Name is the human-readable app label. Added in v0.4.0.
	Name string `json:"name,omitempty"`

	// EnvironmentID is the pre-v0.4 binding. Kept for backward
	// compatibility; v0.4 modules use `Environment.WebsocketApplicationID`
	// instead.
	EnvironmentID string `json:"environment_id,omitempty"`

	MaxConnections *int    `json:"max_connections,omitempty"`
	AppKey         *string `json:"app_key,omitempty"`
	AppSecret      *string `json:"app_secret,omitempty"`
	Status         *string `json:"status,omitempty"`
	CreatedAt      *string `json:"created_at,omitempty"`
	UpdatedAt      *string `json:"updated_at,omitempty"`
}

// CreateWebsocketAppRequest is POST /websocket-servers/:clusterId/applications.
type CreateWebsocketAppRequest struct {
	Name           string `json:"name,omitempty"`
	EnvironmentID  string `json:"environment_id,omitempty"`
	MaxConnections int    `json:"max_connections,omitempty"`
}

// UpdateWebsocketAppRequest is PATCH — name + max_connections mutable.
type UpdateWebsocketAppRequest struct {
	Name           *string `json:"name,omitempty"`
	MaxConnections *int    `json:"max_connections,omitempty"`
}

// CreateWebsocketApp binds an environment to a WS cluster.
func (c *Client) CreateWebsocketApp(ctx context.Context, clusterID string, req CreateWebsocketAppRequest) (*WebsocketApp, error) {
	if clusterID == "" {
		return nil, errors.New("cluster id is required")
	}
	path := fmt.Sprintf("/websocket-servers/%s/applications", clusterID)

	var env Envelope[WebsocketApp]
	if err := c.do(ctx, "POST", path, req, &env); err != nil {
		return nil, fmt.Errorf("create websocket app: %w", err)
	}
	return &env.Data, nil
}

// GetWebsocketApp reads a WS app by ID.
func (c *Client) GetWebsocketApp(ctx context.Context, id string) (*WebsocketApp, error) {
	if id == "" {
		return nil, errors.New("app id is required")
	}
	var env Envelope[WebsocketApp]
	if err := c.do(ctx, "GET", "/websocket-apps/"+id, nil, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// UpdateWebsocketApp PATCHes max_connections.
func (c *Client) UpdateWebsocketApp(ctx context.Context, id string, req UpdateWebsocketAppRequest) (*WebsocketApp, error) {
	if id == "" {
		return nil, errors.New("app id is required")
	}
	var env Envelope[WebsocketApp]
	if err := c.do(ctx, "PATCH", "/websocket-apps/"+id, req, &env); err != nil {
		return nil, fmt.Errorf("update websocket app: %w", err)
	}
	return &env.Data, nil
}

// DeleteWebsocketApp tears down the binding.
func (c *Client) DeleteWebsocketApp(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("app id is required")
	}
	if err := c.do(ctx, "DELETE", "/websocket-apps/"+id, nil, nil); err != nil {
		return fmt.Errorf("delete websocket app: %w", err)
	}
	return nil
}
