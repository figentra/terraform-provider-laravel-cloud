package api

import (
	"context"
	"errors"
	"fmt"
)

// Bucket is an S3-compatible object storage bucket bound to an org.
//
// v0.4.0 attribute expansion: Cloud's bucket surface split `mode` into the
// finer-grained `visibility` + `jurisdiction` + `key_name` +
// `key_permission` quadruplet. The provider carries BOTH the legacy `mode`
// and the new fields so consumers can migrate incrementally — Cloud
// accepts either shape at the API layer, and the provider unions them into
// the same in-memory struct.
type Bucket struct {
	ID             string  `json:"id"`
	OrganizationID string  `json:"organization_id"`
	Name           string  `json:"name"`
	Region         string  `json:"region"`

	// Mode is the legacy access model — "private" | "public". Deprecated
	// in favor of Visibility (v0.4.0+) but kept for backward compatibility
	// with pre-v0.4 consumers.
	Mode string `json:"mode,omitempty"`

	// Visibility is the v0.4.0 canonical access model — "private" |
	// "public". Populated by Cloud in its v2 API responses; the provider
	// falls back to reading `mode` when `visibility` is empty (older
	// API responses).
	Visibility string `json:"visibility,omitempty"`

	// Jurisdiction is the geographic region cluster the bucket is
	// provisioned into — "eu", "us", "ap", or a Cloud-defined slug.
	// Distinct from `Region` which names the specific AWS-style zone.
	// Nullable — Cloud picks a default based on the organisation.
	Jurisdiction *string `json:"jurisdiction,omitempty"`

	// KeyName is the identifier of the auto-generated access key Cloud
	// mints alongside the bucket. Callers reference this key from
	// application env vars. Read-only from the provider's perspective
	// (set at create-time, Cloud-canonical).
	KeyName *string `json:"key_name,omitempty"`

	// KeyPermission is the permission level of the auto-generated key —
	// "read_write" | "read" | "write". Mutable; defaults to "read_write".
	KeyPermission *string `json:"key_permission,omitempty"`

	Status    *string `json:"status,omitempty"`
	CreatedAt *string `json:"created_at,omitempty"`
	UpdatedAt *string `json:"updated_at,omitempty"`
}

// CreateBucketRequest is POST /buckets.
type CreateBucketRequest struct {
	OrganizationID string `json:"organization_id,omitempty"`
	Name           string `json:"name"`
	Region         string `json:"region,omitempty"`

	// Mode + Visibility are aliased at the wire — the provider sends
	// whichever the operator declared. If both are set, Visibility wins.
	Mode       string `json:"mode,omitempty"`
	Visibility string `json:"visibility,omitempty"`

	Jurisdiction  *string `json:"jurisdiction,omitempty"`
	KeyName       *string `json:"key_name,omitempty"`
	KeyPermission *string `json:"key_permission,omitempty"`
}

// UpdateBucketRequest is PATCH /buckets/:id. Mutable fields as of v0.4.0
// include Mode/Visibility + KeyPermission.
type UpdateBucketRequest struct {
	Mode          *string `json:"mode,omitempty"`
	Visibility    *string `json:"visibility,omitempty"`
	KeyPermission *string `json:"key_permission,omitempty"`
}

// CreateBucket provisions a new bucket.
func (c *Client) CreateBucket(ctx context.Context, req CreateBucketRequest) (*Bucket, error) {
	var env Envelope[Bucket]
	if err := c.do(ctx, "POST", "/buckets", req, &env); err != nil {
		return nil, fmt.Errorf("create bucket: %w", err)
	}
	return &env.Data, nil
}

// GetBucket reads a bucket by ID.
func (c *Client) GetBucket(ctx context.Context, id string) (*Bucket, error) {
	if id == "" {
		return nil, errors.New("bucket id is required")
	}
	var env Envelope[Bucket]
	if err := c.do(ctx, "GET", "/buckets/"+id, nil, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// UpdateBucket PATCHes mode.
func (c *Client) UpdateBucket(ctx context.Context, id string, req UpdateBucketRequest) (*Bucket, error) {
	if id == "" {
		return nil, errors.New("bucket id is required")
	}
	var env Envelope[Bucket]
	if err := c.do(ctx, "PATCH", "/buckets/"+id, req, &env); err != nil {
		return nil, fmt.Errorf("update bucket: %w", err)
	}
	return &env.Data, nil
}

// DeleteBucket tears down the bucket. Cloud rejects on 409 when the bucket
// still contains objects — Terraform surfaces the error to the operator.
func (c *Client) DeleteBucket(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("bucket id is required")
	}
	if err := c.do(ctx, "DELETE", "/buckets/"+id, nil, nil); err != nil {
		return fmt.Errorf("delete bucket: %w", err)
	}
	return nil
}
