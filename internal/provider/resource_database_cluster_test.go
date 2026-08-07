// Package provider — acceptance test for `laravelcloud_database_cluster`.
//
// Exercises the full CRUD lifecycle against a real Cloud org (guarded
// by TF_ACC=1). NB: Cloud provisions the cluster asynchronously — the
// test framework polls status until `active` before returning from the
// Create step.
package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccDatabaseClusterResource_basic exercises Create → Read → Update →
// Import → Delete against a real Cloud database cluster. The Update step
// bumps `backup_retention_days` — a cheap mutable field.
func TestAccDatabaseClusterResource_basic(t *testing.T) {
	orgID := os.Getenv("LARAVEL_CLOUD_TEST_ORG_ID")
	name := fmt.Sprintf("tf-acc-dbc-%d", os.Getpid())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create.
			{
				Config: testAccDatabaseClusterResourceConfig(orgID, name, 7),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravelcloud_database_cluster.test", "name", name),
					resource.TestCheckResourceAttr("laravelcloud_database_cluster.test", "engine", "postgres-16"),
					resource.TestCheckResourceAttr("laravelcloud_database_cluster.test", "high_availability", "false"),
					resource.TestCheckResourceAttrSet("laravelcloud_database_cluster.test", "id"),
				),
			},
			// Step 2: ImportState.
			{
				ResourceName:      "laravelcloud_database_cluster.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Step 3: Update — bump backup retention.
			{
				Config: testAccDatabaseClusterResourceConfig(orgID, name, 14),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravelcloud_database_cluster.test", "backup_retention_days", "14"),
				),
			},
		},
	})
}

// testAccDatabaseClusterResourceConfig returns HCL for the test cluster.
func testAccDatabaseClusterResourceConfig(orgID, name string, retentionDays int) string {
	return fmt.Sprintf(`
resource "laravelcloud_database_cluster" "test" {
  organization_id       = %q
  name                  = %q
  region                = %q
  engine                = "postgres-16"
  size                  = "db.small"
  high_availability     = false
  backup_retention_days = %d
}
`, orgID, name, testAccRegion(), retentionDays)
}
