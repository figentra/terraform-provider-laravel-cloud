// Package provider — acceptance test for `laravelcloud_database_schema`.
//
// The schema resource is immutable post-create — no Update step. The test
// provisions a scratch cluster + one schema, then destroys both.
package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccDatabaseSchemaResource_basic exercises Create → Read → Import →
// Delete against a real Cloud database schema. No Update step — schema
// names are immutable per the Cloud API.
func TestAccDatabaseSchemaResource_basic(t *testing.T) {
	orgID := os.Getenv("LARAVEL_CLOUD_TEST_ORG_ID")
	clusterName := fmt.Sprintf("tf-acc-dbs-cluster-%d", os.Getpid())
	schemaName := fmt.Sprintf("tf_acc_dbs_%d", os.Getpid())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create.
			{
				Config: testAccDatabaseSchemaResourceConfig(orgID, clusterName, schemaName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("laravelcloud_database_schema.test", "name", schemaName),
					resource.TestCheckResourceAttrSet("laravelcloud_database_schema.test", "id"),
					resource.TestCheckResourceAttrSet("laravelcloud_database_schema.test", "cluster_id"),
				),
			},
			// Step 2: ImportState.
			{
				ResourceName:      "laravelcloud_database_schema.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccDatabaseSchemaResourceConfig(orgID, clusterName, schemaName string) string {
	return fmt.Sprintf(`
resource "laravelcloud_database_cluster" "parent" {
  organization_id       = %q
  name                  = %q
  region                = %q
  engine                = "postgres-16"
  size                  = "db.small"
  high_availability     = false
  backup_retention_days = 7
}

resource "laravelcloud_database_schema" "test" {
  cluster_id = laravelcloud_database_cluster.parent.id
  name       = %q
}
`, orgID, clusterName, testAccRegion(), schemaName)
}
