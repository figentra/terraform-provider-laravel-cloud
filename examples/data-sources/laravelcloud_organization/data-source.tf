# Reads a Laravel Cloud organisation.
#
# Three lookup patterns depending on what the caller knows up front:
#
#   1) By slug — the workspace-canonical pattern:
#      data "laravelcloud_organization" "figentra" { slug = "figentra" }
#
#   2) By ID — when you already carry the ULID:
#      data "laravelcloud_organization" "figentra" { id = "org_01HAM..." }
#
#   3) Token-scoped default — no attributes; resolves whichever org the
#      current API token was minted for via GET /meta/organization:
#      data "laravelcloud_organization" "current" {}
#
# Every resource that needs `organization_id` (application, database
# cluster, cache, bucket, websocket cluster) can reference
# `.id` off this data source without hard-coding the ULID in HCL.

data "laravelcloud_organization" "figentra" {
  slug = "figentra"
}

data "laravelcloud_organization" "current" {}

output "figentra_org_id" {
  value = data.laravelcloud_organization.figentra.id
}

output "current_org_name" {
  value = data.laravelcloud_organization.current.name
}
