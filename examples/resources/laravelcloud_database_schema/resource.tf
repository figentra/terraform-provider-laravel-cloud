# Manages a logical database (Postgres schema / MySQL database) inside a
# Laravel Cloud database cluster.
#
# One schema per application-environment pair is the workspace convention —
# `identity_prd`, `identity_stg`, `identity_dev`, etc. Schemas are immutable
# post-create; renaming requires destroy + recreate (which drops all data).
#
# Import via: terraform import laravelcloud_database_schema.identity_prd <id>

resource "laravelcloud_database_schema" "identity_prd" {
  # Immutable — parent cluster.
  cluster_id = laravelcloud_database_cluster.shared_prd.id

  # Immutable — logical database name inside the cluster.
  # Cloud enforces uniqueness per-cluster.
  name = "identity_prd"
}

output "identity_prd_schema_id" {
  value = laravelcloud_database_schema.identity_prd.id
}
