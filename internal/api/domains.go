package api

import (
	"context"
	"errors"
	"fmt"
)

// Domain is a custom hostname bound to a Cloud environment.
//
// v0.4.0 renamed several attributes to match Cloud's v2 API surface. The
// provider carries BOTH the pre-v0.4 names and the v0.4 canonical names
// (via omitempty JSON tags) so consumers using either shape produce
// wire-compatible requests. Cloud's API accepts either family and
// normalises server-side.
type Domain struct {
	ID            string `json:"id"`
	EnvironmentID string `json:"environment_id"`
	Name          string `json:"name"`

	// Pre-v0.4 names (kept for backward compat)
	RedirectFromWWW   bool   `json:"redirect_from_www,omitempty"`
	CloudflareManaged bool   `json:"cloudflare_managed,omitempty"`
	Verification      string `json:"verification,omitempty"`

	// v0.4 canonical names. WWWRedirect is a STRING enum in v2 —
	// "www_to_root", "root_to_www", "none".
	WWWRedirect        *string `json:"www_redirect,omitempty"`
	CloudflareStrategy *string `json:"cloudflare_strategy,omitempty"`
	VerificationMethod *string `json:"verification_method,omitempty"`

	WildcardEnabled bool    `json:"wildcard_enabled,omitempty"`
	Status          *string `json:"status,omitempty"`
	CreatedAt       *string `json:"created_at,omitempty"`
	UpdatedAt       *string `json:"updated_at,omitempty"`
}

// CreateDomainRequest is POST /environments/:envId/domains.
type CreateDomainRequest struct {
	Name string `json:"name"`

	// Pre-v0.4 names
	RedirectFromWWW   bool   `json:"redirect_from_www,omitempty"`
	CloudflareManaged bool   `json:"cloudflare_managed,omitempty"`
	Verification      string `json:"verification,omitempty"`

	// v0.4 names
	WWWRedirect        *string `json:"www_redirect,omitempty"`
	CloudflareStrategy *string `json:"cloudflare_strategy,omitempty"`
	VerificationMethod *string `json:"verification_method,omitempty"`

	WildcardEnabled bool `json:"wildcard_enabled,omitempty"`
}

// UpdateDomainRequest is PATCH — redirect + wildcard flags are mutable.
type UpdateDomainRequest struct {
	RedirectFromWWW *bool   `json:"redirect_from_www,omitempty"`
	WWWRedirect     *string `json:"www_redirect,omitempty"`
	WildcardEnabled *bool   `json:"wildcard_enabled,omitempty"`
}

// CreateDomain binds a hostname to an environment.
func (c *Client) CreateDomain(ctx context.Context, environmentID string, req CreateDomainRequest) (*Domain, error) {
	if environmentID == "" {
		return nil, errors.New("environment id is required")
	}
	path := fmt.Sprintf("/environments/%s/domains", environmentID)

	var env Envelope[Domain]
	if err := c.do(ctx, "POST", path, req, &env); err != nil {
		return nil, fmt.Errorf("create domain: %w", err)
	}
	return &env.Data, nil
}

// GetDomain reads a domain binding by ID.
func (c *Client) GetDomain(ctx context.Context, id string) (*Domain, error) {
	if id == "" {
		return nil, errors.New("domain id is required")
	}
	var env Envelope[Domain]
	if err := c.do(ctx, "GET", "/domains/"+id, nil, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// UpdateDomain PATCHes redirect + wildcard flags.
func (c *Client) UpdateDomain(ctx context.Context, id string, req UpdateDomainRequest) (*Domain, error) {
	if id == "" {
		return nil, errors.New("domain id is required")
	}
	var env Envelope[Domain]
	if err := c.do(ctx, "PATCH", "/domains/"+id, req, &env); err != nil {
		return nil, fmt.Errorf("update domain: %w", err)
	}
	return &env.Data, nil
}

// DeleteDomain removes the hostname binding.
func (c *Client) DeleteDomain(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("domain id is required")
	}
	if err := c.do(ctx, "DELETE", "/domains/"+id, nil, nil); err != nil {
		return fmt.Errorf("delete domain: %w", err)
	}
	return nil
}
