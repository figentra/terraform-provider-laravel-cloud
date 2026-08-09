package api

import (
	"context"
	"errors"
	"fmt"
)

// WebsocketCluster is a Reverb-compatible cluster hosting one or more
// WebsocketApp bindings per environment.
//
// v0.4.0: added `Type` (v2 cluster-type slug — currently only `reverb`)
// alongside the pre-v0.4 `Size`. Both remain optional; Cloud infers when
// only one is set.
type WebsocketCluster struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	Name           string `json:"name"`
	Region         string `json:"region"`

	// Type is the Cloud v2 cluster type — `reverb` (Reverb-native). Added
	// in v0.4.0.
	Type *string `json:"type,omitempty"`

	Size           string  `json:"size,omitempty"`
	MaxConnections int     `json:"max_connections,omitempty"`
	Status         *string `json:"status,omitempty"`
	CreatedAt      *string `json:"created_at,omitempty"`
	UpdatedAt      *string `json:"updated_at,omitempty"`
}

// CreateWebsocketClusterRequest is POST /websocket-servers.
type CreateWebsocketClusterRequest struct {
	OrganizationID string  `json:"organization_id,omitempty"`
	Name           string  `json:"name"`
	Region         string  `json:"region,omitempty"`
	Type           *string `json:"type,omitempty"`
	Size           string  `json:"size,omitempty"`
	MaxConnections int     `json:"max_connections,omitempty"`
}

// UpdateWebsocketClusterRequest is PATCH /websocket-servers/:id.
type UpdateWebsocketClusterRequest struct {
	Size           *string `json:"size,omitempty"`
	MaxConnections *int    `json:"max_connections,omitempty"`
}

// CreateWebsocketCluster provisions a WS cluster.
func (c *Client) CreateWebsocketCluster(ctx context.Context, req CreateWebsocketClusterRequest) (*WebsocketCluster, error) {
	var env Envelope[WebsocketCluster]
	if err := c.do(ctx, "POST", "/websocket-servers", req, &env); err != nil {
		return nil, fmt.Errorf("create websocket cluster: %w", err)
	}
	return &env.Data, nil
}

// GetWebsocketCluster reads a WS cluster by ID.
func (c *Client) GetWebsocketCluster(ctx context.Context, id string) (*WebsocketCluster, error) {
	if id == "" {
		return nil, errors.New("cluster id is required")
	}
	var env Envelope[WebsocketCluster]
	if err := c.do(ctx, "GET", "/websocket-servers/"+id, nil, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// UpdateWebsocketCluster PATCHes size + max_connections.
func (c *Client) UpdateWebsocketCluster(ctx context.Context, id string, req UpdateWebsocketClusterRequest) (*WebsocketCluster, error) {
	if id == "" {
		return nil, errors.New("cluster id is required")
	}
	var env Envelope[WebsocketCluster]
	if err := c.do(ctx, "PATCH", "/websocket-servers/"+id, req, &env); err != nil {
		return nil, fmt.Errorf("update websocket cluster: %w", err)
	}
	return &env.Data, nil
}

// DeleteWebsocketCluster tears down the cluster.
func (c *Client) DeleteWebsocketCluster(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("cluster id is required")
	}
	if err := c.do(ctx, "DELETE", "/websocket-servers/"+id, nil, nil); err != nil {
		return fmt.Errorf("delete websocket cluster: %w", err)
	}
	return nil
}
