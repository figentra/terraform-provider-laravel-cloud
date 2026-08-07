# Manages a Laravel Cloud database cluster.
#
# A cluster is a shared Postgres / MySQL server that hosts one or more logical
# schemas (see laravelcloud_database_schema). The `high_availability` flag +
# `backup_retention_days` are the two knobs that upgrade a dev-tier cluster
# into a production-ready one.
#
# Import via: terraform import laravelcloud_database_cluster.shared_prd <id>

resource "laravelcloud_database_cluster" "shared_prd" {
  # Immutable — owning organisation.
  organization_id = data.laravelcloud_organization.figentra.id

  # Human-readable name shown in the Cloud dashboard.
  name = "shared-prd"

  # Immutable — deploy region. Match your applications' region.
  region = "us-east-1"

  # Immutable — DB engine + major version.
  # Common values: "postgres-16", "postgres-17", "mysql-8".
  engine = "postgres-16"

  # Cloud size slug. Mutable — bumping resizes the underlying host.
  # Common values: "db.small", "db.medium", "db.large", "db.xlarge".
  size = "db.medium"

  # High-availability replica. Recommended for production.
  high_availability = true

  # Retention window for automatic daily backups.
  # Cloud caps at 35 days on the standard tier.
  backup_retention_days = 30
}

output "shared_prd_cluster_id" {
  value = laravelcloud_database_cluster.shared_prd.id
}
