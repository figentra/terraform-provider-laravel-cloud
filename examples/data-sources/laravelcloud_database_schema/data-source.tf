# Reads an existing Laravel Cloud database schema by ID.

data "laravelcloud_database_schema" "identity_prd" {
  id = "dbs_01HAM..."
}

output "identity_prd_schema_name" {
  value = data.laravelcloud_database_schema.identity_prd.name
}
