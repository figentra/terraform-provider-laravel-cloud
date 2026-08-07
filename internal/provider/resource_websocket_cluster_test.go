// Package provider — acceptance test for `laravelcloud_websocket_cluster`.
package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccWebsocketClusterResource_basic exercises Create → Read →
// Update → Import → Delete against a real Cloud WS cluster. The Update
// step bumps `max_connections` — a cheap mutable field.
func TestAccWebsocketClusterResource_basic(t *testing.T) {
	orgID := os.Getenv("LARAVEL_CLOUD_TEST_ORG_ID")
	name := fmt.Sprintf("tf-acc-wsc-%d", os.Getpid())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create.
			{
				Config: testAccWebsocketClusterResourceConfig(orgID, name, 1000),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravelcloud_websocket_cluster.test", "name", name),
					resource.TestCheckResourceAttr("laravelcloud_websocket_cluster.test", "max_connections", "1000"),
					resource.TestCheckResourceAttrSet("laravelcloud_websocket_cluster.test", "id"),
				),
			},
			// Step 2: ImportState.
			{
				ResourceName:      "laravelcloud_websocket_cluster.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Step 3: Update — raise the connection cap.
			{
				Config: testAccWebsocketClusterResourceConfig(orgID, name, 2500),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravelcloud_websocket_cluster.test", "max_connections", "2500"),
				),
			},
		},
	})
}

func testAccWebsocketClusterResourceConfig(orgID, name string, maxConns int) string {
	return fmt.Sprintf(`
resource "laravelcloud_websocket_cluster" "test" {
  organization_id = %q
  name            = %q
  region          = %q
  size            = "reverb.small"
  max_connections = %d
}
`, orgID, name, testAccRegion(), maxConns)
}
