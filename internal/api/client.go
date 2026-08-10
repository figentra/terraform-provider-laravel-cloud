package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultBaseURL is the production Laravel Cloud API endpoint. Consumers
// override via the provider's `base_url` attribute for local development
// against a preview build or staging environment.
const DefaultBaseURL = "https://cloud.laravel.com/api"

// DefaultTimeout is the per-request budget applied when the provider's
// `timeout` attribute is unset. Chosen to match the PHP CLI's default.
const DefaultTimeout = 60 * time.Second

// DefaultUserAgent identifies this provider in Cloud's audit log. The
// version string is stamped by main.go at release time.
const DefaultUserAgent = "terraform-provider-laravel-cloud"

// Client is the typed HTTP client every resource reaches for. Constructed
// once in `provider.Configure` and stashed on `resourceData.ProviderData`
// so every resource shares the same auth + HTTP settings.
type Client struct {
	BaseURL    string
	Token      string
	UserAgent  string
	HTTPClient *http.Client
}

// New returns a Client with sensible defaults + operator overrides. Called
// exactly once per Terraform plan/apply cycle. The returned client is
// safe for concurrent use by multiple resources.
func New(baseURL, token, userAgent string, timeout time.Duration) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if userAgent == "" {
		userAgent = DefaultUserAgent
	}
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	return &Client{
		BaseURL:   strings.TrimRight(baseURL, "/"),
		Token:     token,
		UserAgent: userAgent,
		HTTPClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// do executes an authenticated JSON request against Cloud. Every resource
// method wraps this — never construct `http.NewRequest` directly.
//
// Retry policy: on 429 (rate limit), exponential backoff up to 3 attempts.
// On 5xx, single retry with 500ms backoff. On 4xx (other than 429), fail
// immediately — no retry loop can turn a 422 into a 200.
//
// Response handling: on 2xx, unmarshal `body` into `out` (typed pointer OR
// nil for `204 No Content`). On non-2xx, unmarshal the error envelope and
// return a typed error operators can pattern-match.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	url := c.BaseURL + path

	const maxAttempts = 3
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		var reqBody io.Reader
		if body != nil {
			b, err := json.Marshal(body)
			if err != nil {
				return fmt.Errorf("marshal request body: %w", err)
			}
			reqBody = bytes.NewReader(b)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+c.Token)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", c.UserAgent)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("http request: %w", err)
			if attempt < maxAttempts {
				time.Sleep(backoff(attempt))
				continue
			}
			return lastErr
		}

		respBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read response body: %w", readErr)
		}

		// Success: unmarshal or return.
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if out == nil || len(respBody) == 0 {
				return nil
			}
			// Cloud has an intermittent auth-redirect fluke where a
			// request to /api/<path> gets served the marketing SPA
			// (HTML body) instead of the API JSON. This shows up as
			// a 2xx response with `<!DOCTYPE html>...` content.
			//
			// The previous heuristic here treated HTML-body 2xx as
			// an empty success (`return nil`), silently accepting an
			// unusable response. On Create endpoints this left the
			// provider with a zero-valued resource ID + no downstream
			// visibility — the resource looked "created" in state
			// but every dependent child failed with "application id
			// is required".
			//
			// Correct handling: retry the request (auth-redirect is
			// transient — a second attempt usually hits the API
			// route correctly). If HTML persists across every
			// attempt, surface as a hard error so upstream sees the
			// substrate is broken.
			if bytes.HasPrefix(bytes.TrimSpace(respBody), []byte("<")) {
				if attempt < maxAttempts {
					lastErr = fmt.Errorf(
						"cloud API returned HTML body on %s %s (auth-redirect fluke) — retry %d/%d",
						method, path, attempt, maxAttempts,
					)
					time.Sleep(backoff(attempt))
					continue
				}
				return fmt.Errorf(
					"cloud API returned HTML on %s %s after %d attempts "+
						"(auth-redirect fluke persisted; response start: %q)",
					method, path, maxAttempts, string(respBody[:min(len(respBody), 200)]),
				)
			}
			if err := json.Unmarshal(respBody, out); err != nil {
				return fmt.Errorf("unmarshal response: %w (body: %s)", err, respBody)
			}
			return nil
		}

		// Rate limit: back off + retry.
		if resp.StatusCode == http.StatusTooManyRequests && attempt < maxAttempts {
			time.Sleep(backoff(attempt))
			continue
		}

		// 5xx: single retry.
		if resp.StatusCode >= 500 && attempt < maxAttempts {
			time.Sleep(backoff(attempt))
			continue
		}

		// Terminal error — unmarshal + return.
		var errResp ErrorResponse
		_ = json.Unmarshal(respBody, &errResp) // best-effort
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    errResp.Message,
			Errors:     errResp.Errors,
			RawBody:    string(respBody),
		}
	}

	return lastErr
}

// backoff returns an exponential backoff duration for the given attempt.
// attempt=1 → 500ms, attempt=2 → 1s, attempt=3 → 2s. Capped at 10s so no
// single request stalls a plan indefinitely.
func backoff(attempt int) time.Duration {
	d := time.Duration(500*(1<<uint(attempt-1))) * time.Millisecond
	if d > 10*time.Second {
		d = 10 * time.Second
	}
	return d
}

// APIError is the typed error every resource surfaces on a non-2xx response.
// Provider resources render `Message` in the diagnostic summary + include
// `RawBody` in the detail so operators can debug without curl.
type APIError struct {
	StatusCode int
	Message    string
	Errors     map[string][]string
	RawBody    string
}

// Error satisfies the standard error interface. Format matches the PHP
// CLI's error shape for consistency across tools.
func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("HTTP %d", e.StatusCode)
}

// IsNotFound returns true when the response was a 404 — provider Read
// implementations use this to distinguish "resource genuinely missing"
// (drift, drop from state) from "API had a transient error".
func (e *APIError) IsNotFound() bool {
	return e.StatusCode == http.StatusNotFound
}
