// Package provider — shared test helpers.
//
// Every acceptance test in this package uses `testAccProtoV6ProviderFactories`
// (to spin up the provider in-process, no `terraform init` shell-out) +
// `testAccPreCheck` (to bail cleanly when required env vars aren't set).
//
// Acceptance tests run only when `TF_ACC=1` is set. Unit tests run on
// every `go test` and never touch Cloud.
package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories registers the provider under test.
// Every acceptance test declares this factory so `terraform-plugin-testing`
// can spin up an in-process provider instance without shelling out to
// `terraform init`. `"test"` is the version stamp — surfaced via
// `terraform version` inside the test harness.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"laravelcloud": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccPreCheck runs before every acceptance test — bails out with a
// clear message when required env vars aren't set. Called from every
// `resource.Test` case via its `PreCheck` hook.
//
// Env vars every acceptance test relies on:
//   - `LARAVEL_CLOUD_TOKEN`         — token with write scope on the test org
//   - `LARAVEL_CLOUD_TEST_ORG_ID`   — ULID of the disposable test org
//   - `LARAVEL_CLOUD_TEST_REGION`   — optional; defaults to `us-east-1`
//   - `LARAVEL_CLOUD_TEST_CLUSTER_ID` — used by schema / websocket-app
//     acceptance tests that need a pre-provisioned parent cluster.
//     Optional per-test; individual test files enforce their own gate.
func testAccPreCheck(t *testing.T) {
	if os.Getenv("LARAVEL_CLOUD_TOKEN") == "" {
		t.Fatal("LARAVEL_CLOUD_TOKEN must be set for acceptance tests")
	}
	if os.Getenv("LARAVEL_CLOUD_TEST_ORG_ID") == "" {
		t.Fatal("LARAVEL_CLOUD_TEST_ORG_ID must be set for acceptance tests")
	}
}

// testAccRegion returns the region every test creates resources in.
// Defaults to `us-east-1` — override via `LARAVEL_CLOUD_TEST_REGION` when
// the test tenant is provisioned in a different region.
func testAccRegion() string {
	if r := os.Getenv("LARAVEL_CLOUD_TEST_REGION"); r != "" {
		return r
	}
	return "us-east-1"
}
