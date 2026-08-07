# Manages a Laravel Cloud cache instance (Valkey / Redis-compatible).
#
# Caches are shared across environments — bind via `cache_id` on
# `laravelcloud_environment`. Cloud injects REDIS_* env-vars into every
# runtime that references the binding.
#
# Import via: terraform import laravelcloud_cache.shared_prd <id>

resource "laravelcloud_cache" "shared_prd" {
  # Immutable — owning organisation.
  organization_id = data.laravelcloud_organization.figentra.id

  # Human-readable name shown in the Cloud dashboard.
  name = "shared-prd"

  # Immutable — deploy region. Match your applications' region.
  region = "us-east-1"

  # Cloud size slug. Mutable — bumping resizes the underlying host.
  # Common values: "valkey-pro.1gb", "valkey-pro.5gb", "valkey-pro.20gb".
  size = "valkey-pro.5gb"
}

output "shared_prd_cache_id" {
  value = laravelcloud_cache.shared_prd.id
}
