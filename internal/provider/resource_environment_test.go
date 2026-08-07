// Package provider — acceptance test for `laravelcloud_environment`.
//
// Exercises the full CRUD lifecycle against a real Cloud org (guarded
// by TF_ACC=1). Requires a scratch application the test provisions +
// destroys inside the same test run.
package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccEnvironmentResource_basic exercises Create → Read → Update →
// Import → Delete against a real Cloud environment. Every step verifies
// a computed attribute hydrates + a mutable attribute updates.
func TestAccEnvironmentResource_basic(t *testing.T) {
	orgID := os.Getenv("LARAVEL_CLOUD_TEST_ORG_ID")
	appName := fmt.Sprintf("tf-acc-env-app-%d", os.Getpid())
	envName := "development"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create the parent application + one dev environment.
			{
				Config: testAccEnvironmentResourceConfig(orgID, appName, envName, "develop"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravelcloud_environment.test", "name", envName),
					resource.TestCheckResourceAttr("laravelcloud_environment.test", "branch", "develop"),
					resource.TestCheckResourceAttrSet("laravelcloud_environment.test", "id"),
					resource.TestCheckResourceAttrSet("laravelcloud_environment.test", "application_id"),
				),
			},
			// Step 2: ImportState.
			{
				ResourceName:      "laravelcloud_environment.test",
				ImportState:       true,
				ImportStateVerify: true,
				// `variables` returns encrypted at rest — Cloud may
				// canonicalise on read, so we skip the strict-import
				// verification on that field only.
				ImportStateVerifyIgnore: []string{"variables"},
			},
			// Step 3: Update — patch the branch.
			{
				Config: testAccEnvironmentResourceConfig(orgID, appName, envName, "develop-v2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravelcloud_environment.test", "branch", "develop-v2"),
				),
			},
		},
	})
}

// testAccEnvironmentResourceConfig returns HCL that provisions a scratch
// application + one environment attached to it. Parameterised over the
// caller-supplied branch so update steps flip only that attribute.
func testAccEnvironmentResourceConfig(orgID, appName, envName, branch string) string {
	return fmt.Sprintf(`
resource "laravelcloud_application" "parent" {
  organization_id              = %q
  name                         = %q
  region                       = %q
  source_control_provider_type = "github"
}

resource "laravelcloud_environment" "test" {
  application_id = laravelcloud_application.parent.id
  name           = %q
  branch         = %q

  variables = {
    APP_ENV = "development"
  }
}
`, orgID, appName, testAccRegion(), envName, branch)
}
