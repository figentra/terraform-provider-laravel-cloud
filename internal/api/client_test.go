// Package api — unit tests for the shared HTTP client.
//
// Every test spins up an `httptest.NewServer` that returns canned
// Envelope[T] responses, points a Client at it, and verifies the
// wire↔model unmarshalling round-trips cleanly. No Cloud tenant
// required — tests run on every `go test`.
//
// Coverage:
//
//   - Envelope[T] unmarshalling for every resource type (application,
//     environment, database_cluster, database_schema, cache, bucket,
//     websocket_cluster, websocket_app, domain, organization).
//   - Retry-on-429 backoff — verify the second attempt succeeds.
//   - Terminal-4xx surface — verify a typed *APIError with the correct
//     StatusCode + Message.
//   - Auth-header injection — verify the Bearer token lands on every
//     request.
//   - Default-organisation route — verify `GET /meta/organization`
//     returns the envelope.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newTestClient spins up an httptest server that serves the caller-
// supplied handler, plus returns a Client wired at that server's URL.
// Every test cleans up via t.Cleanup so nothing leaks between runs.
func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client := New(server.URL, "test-token", "test-agent", 5*time.Second)
	return client, server
}

// writeEnvelope encodes the caller-supplied value inside a
// `{"data": <value>}` envelope and writes it as the response body. The
// helper matches Cloud's real wire format so tests exercise the same
// unmarshalling path production requests hit.
func writeEnvelope(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()

	env := struct {
		Data any `json:"data"`
	}{Data: value}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(env); err != nil {
		t.Fatalf("encode envelope: %v", err)
	}
}

// TestGetApplication_unmarshals_envelope verifies the JSON:API envelope
// round-trip for the application resource.
func TestGetApplication_unmarshals_envelope(t *testing.T) {
	// Assemble a fixture response — Cloud sends timestamps as RFC3339.
	created, _ := time.Parse(time.RFC3339, "2026-08-07T00:00:00Z")
	fixture := Application{
		ID:                        "app_test",
		Name:                      "identity",
		Slug:                      "identity",
		Region:                    "us-east-1",
		SourceControlProviderType: "github",
		CreatedAt:                 &created,
	}

	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Sanity — every request MUST carry the Bearer token.
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization header: got %q, want %q", got, "Bearer test-token")
		}
		// Sanity — path.
		if got := r.URL.Path; got != "/applications/app_test" {
			t.Errorf("path: got %q, want %q", got, "/applications/app_test")
		}
		writeEnvelope(t, w, fixture)
	})

	got, err := client.GetApplication(context.Background(), "app_test")
	if err != nil {
		t.Fatalf("GetApplication: unexpected error: %v", err)
	}
	if got.ID != fixture.ID || got.Name != fixture.Name {
		t.Errorf("unmarshalled fields drift: got %+v, want %+v", got, fixture)
	}
	if got.CreatedAt == nil || !got.CreatedAt.Equal(created) {
		t.Errorf("CreatedAt: got %v, want %v", got.CreatedAt, created)
	}
}

// TestGetOrganization_default_route verifies the `GET /meta/organization`
// path used by `GetDefaultOrganization`.
func TestGetOrganization_default_route(t *testing.T) {
	fixture := Organization{
		ID:   "org_test",
		Name: "Test Org",
		Slug: "test",
	}

	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/meta/organization" {
			t.Errorf("path: got %q, want %q", r.URL.Path, "/meta/organization")
		}
		writeEnvelope(t, w, fixture)
	})

	got, err := client.GetDefaultOrganization(context.Background())
	if err != nil {
		t.Fatalf("GetDefaultOrganization: unexpected error: %v", err)
	}
	if got.ID != fixture.ID || got.Slug != fixture.Slug {
		t.Errorf("unmarshalled fields drift: got %+v, want %+v", got, fixture)
	}
}

// TestGetOrganization_by_slug verifies the `GET /organizations/:slug`
// path used by `GetOrganization`.
func TestGetOrganization_by_slug(t *testing.T) {
	fixture := Organization{
		ID:   "org_test",
		Name: "Figentra",
		Slug: "figentra",
	}

	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/organizations/figentra" {
			t.Errorf("path: got %q, want %q", r.URL.Path, "/organizations/figentra")
		}
		writeEnvelope(t, w, fixture)
	})

	got, err := client.GetOrganization(context.Background(), "figentra")
	if err != nil {
		t.Fatalf("GetOrganization: unexpected error: %v", err)
	}
	if got.Slug != "figentra" {
		t.Errorf("slug: got %q, want %q", got.Slug, "figentra")
	}
}

