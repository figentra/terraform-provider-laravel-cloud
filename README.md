---
authored_by: kiro
authored_at: 2026-08-07
source: prompt://enterprise-close-out-terraform-pivot
reviewed_by: null
reviewed_at: null
---

# Terraform Provider for Laravel Cloud

Manage [Laravel Cloud](https://cloud.laravel.com) applications, environments,
database clusters, caches, buckets, WebSocket clusters, and domains via
Terraform.

Published as
[`registry.terraform.io/figentra/laravel-cloud`](https://registry.terraform.io/providers/figentra/laravel-cloud).

## Status

**v0.1.0 (Phase 1 — provider skeleton)**. This release ships one working
resource (`laravelcloud_application`) + one data source
(`data.laravelcloud_application`). Phase 2 lands the full resource set —
see the [migration plan](../../.kiro/plans/2026-08-07-terraform-pivot-plan.md).

## Why

The workspace previously drove Cloud writes through the PHP CLI's
`cloud:apply` / `cloud:destroy` / `cloud:sync` / `cloud:bootstrap` commands
(ADR-0079). We're pivoting to Terraform per
[ADR-0080](../../.docs/adr/0080-terraform-for-cloud-devops.md) so:

- The same IaC substrate (Terraform + HCL) drives Laravel Cloud, Cloudflare,
  Doppler, Sentry, PagerDuty, and Better Stack — one plan, one apply, one
  state.
- Standard operator UX + Registry publishing shortens the learning curve
  for new engineers.
- Read commands stay in the PHP CLI (`cloud:state:show`, `state:diff`,
  `evidence`, `apps`, `status`, `tail`, `tinker`, `auth`) because they add
  workspace-specific reporting the Terraform provider doesn't.

## Requirements

- Terraform 1.9+ (or OpenTofu 1.8+)
- Go 1.22+ (only to build from source)
- A Laravel Cloud API token
  ([generate](https://cloud.laravel.com/settings/api-tokens))

## Usage

```hcl
terraform {
  required_providers {
    laravelcloud = {
      source  = "figentra/laravel-cloud"
      version = "~> 0.1"
    }
  }
}

provider "laravelcloud" {
  # Falls back to the LARAVEL_CLOUD_TOKEN env var when unset.
  # cloud_org drives the <ORG>_LARAVEL_CLOUD_TOKEN fallback.
  cloud_org = "figentra"
}

resource "laravelcloud_application" "identity" {
  organization_id              = "org_01HAM..."
  name                         = "identity"
  region                       = "us-east-1"
  source_control_provider_type = "github"
  repository                   = "figentra/identity-service"
  slack_channel                = "#deploys-identity"
}

output "identity_id" {
  value = laravelcloud_application.identity.id
}
```

## Authentication

The provider resolves the API token via a 6-step priority chain (highest
wins):

1. `provider.token` block attribute
2. `<CLOUD_ORG>_LARAVEL_CLOUD_TOKEN` env var (when `cloud_org` is set)
3. `LARAVEL_CLOUD_TOKEN` env var
4. `provider.token_file` contents
5. `.kiro/cloud/token` contents (workspace-canonical default)
6. Error diagnostic — operator must supply one of the above

The chain matches the PHP CLI's token resolution shape so operators can
migrate without re-learning auth.

## Development

```sh
# Build the provider.
make build

# Install into Terraform's per-user plugin cache.
make install

# Wire ~/.terraformrc to prefer the local build:
cat >> ~/.terraformrc <<EOF
provider_installation {
  dev_overrides {
    "figentra/laravel-cloud" = "$HOME/.terraform.d/plugins/registry.terraform.io/figentra/laravel-cloud/dev/$(go env GOOS)_$(go env GOARCH)"
  }
  direct {}
}
EOF

# Run unit tests.
make test

# Run acceptance tests (requires LARAVEL_CLOUD_TOKEN + LARAVEL_CLOUD_TEST_ORG_ID).
make testacc

# Regenerate docs.
make generate
```

## Roadmap

Phase 1 (this release):

- ✅ Provider config schema mirroring PHP CLI's token chain
- ✅ HTTP client with retry + rate-limit backoff
- ✅ `laravelcloud_application` resource — Create/Read/Update/Delete/Import
- ✅ `data.laravelcloud_application` data source
- ✅ Acceptance test for the application CRUD lifecycle
- ✅ GoReleaser + Registry publishing pipeline
- ✅ Generated docs via `tfplugindocs`

Phase 2 (weeks 2-3):

- 🔲 `laravelcloud_environment`
- 🔲 `laravelcloud_database_cluster`
- 🔲 `laravelcloud_database_schema`
- 🔲 `laravelcloud_cache`
- 🔲 `laravelcloud_bucket`
- 🔲 `laravelcloud_websocket_cluster`
- 🔲 `laravelcloud_websocket_app`
- 🔲 `laravelcloud_domain`
- 🔲 Data sources for every resource + one for organizations

See the [migration plan](../../.kiro/plans/2026-08-07-terraform-pivot-plan.md)
for the full phase-by-phase breakdown.

## Contributing

This provider is authored + maintained by the
[`go-terraform-provider-builder`](../../.kiro/agents/go-terraform-provider-builder.md)
agent. Human maintainers review PRs before merge.

Read before contributing:

- Agent charter — the surface + non-surface every PR respects
- [`terraform-conventions.md`](../../.kiro/steering/terraform-conventions.md)
- [ADR-0080](../../.docs/adr/0080-terraform-for-cloud-devops.md)

## License

[MIT](LICENSE) © 2026 Figentra L.L.C.

## Cross-references

- [ADR-0080](../../.docs/adr/0080-terraform-for-cloud-devops.md) — the pivot decision
- [Migration plan](../../.kiro/plans/2026-08-07-terraform-pivot-plan.md)
- [Laravel Cloud conventions](../../.kiro/steering/laravel-cloud-conventions.md)
- [Cloudflare conventions](../../.kiro/steering/cloudflare-conventions.md)
- [Terraform conventions](../../.kiro/steering/terraform-conventions.md)
- [Redberry PHP SDK](https://github.com/RedberryProducts/laravel-cloud-sdk) — the reference we mirror for the Cloud API shape
