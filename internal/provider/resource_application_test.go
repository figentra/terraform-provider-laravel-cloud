// Package provider — acceptance test for `laravelcloud_application`.
//
// Exercises the full CRUD lifecycle against a real Cloud org (guarded
// by TF_ACC=1 + LARAVEL_CLOUD_TEST_ORG_ID). Shared provider factory +
// preCheck helpers live in `provider_test.go`.
package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccApplicationResource_basic exercises the full CRUD lifecycle:
//   - Create: apply a resource block, verify the application exists
//   - Read: refresh state, verify computed fields hydrate
//   - Update: patch the name, verify the API accepts the update
//   - ImportState: import the resource by ID, verify state matches
//   - Delete: (implicit via t.Cleanup) — apply an empty config, verify
//     the application is gone
//
// Runs only when `TF_ACC=1` is set. Skipped otherwise so unit-test runs
// don't touch Cloud.
func TestAccApplicationResource_basic(t *testing.T) {
	orgID := os.Getenv("LARAVEL_CLOUD_TEST_ORG_ID")
	appName := fmt.Sprintf("tf-acc-test-%d", os.Getpid())
	appNameUpdated := appName + "-updated"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create + Read.
			{
				Config: testAccApplicationResourceConfig(orgID, appName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravelcloud_application.test", "name", appName),
					resource.TestCheckResourceAttr("laravelcloud_application.test", "organization_id", orgID),
					resource.TestCheckResourceAttr("laravelcloud_application.test", "region", testAccRegion()),
					resource.TestCheckResourceAttrSet("laravelcloud_application.test", "id"),
					resource.TestCheckResourceAttrSet("laravelcloud_application.test", "slug"),
					resource.TestCheckResourceAttrSet("laravelcloud_application.test", "created_at"),
				),
			},
			// Step 2: ImportState — verify the imported resource matches.
			{
				ResourceName:      "laravelcloud_application.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Step 3: Update — patch the name.
			{
				Config: testAccApplicationResourceConfig(orgID, appNameUpdated),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravelcloud_application.test", "name", appNameUpdated),
				),
			},
			// Delete happens implicitly via the test framework's cleanup.
		},
	})
}

// testAccApplicationResourceConfig returns the HCL for the test resource.
// Parameterised over org + name so multiple tests can reuse the shape.
func testAccApplicationResourceConfig(orgID, name string) string {
	return fmt.Sprintf(`
resource "laravelcloud_application" "test" {
  organization_id              = %q
  name                         = %q
  region                       = %q
  source_control_provider_type = "github"
}
`, orgID, name, testAccRegion())
}
