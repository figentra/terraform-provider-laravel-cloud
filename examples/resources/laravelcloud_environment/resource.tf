# Manages a Laravel Cloud environment.
#
# An environment lives under an application and represents a single deployable
# stage (development, staging, production, or a per-PR preview env). It carries
# a git branch binding + a per-env variable map + optional links to a database
# schema / cache / websocket app / parent env for variable inheritance.
#
# Import via: terraform import laravelcloud_environment.production <env_id>

resource "laravelcloud_environment" "production" {
  # Immutable — owning application. Change requires re-creation.
  application_id = laravelcloud_application.identity.id

  # Immutable — the environment slug (dev / staging / production / preview-*).
  name = "production"

  # Source-control branch that auto-deploys to this env. Nullable — omit for
  # envs that don't auto-deploy (marketing envs, static preview slots).
  branch = "main"

  # Per-env variable map. Values are encrypted at rest server-side. Secrets
  # should NOT be stored here in cleartext — reference Doppler + inject via
  # `${DOPPLER_TOKEN_SECRET}` style substitution instead.
  variables = {
    APP_ENV   = "production"
    APP_DEBUG = "false"
    LOG_LEVEL = "info"
  }

  # Optional — bind a database schema to this env. Cloud injects the DSN into
  # the runtime env-vars as DB_* automatically.
  database_schema_id = laravelcloud_database_schema.identity_prd.id

  # Optional — bind a cache to this env. Cloud injects REDIS_* env-vars.
  cache_id = laravelcloud_cache.shared_prd.id

  # Optional — inherit variables from a parent env. Common patterns: staging
  # inherits from development, previews inherit from development.
  # inherits_id = laravelcloud_environment.development.id
}

output "production_env_id" {
  value = laravelcloud_environment.production.id
}
