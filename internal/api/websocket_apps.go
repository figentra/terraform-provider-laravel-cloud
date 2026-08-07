package api

import (
	"context"
	"errors"
	"fmt"
)

// WebsocketApp binds an environment to a WebsocketCluster.
type WebsocketApp struct {
	ID             string  `json:"id"`
	ClusterID      string  `json:"cluster_id"`
	EnvironmentID  string  `json:"environment_id"`
	MaxConnections *int    `json:"max_connections"`
	AppKey         *string `json:"app_key"`
	Status         *string `json:"status"`
	CreatedAt      *string `json:"created_at"`
}

// CreateWebsocketAppRequest is POST /websocket-servers/:clusterId/applications.
type CreateWebsocketAppRequest struct {
	EnvironmentID  string `json:"environment_id"`
	MaxConnections int    `json:"max_connections,omitempty"`
}

// UpdateWebsocketAppRequest is PATCH — only max_connections is mutable.
type UpdateWebsocketAppRequest struct {
	MaxConnections *int `json:"max_connections,omitempty"`
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
