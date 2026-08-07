// Package provider — acceptance test for `laravelcloud_cache`.
//
// The `size` attribute is the only mutable field — the test bumps it
// to exercise the PATCH path.
package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccCacheResource_basic exercises Create → Read → Update → Import →
// Delete against a real Cloud cache instance.
func TestAccCacheResource_basic(t *testing.T) {
	orgID := os.Getenv("LARAVEL_CLOUD_TEST_ORG_ID")
	name := fmt.Sprintf("tf-acc-cache-%d", os.Getpid())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create at the small size.
			{
				Config: testAccCacheResourceConfig(orgID, name, "valkey-pro.1gb"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravelcloud_cache.test", "name", name),
					resource.TestCheckResourceAttr("laravelcloud_cache.test", "size", "valkey-pro.1gb"),
					resource.TestCheckResourceAttrSet("laravelcloud_cache.test", "id"),
				),
			},
			// Step 2: ImportState.
			{
				ResourceName:      "laravelcloud_cache.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Step 3: Update — bump the cache size.
			{
				Config: testAccCacheResourceConfig(orgID, name, "valkey-pro.5gb"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravelcloud_cache.test", "size", "valkey-pro.5gb"),
				),
			},
		},
	})
}

func testAccCacheResourceConfig(orgID, name, size string) string {
	return fmt.Sprintf(`
resource "laravelcloud_cache" "test" {
  organization_id = %q
  name            = %q
  region          = %q
  size            = %q
}
`, orgID, name, testAccRegion(), size)
}
