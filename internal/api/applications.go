package api

import (
	"context"
	"errors"
	"fmt"
)

// CreateApplication provisions a new Cloud application under an organisation.
//
// Wire: POST /applications
// Success: HTTP 201 with `{"data": <application>}`
// Failure: HTTP 422 with per-field validation errors when name collides in
// the org OR the region is unsupported for the source-control-provider.
func (c *Client) CreateApplication(ctx context.Context, req CreateApplicationRequest) (*Application, error) {
	var env Envelope[Application]
	if err := c.do(ctx, "POST", "/applications", req, &env); err != nil {
		return nil, fmt.Errorf("create application: %w", err)
	}
	return &env.Data, nil
}

// GetApplication reads an application by ID. Passes
// `?include=organization,environments,defaultEnvironment` so the returned
// record carries every relationship the provider surfaces as computed
// attributes.
//
// Wire: GET /applications/:id
// Success: HTTP 200 with the enveloped record + included relationships
// Failure: HTTP 404 when the app was deleted out-of-band. The provider's
// Read implementation calls IsNotFound() on the returned error and drops
// the resource from state — no diagnostic fired, matches Terraform's
// "drift-tolerant" semantics.
func (c *Client) GetApplication(ctx context.Context, id string) (*Application, error) {
	if id == "" {
		return nil, errors.New("application id is required")
	}
	path := fmt.Sprintf("/applications/%s?include=organization,environments,defaultEnvironment", id)

	var env Envelope[Application]
	if err := c.do(ctx, "GET", path, nil, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// UpdateApplication PATCHes mutable fields (name, repository, slack channel).
// Region + source-control-provider are immutable post-create — the schema
// on the provider side enforces `RequiresReplace` for those attributes.
//
// Wire: PATCH /applications/:id
// Success: HTTP 200 with the updated record.
func (c *Client) UpdateApplication(ctx context.Context, id string, req UpdateApplicationRequest) (*Application, error) {
	if id == "" {
		return nil, errors.New("application id is required")
	}
	path := fmt.Sprintf("/applications/%s", id)

	var env Envelope[Application]
	if err := c.do(ctx, "PATCH", path, req, &env); err != nil {
		return nil, fmt.Errorf("update application: %w", err)
	}
	return &env.Data, nil
}

// DeleteApplication tears down the application + every dependent resource
// (environments, database bindings, cache bindings, ...). This is a
// terminal operation — no soft-delete, no undo path from Cloud's side.
//
// Wire: DELETE /applications/:id
// Success: HTTP 204 No Content
// Failure: HTTP 409 when the app has bound resources that must be freed
// first. The PHP CLI works around this with `purgeClusterDatabases()` +
// `purgeBucketKeys()`; the provider mirrors the pattern by ordering
// resource destruction via Terraform's dependency graph.
func (c *Client) DeleteApplication(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("application id is required")
	}
	path := fmt.Sprintf("/applications/%s", id)

	if err := c.do(ctx, "DELETE", path, nil, nil); err != nil {
		return fmt.Errorf("delete application: %w", err)
	}
	return nil
}
