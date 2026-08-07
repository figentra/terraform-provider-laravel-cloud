# Provider configuration for the Laravel Cloud provider.
#
# The `token` attribute falls back through a 6-step priority chain — see
# the README §Authentication for the full ladder:
#
#   1. `provider.token` block attribute
#   2. `<CLOUD_ORG>_LARAVEL_CLOUD_TOKEN` env var (when `cloud_org` set)
#   3. `LARAVEL_CLOUD_TOKEN` env var
#   4. `provider.token_file` contents
#   5. `.kiro/cloud/token` contents (workspace-canonical default)
#   6. Error diagnostic
#
# The chain lets a workstation hold multiple org tokens simultaneously
# (FIGENTRA_LARAVEL_CLOUD_TOKEN + ACADEMORIX_LARAVEL_CLOUD_TOKEN) while
# CI runners use the generic LARAVEL_CLOUD_TOKEN.

terraform {
  required_version = ">= 1.5.0"

  required_providers {
    laravelcloud = {
      source  = "figentra/laravel-cloud"
      version = "~> 0.3"
    }
  }
}

provider "laravelcloud" {
  # cloud_org drives the <ORG>_LARAVEL_CLOUD_TOKEN env-var fallback.
  # Setting this to "figentra" makes the provider look for
  # FIGENTRA_LARAVEL_CLOUD_TOKEN before falling back to the generic
  # LARAVEL_CLOUD_TOKEN.
  cloud_org = "figentra"

  # Uncomment to override for staging + preview builds. Defaults to
  # https://cloud.laravel.com/api when unset.
  # base_url = "https://cloud-staging.laravel.com/api"

  # Uncomment to override the workspace-canonical token file path.
  # Defaults to `.kiro/cloud/token`.
  # token_file = "/path/to/custom-token"

  # Per-request HTTP timeout in seconds. Defaults to 60. Raise for large
  # applies against slow WAN links; lower for CI that fails-fast.
  # timeout = 120
}

# Discover the current-token's organisation without hard-coding the ULID.
data "laravelcloud_organization" "current" {}

# Every resource below picks up `data.laravelcloud_organization.current.id`
# as its `organization_id` binding — see the per-resource examples for the
# full patterns.
