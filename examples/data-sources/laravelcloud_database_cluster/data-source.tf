# Reads an existing Laravel Cloud database cluster by ID.
#
# Common use case: an app-scoped module needs to reference a SHARED
# cluster provisioned by a platform-team-managed root module. Use this
# data source to hydrate cluster metadata without importing.

data "laravelcloud_database_cluster" "shared_prd" {
  id = "dbc_01HAM..."
}

output "shared_prd_engine" {
  value = data.laravelcloud_database_cluster.shared_prd.engine
}

output "shared_prd_ha_enabled" {
  value = data.laravelcloud_database_cluster.shared_prd.high_availability
}
