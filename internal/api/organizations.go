// Package api — organization endpoint bindings.
//
// The Cloud API exposes organisation metadata via two distinct routes:
//
//  1. `GET /organizations/:slug` — reads a specific organisation the token
//     has access to. Consumers pass the slug (`figentra`, `academorix`).
//  2. `GET /meta/organization` — reads the "default" organisation the
//     current token is scoped to. Some Cloud tokens are org-scoped at
//     creation and this endpoint returns THAT org without the caller
//     needing to know its slug.
//
// The provider exposes both via the `laravelcloud_organization` data
// source — the `id` attribute is Optional; when unset the data source
// falls back to `/meta/organization` (token-scoped resolution).
package api

import (
	"context"
	"errors"
	"fmt"
)

// GetOrganization reads an organisation by slug (or ULID — Cloud accepts
// either). Returns the typed Organization envelope from the API.
//
// Consumers use this when they know the org slug ahead of time and want
// to hydrate the ID + name for downstream module composition.
func (c *Client) GetOrganization(ctx context.Context, slugOrID string) (*Organization, error) {
	if slugOrID == "" {
		return nil, errors.New("organization slug or id is required")
	}

	// Cloud accepts both slug (`figentra`) and ULID at this endpoint.
	var env Envelope[Organization]
	if err := c.do(ctx, "GET", "/organizations/"+slugOrID, nil, &env); err != nil {
		return nil, fmt.Errorf("get organization %q: %w", slugOrID, err)
	}
	return &env.Data, nil
}

// GetDefaultOrganization reads the organisation the current API token is
// scoped to via `GET /meta/organization`. Useful when the token was
// generated for a specific org and the caller doesn't want to hard-code
// the slug in HCL.
func (c *Client) GetDefaultOrganization(ctx context.Context) (*Organization, error) {
	var env Envelope[Organization]
	if err := c.do(ctx, "GET", "/meta/organization", nil, &env); err != nil {
		return nil, fmt.Errorf("get default organization: %w", err)
	}
	return &env.Data, nil
}
