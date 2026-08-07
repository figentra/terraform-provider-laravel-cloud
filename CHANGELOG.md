# Changelog

All notable changes to the Terraform Provider for Laravel Cloud are documented
in this file. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Version numbers follow [SemVer 2.0](https://semver.org/).

## [Unreleased]

Phase 2 lands additional resources — see the
[migration plan](../../.kiro/plans/2026-08-07-terraform-pivot-plan.md).

## [0.1.0] — 2026-08-07 (planned first release)

**Phase 1 — Provider skeleton (ADR-0080)**

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

- This is the FIRST release under `stackra/laravel-cloud` on the Registry.
- Consumers should pin `version = "~> 0.1"` to receive Phase 2 patches
  without breaking-change surprises.
- Cloud CLI's write commands (`cloud:apply`, `cloud:destroy`, `cloud:sync`,
  `cloud:bootstrap`) are **not yet deprecated** — the deprecation banner
  lands in Phase 5 of the migration plan once resource coverage is complete.
