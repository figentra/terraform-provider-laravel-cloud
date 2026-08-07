// Package provider — acceptance test for `laravelcloud_bucket`.
//
// Cloud rejects `terraform destroy` on non-empty buckets — the test
// keeps the bucket empty so cleanup completes without a 409.
package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccBucketResource_basic exercises Create → Read → Update → Import →
// Delete against a real Cloud bucket. The Update step flips `mode`
// (private ↔ public).
func TestAccBucketResource_basic(t *testing.T) {
	orgID := os.Getenv("LARAVEL_CLOUD_TEST_ORG_ID")
	name := fmt.Sprintf("tf-acc-bkt-%d", os.Getpid())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create private.
			{
				Config: testAccBucketResourceConfig(orgID, name, "private"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravelcloud_bucket.test", "name", name),
					resource.TestCheckResourceAttr("laravelcloud_bucket.test", "mode", "private"),
					resource.TestCheckResourceAttrSet("laravelcloud_bucket.test", "id"),
				),
			},
			// Step 2: ImportState.
			{
				ResourceName:      "laravelcloud_bucket.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Step 3: Update — flip to public.
			{
				Config: testAccBucketResourceConfig(orgID, name, "public"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravelcloud_bucket.test", "mode", "public"),
				),
			},
		},
	})
}

func testAccBucketResourceConfig(orgID, name, mode string) string {
	return fmt.Sprintf(`
resource "laravelcloud_bucket" "test" {
  organization_id = %q
  name            = %q
  region          = %q
  mode            = %q
}
`, orgID, name, testAccRegion(), mode)
}
