# Reads an existing Laravel Cloud application by ID.
#
# Use case: an env-root module composes a service via
# `laravel-cloud-service` but needs to reference an application created by
# another Terraform state OR by the Cloud dashboard. The data source
# hydrates the read-only state (slug, region, org binding, timestamps)
# without importing.

data "laravelcloud_application" "identity" {
  id = "app_01HAM..."
}

output "identity_slug" {
  value = data.laravelcloud_application.identity.slug
}

output "identity_region" {
  value = data.laravelcloud_application.identity.region
}
