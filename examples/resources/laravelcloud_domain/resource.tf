# Manages a custom-hostname binding for a Laravel Cloud environment.
#
# The binding tells Cloud's edge to route `<name>` (and optionally
# `www.<name>` + `*.<name>`) to the environment. DNS still lives outside
# Cloud — point a CNAME to the value Cloud publishes on the domain page.
#
# Import via: terraform import laravelcloud_domain.production <id>

resource "laravelcloud_domain" "production" {
  # Immutable — bound environment.
  environment_id = laravelcloud_environment.production.id

  # Immutable — fully-qualified hostname. Cloud enforces global uniqueness.
  name = "identity.figentra.com"

  # Redirect `www.<name>` → `<name>` at the Cloud edge. Mutable post-create.
  redirect_from_www = true

  # Route every `*.<name>` subdomain to this env (multi-tenant SaaS pattern).
  # Wildcard TLS is issued automatically by Cloud when true.
  wildcard_enabled = false

  # Cloud-coordinated Cloudflare orange-cloud proxying. Cloud creates the
  # CF record on your behalf when true (requires a Cloudflare integration).
  cloudflare_managed = false

  # DNS verification mode:
  #   real_time — Cloud polls DNS + auto-flips to `verified` when the CNAME lands
  #   manual    — operator confirms verification in the dashboard
  verification = "real_time"
}

output "production_domain_id" {
  value = laravelcloud_domain.production.id
}
