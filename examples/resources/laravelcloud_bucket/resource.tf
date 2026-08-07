# Manages a Laravel Cloud S3-compatible bucket.
#
# Buckets sit at the organisation scope, not per-environment — bind via
# S3_* env-vars on the environments that need access. Cloud rejects a
# `terraform destroy` when the bucket is non-empty; empty it via the CLI or
# Cloud dashboard before applying.
#
# Import via: terraform import laravelcloud_bucket.uploads_prd <id>

resource "laravelcloud_bucket" "uploads_prd" {
  # Immutable — owning organisation.
  organization_id = data.laravelcloud_organization.figentra.id

  # Immutable — bucket name. Cloud enforces global uniqueness.
  name = "figentra-identity-uploads-prd"

  # Immutable — deploy region. Match your applications' region.
  region = "us-east-1"

  # Access mode. "private" requires Cloud-signed URLs for reads; "public"
  # allows unauthenticated GETs. Mutable post-create.
  mode = "private"
}

output "uploads_prd_bucket_id" {
  value = laravelcloud_bucket.uploads_prd.id
}
