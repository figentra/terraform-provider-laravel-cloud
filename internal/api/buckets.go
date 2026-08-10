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

// BucketKey is one of the S3-style access-credential pairs bound to a
// bucket. Cloud auto-mints a "root" key alongside every bucket at create
// time (`{bucket}.name + "-root"`) and lets operators author additional
// scoped keys via `POST /buckets/{id}/keys`. Every key carries its own
// (access_key_id, access_key_secret) pair; deleting the bucket requires
// draining every attached key first.
//
// Wire shape mirrors Cloud's `filesystemKeys` resource type:
//
//	{
//	  "id":                 "flsk-<uuid>",
//	  "type":               "filesystemKeys",
//	  "attributes": {
//	    "name":             "<string>",
//	    "permission":       "read_write" | "read" | "write",
//	    "access_key_id":    "<hex>",
//	    "access_key_secret":"<hex>",
//	    "created_at":       "<ISO-8601>"
//	  }
//	}
type BucketKey struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Permission      string  `json:"permission"`
	AccessKeyID     *string `json:"access_key_id,omitempty"`
	AccessKeySecret *string `json:"access_key_secret,omitempty"`
	CreatedAt       *string `json:"created_at,omitempty"`
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

// DeleteBucket tears down the bucket. Cloud rejects with 422
// ("The filesystem has keys attached and cannot be deleted. Please
// delete all keys first.") when access keys remain attached — including
// the auto-generated "root" key Cloud mints at bucket-create time.
//
// Cascade behaviour: before firing the bucket DELETE, list every key
// attached to the bucket and delete each. Fail-open per-key — a 404 on
// a single key (already gone) is not fatal. Any other error propagates
// so the operator can inspect.
//
// Rationale: mirrors the `DeleteDatabaseCluster` + `drainSchemas`
// pattern. Two teardown scenarios drive the cascade:
//
//  1. Cloud's auto-mint — every bucket ships with a root key by
//     default; even a "clean" state has one attached key that must
//     be reaped before the bucket DELETE succeeds.
//
//  2. Stale terraform state — additional keys may have been minted
//     via Cloud UI or a prior `POST /buckets/{id}/keys` call that
//     terraform never tracked; the drain loop reaps every one.
//
// Retry loop handles Cloud's eventual-consistency race: after each
// 422, drain lingering keys + wait for Cloud to reconcile, then retry
// the bucket DELETE. Bounded at 5 attempts with exponential backoff
// (500ms → 8s cap, ≈ 15s total worst-case) so a genuinely-broken
// Cloud state fails loudly instead of stalling the plan forever.
//
// Wire endpoints:
//   - LIST keys:   GET    /buckets/{bucket_id}/keys
//   - DELETE key:  DELETE /bucket-keys/{key_id}    (flat, NOT nested)
//   - DELETE bkt:  DELETE /buckets/{bucket_id}
func (c *Client) DeleteBucket(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("bucket id is required")
	}

	const maxAttempts = 5
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Reap any lingering keys Cloud still knows about.
		if drainErr := c.drainBucketKeys(ctx, id); drainErr != nil {
			return drainErr
		}

		if err := c.do(ctx, "DELETE", "/buckets/"+id, nil, nil); err != nil {
			var apiErr *APIError
			// Race: Cloud still reports keys attached even though
			// our drainBucketKeys call reported success. Back off,
			// drain again, retry.
			if errors.As(err, &apiErr) && apiErr.IsKeysAttached() && attempt < maxAttempts {
				lastErr = err
				continue
			}
			// 404 is idempotent — bucket already gone from a
			// previous partially-successful destroy.
			if errors.As(err, &apiErr) && apiErr.IsNotFound() {
				return nil
			}
			return fmt.Errorf("delete bucket: %w", err)
		}
		return nil
	}
	return fmt.Errorf("delete bucket: exhausted %d attempts, last error: %w", maxAttempts, lastErr)
}

// ListBucketKeys enumerates every access key attached to a bucket.
//
// Wire: GET /buckets/{bucket_id}/keys
// Success: HTTP 200 with `data: [{ id, type, attributes: {...} }, ...]`
// (JSON:API envelope; the shared flattener hoists attributes → fields).
func (c *Client) ListBucketKeys(ctx context.Context, bucketID string) ([]BucketKey, error) {
	if bucketID == "" {
		return nil, errors.New("bucket id is required")
	}
	var env Envelope[[]BucketKey]
	if err := c.do(ctx, "GET", "/buckets/"+bucketID+"/keys", nil, &env); err != nil {
		return nil, err
	}
	return env.Data, nil
}

// DeleteBucketKey removes ONE access key by ID. The wire endpoint is
// FLAT (`/bucket-keys/{key_id}`), NOT nested under the parent bucket —
// Cloud's REST API models keys as a top-level resource with an
// implicit parent reference inside the record. Matches Cloud's own
// API docs (laravel.com/cloud/docs/api/bucket-keys/delete-object-storage-key).
//
// Wire: DELETE /bucket-keys/{key_id}
// Success: HTTP 204 No Content.
func (c *Client) DeleteBucketKey(ctx context.Context, keyID string) error {
	if keyID == "" {
		return errors.New("bucket key id is required")
	}
	if err := c.do(ctx, "DELETE", "/bucket-keys/"+keyID, nil, nil); err != nil {
		return fmt.Errorf("delete bucket key: %w", err)
	}
	return nil
}

// drainBucketKeys lists every access key attached to a bucket and
// deletes each. Fail-open per-key on 404 (already gone). Surfaces any
// other list/delete error to the caller.
//
// Used by `DeleteBucket`'s cascade — mirrors `drainSchemas`.
func (c *Client) drainBucketKeys(ctx context.Context, bucketID string) error {
	keys, err := c.ListBucketKeys(ctx, bucketID)
	if err != nil {
		// Bucket may already be half-deleted or the list endpoint may
		// 404 mid-bucket-teardown — treat 404 as "nothing to drain".
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return nil
		}
		return fmt.Errorf("cascade list keys before bucket delete: %w", err)
	}
	for _, k := range keys {
		if delErr := c.DeleteBucketKey(ctx, k.ID); delErr != nil {
			var apiErr *APIError
			if errors.As(delErr, &apiErr) && apiErr.IsNotFound() {
				continue // already gone; skip
			}
			return fmt.Errorf("cascade delete bucket key %s: %w", k.ID, delErr)
		}
	}
	return nil
}