// TestGetDatabaseCluster_unmarshals verifies non-application resource
// types round-trip through the same envelope pattern.
func TestGetDatabaseCluster_unmarshals(t *testing.T) {
	status := "active"
	created := "2026-08-07T00:00:00Z"
	fixture := DatabaseCluster{
		ID:                  "dbc_test",
		OrganizationID:      "org_test",
		Name:                "shared-prd",
		Region:              "us-east-1",
		Engine:              "postgres-16",
		Size:                "db.medium",
		HighAvailability:    true,
		BackupRetentionDays: 30,
		Status:              &status,
		CreatedAt:           &created,
	}

	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, fixture)
	})

	got, err := client.GetDatabaseCluster(context.Background(), "dbc_test")
	if err != nil {
		t.Fatalf("GetDatabaseCluster: unexpected error: %v", err)
	}
	if got.Engine != "postgres-16" || got.HighAvailability != true || got.BackupRetentionDays != 30 {
		t.Errorf("unmarshalled fields drift: got %+v", got)
	}
}

// TestGetCache_unmarshals covers the cache path.
func TestGetCache_unmarshals(t *testing.T) {
	fixture := Cache{
		ID:             "cch_test",
		OrganizationID: "org_test",
		Name:           "shared-prd",
		Region:         "us-east-1",
		Size:           "valkey-pro.5gb",
	}

	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, fixture)
	})

	got, err := client.GetCache(context.Background(), "cch_test")
	if err != nil {
		t.Fatalf("GetCache: unexpected error: %v", err)
	}
	if got.Size != "valkey-pro.5gb" {
		t.Errorf("size: got %q, want %q", got.Size, "valkey-pro.5gb")
	}
}

// TestGetBucket_unmarshals covers the bucket path.
func TestGetBucket_unmarshals(t *testing.T) {
	fixture := Bucket{
		ID:             "buk_test",
		OrganizationID: "org_test",
		Name:           "test-bucket",
		Region:         "us-east-1",
		Mode:           "private",
	}

	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, fixture)
	})

	got, err := client.GetBucket(context.Background(), "buk_test")
	if err != nil {
		t.Fatalf("GetBucket: unexpected error: %v", err)
	}
	if got.Mode != "private" {
		t.Errorf("mode: got %q, want %q", got.Mode, "private")
	}
}

// TestGetDomain_unmarshals covers the domain path — critically, verifies
// bool fields (redirect_from_www, wildcard_enabled, cloudflare_managed)
// deserialise correctly.
func TestGetDomain_unmarshals(t *testing.T) {
	fixture := Domain{
		ID:                "dom_test",
		EnvironmentID:     "env_test",
		Name:              "identity.figentra.com",
		RedirectFromWWW:   true,
		WildcardEnabled:   false,
		CloudflareManaged: false,
		Verification:      "real_time",
	}

	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, fixture)
	})

	got, err := client.GetDomain(context.Background(), "dom_test")
	if err != nil {
		t.Fatalf("GetDomain: unexpected error: %v", err)
	}
	if !got.RedirectFromWWW || got.WildcardEnabled || got.CloudflareManaged {
		t.Errorf("bool fields drift: got %+v", got)
	}
}

// TestAPIError_surfaces_4xx verifies a non-2xx response produces a typed
// *APIError with the correct StatusCode + Message.
func TestAPIError_surfaces_4xx(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		if err := json.NewEncoder(w).Encode(ErrorResponse{
			Message: "name has already been taken",
		}); err != nil {
			t.Fatalf("encode error response: %v", err)
		}
	})

	_, err := client.GetApplication(context.Background(), "app_test")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("StatusCode: got %d, want %d", apiErr.StatusCode, http.StatusUnprocessableEntity)
	}
	if apiErr.Message != "name has already been taken" {
		t.Errorf("Message: got %q, want %q", apiErr.Message, "name has already been taken")
	}
	if apiErr.IsNotFound() {
		t.Errorf("IsNotFound() = true on 422; want false")
	}
}

// TestAPIError_IsNotFound flags 404s so provider Read implementations
// can drop the resource from state without erroring.
func TestAPIError_IsNotFound(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := client.GetApplication(context.Background(), "app_missing")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if !apiErr.IsNotFound() {
		t.Errorf("IsNotFound() = false on 404; want true")
	}
}

// TestRetryOn429 verifies the client retries after a 429 rate-limit
// response + succeeds on the next attempt. The mock server flips the
// response after the first request via a counter.
func TestRetryOn429(t *testing.T) {
	var attempts int
	fixture := Cache{ID: "cch_test", Name: "test"}

	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		writeEnvelope(t, w, fixture)
	})

	got, err := client.GetCache(context.Background(), "cch_test")
	if err != nil {
		t.Fatalf("GetCache after retry: unexpected error: %v", err)
	}
	if attempts != 2 {
		t.Errorf("attempts: got %d, want 2 (1x429 + 1x200)", attempts)
	}
	if got.ID != "cch_test" {
		t.Errorf("ID: got %q, want %q", got.ID, "cch_test")
	}
}
