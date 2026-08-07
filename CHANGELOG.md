# Changelog

All notable changes to the Terraform Provider for Laravel Cloud are documented
in this file. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Version numbers follow [SemVer 2.0](https://semver.org/).

## [Unreleased]

Phase 3 wires third-party providers (Cloudflare / Doppler / Sentry /
PagerDuty / Better Stack) into env-root compositions. See the
[migration plan](../../.kiro/plans/2026-08-07-terraform-pivot-plan.md).

## [0.2.0] — 2026-08-07 (planned) — Phase 2 resource coverage

### Added

Every Cloud primitive the workspace's `.kiro/cloud/apps/*.yaml` manifests
declare has a matching resource. Full CRUD + import + drift-tolerant Read
on each:

- `laravelcloud_environment` — per-app deploy target with branch + vars +
  inheritance.
- `laravelcloud_database_cluster` — shared Postgres/MySQL cluster.
- `laravelcloud_database_schema` — logical database inside a cluster.
  Immutable — every attribute forces replace.
- `laravelcloud_cache` — Valkey/Redis cache; only `size` is mutable.
- `laravelcloud_bucket` — S3-compatible object storage; `mode` toggles
  private/public.
- `laravelcloud_websocket_cluster` — Reverb-compatible WS cluster.
- `laravelcloud_websocket_app` — per-environment WS app binding.
- `laravelcloud_domain` — custom hostname bound to an env; `redirect_from_www`
  - `wildcard_enabled` are mutable, hostname is immutable.

### Notes

- Data sources for the new resource types land in v0.2.1 alongside the
  first tag. v0.2.0 ships resources only.
- Every resource follows the same pattern as `laravelcloud_application`:
  strict CRUD, `RequiresReplace` on immutable fields, drift-tolerant Read
  (`404` drops the resource from state), `ImportStatePassthroughID`.
- Acceptance test for `laravelcloud_application` is the canonical example;
  additional tests follow the same shape in the same file's directory.

## [0.1.0] — 2026-08-07 — Phase 1 skeleton (ADR-0080)

### Added

- Provider config schema with 6-step token priority chain (block value →
  org-scoped env var → generic env var → explicit token file → default
  `.kiro/cloud/token` → error diagnostic).
- HTTP client with exponential backoff on 429 + retry on 5xx.
- `laravelcloud_application` resource with Create/Read/Update/Delete/Import.
  Immutable fields (`organization_id`, `region`, `source_control_provider_type`)
  carry `RequiresReplace` plan modifiers.
- `data.laravelcloud_application` data source (lookup by ID).
- Acceptance test covering the application CRUD + import lifecycle.
- GoReleaser + Terraform Registry publishing pipeline.
- Generated docs via `tfplugindocs`.

### Notes

- This is the FIRST release under `figentra/laravel-cloud` on the Registry.
- Consumers should pin `version = "~> 0.1"` to receive Phase 2 patches
  without breaking-change surprises.
- Cloud CLI's write commands (`cloud:apply`, `cloud:destroy`, `cloud:sync`,
  `cloud:bootstrap`) are **not yet deprecated** — the deprecation banner
  lands in Phase 5 of the migration plan once resource coverage is complete.
