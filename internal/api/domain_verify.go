package api

import (
	"context"
	"errors"
	"fmt"
)

// VerifyDomain triggers Cloud's DNS-verification pass on a domain. Cloud
// polls the domain's DNS records + verifies the CNAME/A points at the
// env's vanity_domain (or its ACME challenge for pre-verification).
//
// Wire: POST /domains/:id/verify
// Success: HTTP 200 with the enveloped domain record — `status` reflects
// the verification result (`verified`, `pending`, `failed`).
// Failure: HTTP 404 when the domain was deleted out-of-band.
func (c *Client) VerifyDomain(ctx context.Context, id string) (*Domain, error) {
	if id == "" {
		return nil, errors.New("domain id is required")
	}
	path := fmt.Sprintf("/domains/%s/verify", id)

	var env Envelope[Domain]
	if err := c.do(ctx, "POST", path, struct{}{}, &env); err != nil {
		return nil, fmt.Errorf("verify domain: %w", err)
	}
	return &env.Data, nil
}
