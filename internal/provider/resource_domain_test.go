// Package provider — acceptance test for `laravelcloud_domain`.
//
// Requires a real, DNS-controllable hostname. Skips when
// `LARAVEL_CLOUD_TEST_DOMAIN` isn't set — no fallback because we can't
// invent a domain the test tenant provably owns.
package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccDomainResource_basic exercises Create → Read → Update → Import →
// Delete against a real Cloud domain binding. Skipped when
// `LARAVEL_CLOUD_TEST_DOMAIN` isn't set.
func TestAccDomainResource_basic(t *testing.T) {
	testDomain := os.Getenv("LARAVEL_CLOUD_TEST_DOMAIN")
	if testDomain == "" {
		t.Skip("LARAVEL_CLOUD_TEST_DOMAIN not set — skipping domain acceptance test")
	}

	orgID := os.Getenv("LARAVEL_CLOUD_TEST_ORG_ID")
	appName := fmt.Sprintf("tf-acc-dom-app-%d", os.Getpid())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create with real_time verification.
			{
				Config: testAccDomainResourceConfig(orgID, appName, testDomain, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravelcloud_domain.test", "name", testDomain),
					resource.TestCheckResourceAttr("laravelcloud_domain.test", "redirect_from_www", "false"),
					resource.TestCheckResourceAttrSet("laravelcloud_domain.test", "id"),
					resource.TestCheckResourceAttrSet("laravelcloud_domain.test", "environment_id"),
				),
			},
			// Step 2: ImportState.
			{
				ResourceName:      "laravelcloud_domain.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Step 3: Update — enable www redirect.
			{
				Config: testAccDomainResourceConfig(orgID, appName, testDomain, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravelcloud_domain.test", "redirect_from_www", "true"),
				),
			},
		},
	})
}

func testAccDomainResourceConfig(orgID, appName, domainName string, redirectWWW bool) string {
	return fmt.Sprintf(`
resource "laravelcloud_application" "parent" {
  organization_id              = %q
  name                         = %q
  region                       = %q
  source_control_provider_type = "github"
}

resource "laravelcloud_environment" "parent" {
  application_id = laravelcloud_application.parent.id
  name           = "development"
  branch         = "develop"
}

resource "laravelcloud_domain" "test" {
  environment_id     = laravelcloud_environment.parent.id
  name               = %q
  redirect_from_www  = %t
  wildcard_enabled   = false
  cloudflare_managed = false
  verification       = "real_time"
}
`, orgID, appName, testAccRegion(), domainName, redirectWWW)
}
