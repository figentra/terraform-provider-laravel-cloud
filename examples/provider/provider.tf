# Basic provider configuration.
#
# `token` falls back through the 6-step priority chain — see README §Authentication.
# For CI, set LARAVEL_CLOUD_TOKEN in the env. For local dev, drop the token
# into `.kiro/cloud/token`.

terraform {
  required_providers {
    laravelcloud = {
      source  = "stackra/laravel-cloud"
      version = "~> 0.1"
    }
  }
}

provider "laravelcloud" {
  # cloud_org drives the <ORG>_LARAVEL_CLOUD_TOKEN env-var fallback.
  # Setting this to "figentra" makes the provider look for
  # FIGENTRA_LARAVEL_CLOUD_TOKEN before falling back to the generic
  # LARAVEL_CLOUD_TOKEN. Useful when a workstation has tokens for
  # multiple Cloud orgs.
  cloud_org = "figentra"

  # Override for staging + preview builds. Defaults to
  # https://cloud.laravel.com/api when unset.
  # base_url = "https://cloud-staging.laravel.com/api"

  # Per-request timeout in seconds. Defaults to 60.
  # timeout = 120
}
