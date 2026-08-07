# Manages a Laravel Cloud application.
#
# The application is the top-level deploy unit — every environment,
# database binding, cache binding, WS binding, and domain hangs off an
# application. Import via `terraform import laravelcloud_application.foo <id>`.

resource "laravelcloud_application" "identity" {
  # Immutable — Cloud organisation this app belongs to.
  organization_id = "org_01HAM..."

  # Displayed in the Cloud dashboard + used to derive the slug.
  name = "identity"

  # Immutable — deploy region. See Cloud docs for the current list.
  region = "us-east-1"

  # Immutable — one of "github", "gitlab", "bitbucket".
  source_control_provider_type = "github"

  # Optional — `owner/repo` shape.
  repository = "figentra/identity-service"

  # Optional — Slack channel for deploy notifications.
  slack_channel = "#deploys-identity"
}

# Reference the created application from downstream resources.
output "identity_slug" {
  value = laravelcloud_application.identity.slug
}

output "identity_id" {
  value = laravelcloud_application.identity.id
}
