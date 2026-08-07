// Package provider — acceptance test for `laravelcloud_websocket_app`.
//
// The websocket-app binding attaches an environment to a websocket
// cluster — so this test provisions the full chain (app → env →
// ws-cluster → ws-app) then destroys it in reverse.
package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccWebsocketAppResource_basic exercises Create → Read → Update →
// Import → Delete against a real Cloud WS app binding.
func TestAccWebsocketAppResource_basic(t *testing.T) {
	orgID := os.Getenv("LARAVEL_CLOUD_TEST_ORG_ID")
	appName := fmt.Sprintf("tf-acc-wsa-app-%d", os.Getpid())
	wsClusterName := fmt.Sprintf("tf-acc-wsa-cluster-%d", os.Getpid())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create the full chain.
			{
				Config: testAccWebsocketAppResourceConfig(orgID, appName, wsClusterName, 500),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravelcloud_websocket_app.test", "max_connections", "500"),
					resource.TestCheckResourceAttrSet("laravelcloud_websocket_app.test", "id"),
					resource.TestCheckResourceAttrSet("laravelcloud_websocket_app.test", "cluster_id"),
					resource.TestCheckResourceAttrSet("laravelcloud_websocket_app.test", "environment_id"),
					resource.TestCheckResourceAttrSet("laravelcloud_websocket_app.test", "app_key"),
				),
			},
			// Step 2: ImportState — skip `app_key` verification because
			// Cloud rotates it on binding-recreate and the imported state
			// carries the current value only.
			{
				ResourceName:            "laravelcloud_websocket_app.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"app_key"},
			},
			// Step 3: Update — bump the per-app connection cap.
			{
				Config: testAccWebsocketAppResourceConfig(orgID, appName, wsClusterName, 1000),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravelcloud_websocket_app.test", "max_connections", "1000"),
				),
			},
		},
	})
}

func testAccWebsocketAppResourceConfig(orgID, appName, wsClusterName string, maxConns int) string {
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

resource "laravelcloud_websocket_cluster" "parent" {
  organization_id = %q
  name            = %q
  region          = %q
  size            = "reverb.small"
  max_connections = 5000
}

resource "laravelcloud_websocket_app" "test" {
  cluster_id      = laravelcloud_websocket_cluster.parent.id
  environment_id  = laravelcloud_environment.parent.id
  max_connections = %d
}
`, orgID, appName, testAccRegion(), orgID, wsClusterName, testAccRegion(), maxConns)
}
