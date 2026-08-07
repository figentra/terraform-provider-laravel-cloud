package api

import (
	"context"
	"errors"
	"fmt"
)

// Bucket is an S3-compatible object storage bucket bound to an org.
type Bucket struct {
	ID             string  `json:"id"`
	OrganizationID string  `json:"organization_id"`
	Name           string  `json:"name"`
	Region         string  `json:"region"`
	Mode           string  `json:"mode"` // "private" | "public"
	Status         *string `json:"status"`
	CreatedAt      *string `json:"created_at"`
}

// CreateBucketRequest is POST /buckets.
type CreateBucketRequest struct {
	OrganizationID string `json:"organization_id"`
	Name           string `json:"name"`
	Region         string `json:"region"`
	Mode           string `json:"mode"`
}

// UpdateBucketRequest is PATCH /buckets/:id — only mode is mutable.
type UpdateBucketRequest struct {
	Mode *string `json:"mode,omitempty"`
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
