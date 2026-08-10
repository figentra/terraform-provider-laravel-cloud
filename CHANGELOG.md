# Changelog

All notable changes to the Terraform Provider for Laravel Cloud are documented
in this file. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Version numbers follow [SemVer 2.0](https://semver.org/).

## [Unreleased]

## [0.5.0] — 2026-08-11 — deploy runtime primitives (BREAKING minor: 7 new resources)

Wire-format-safe minor bump — every existing resource keeps its current
schema + API surface. Seven brand-new resource types unlock the actual
Cloud runtime (deployments, compute sizing, background processes, network
hardening, domain verification, DB snapshots, one-shot commands) that the
v0.4.x provider only scaffolded.

### Added

- **`laravelcloud_deployment`** — Trigger deploys from Terraform.
  Cloud does NOT auto-deploy on env creation; this resource fires
  `POST /environments/:id/deployments` and (optionally) polls for
  terminal status. `redeploy_trigger` attribute forces re-deploy
  when it changes (`timestamp()`, `-var="redeploy_trigger=$(date +%s)"`).
  Failed deploys emit a diagnostic with `deployment_id`, terminal
  status, and failure reason so operators can inspect the Cloud
  dashboard. Retry contract: bounded 1200-second poll (bumpable via
  `timeout_seconds`), 5s cadence, `wait_for_completion = false` for
  fire-and-forget. Every field except `wait_for_completion` and
  `timeout_seconds` carries RequiresReplace so each terraform apply
  can only create a fresh deployment record — matches Cloud's own
  semantics (deployments are immutable rows).

- **`laravelcloud_instance`** — Compute-unit sizing + Octane + autoscale
  - hibernation. An environment carries at least one instance
    (`type = app`) plus optionally `queue` / `service` / `serverless_queue`
    instances for Horizon workers, custom daemons, and serverless jobs.
    Every InstanceSize slug from the SDK enum is accepted
    (`flex.g-1vcpu-512mb` through `dedicated.m-8vcpu-64gb`).
    Toggleable knobs: `uses_octane`, `uses_scheduler`, `uses_sleep_mode`
    (Cloud scale-to-zero), `uses_inertia_ssr`. Autoscale via
    `scaling_type` + `min_replicas` + `max_replicas` + optional CPU/mem
    thresholds. Fixes the "Cloud picks 512MB default and OOMs under
    load" trap.

- **`laravelcloud_background_process`** — Horizon workers + custom
  daemons on an instance. Two shapes: `type = "worker"` (Laravel
  queue worker with `config` block carrying `connection`, `queue`,
  `tries`, `backoff`, `sleep`, `rest`, `timeout`, `force`) or
  `type = "custom"` (any long-lived process; `command` REQUIRED).
  Fixes the "app deploys but no jobs run" trap.

- **`laravelcloud_environment_network_settings`** — Env-scoped HSTS
  - rate-limit tier + robots_tag + X-Frame-Options + X-Content-Type
  - firewall under-attack mode. Separate resource so network hardening
    has its own lifecycle (typical pattern: dev = noindex + rate_limit
    none + no HSTS, stg = noindex + rate_limit low + HSTS 1yr, prd =
    index + rate_limit high + HSTS 2yr + preload + include_subdomains).
    Delete PATCHes the env back to Cloud's permissive defaults; the
    environment itself is untouched. On the wire this hits
    `PATCH /environments/:id` with a disjoint field set from
    `laravelcloud_environment`, so both resources can co-manage the
    same env without conflict.

- **`laravelcloud_domain_verify`** — Fire `POST /domains/:id/verify`
  as an explicit Terraform action. Attach a `depends_on` chain from
  the DNS record so verification only fires after propagation. Bump
  `verify_trigger` to re-run. Fixes the "domain stuck on Not connected
  until Cloud auto-polls" wait.

- **`laravelcloud_database_snapshot`** — Manual snapshots on a database
  cluster. Cloud's automated snapshots are managed by Cloud; this
  resource manages operator-initiated snapshots (pre-migration safety
  nets, DR test artifacts). Cluster-scoped: import path is
  `<cluster_id>:<snapshot_id>`.

- **`laravelcloud_command`** — One-shot artisan/shell commands in an
  environment (`migrate --force`, `db:seed --class=X`,
  `cache:clear`, `tinker`). Bump `rerun_trigger` to re-run.
  Non-happy terminal exits emit a diagnostic with captured
  stdout+stderr (capped at 4KB in the diagnostic; full output stays
  in state). Delete is a no-op — Cloud retains the command record
  for history.

### Wire additions

- New API client methods: `CreateDeployment`, `GetDeployment`,
  `PollDeployment`, `CreateInstance`, `GetInstance`,
  `UpdateInstance`, `DeleteInstance`, `CreateBackgroundProcess`,
  `GetBackgroundProcess`, `UpdateBackgroundProcess`,
  `DeleteBackgroundProcess`, `GetEnvironmentNetworkSettings`,
  `UpdateEnvironmentNetworkSettings`, `VerifyDomain`,
  `CreateDatabaseSnapshot`, `GetDatabaseSnapshot`,
  `DeleteDatabaseSnapshot`, `ListDatabaseSnapshots`, `RunCommand`,
  `GetCommand`, `PollCommand`.
- New response structs: `Deployment`, `Instance`, `BackgroundProcess`
  (+ nested `BackgroundProcessConfig`), `EnvironmentNetworkSettings`
  (+ nested `HstsSettings`), `DatabaseSnapshot`, `Command`.

### Semver rationale

`0.5.0` is a MINOR bump under 0.x semver (no breaking changes to
existing resources). Consumers on `~> 0.4` upgrade freely; the seven
new resources are additive.

## [0.4.10] — 2026-08-11 — cluster-delete race retry

### Fixed

- **`laravelcloud_database_cluster` DELETE now survives Cloud's
  eventual-consistency race on schema-attachment.** Terraform's DAG
  destroys child schemas before the parent cluster, but Cloud's
  internal `cluster.has_schemas` guard lags behind the schema
  DELETEs by up to several seconds. Every child schema DELETE
  returns 200, the cascade LIST returns 0 schemas, and the cluster
  DELETE still hits `HTTP 422: The database cluster has schemas
attached and cannot be deleted.` The v0.4.9 cascade-list-and-
  delete pass was the right shape but a one-shot — it didn't survive
  the race.
- `DeleteDatabaseCluster` now wraps its cascade + cluster-DELETE
  pass in a bounded retry loop (5 attempts, exponential backoff
  500ms → 8s cap, ≈ 15s worst-case). Every retry re-lists +
  re-drains schemas before firing the cluster DELETE again. A
  genuinely-broken Cloud state (persistent 422 across every retry)
  fails loudly with the last error surfaced verbatim — the loop
  doesn't stall a plan indefinitely.
- New `(*APIError).IsSchemasAttached()` predicate detects the
  specific 422 by message pattern. Isolated from the generic 422
  handling so the retry loop keys on this exact race, not on any
  422 (which could be a legitimately-broken destroy the operator
  needs to see).
- 404s on the cluster DELETE are now treated as idempotent — a
  cluster already gone from a previous partially-successful destroy
  no longer errors the plan.

## [0.4.9] — 2026-08-10 — cluster cascade-delete for orphaned schemas

## [0.4.1] — 2026-08-10 — JSON:API envelope flatten

### Fixed

- **CRITICAL — every resource with computed attributes was reading null.**
  `Envelope[T].UnmarshalJSON` was treating Cloud's JSON:API response wrapper
  (`{"data": {"id": …, "type": …, "attributes": {…}}}`) as flat REST, so every
  Read/Create call left `.name`, `.region`, `.size`, `.type`, `.is_public`,
  `.max_connections`, etc. at Go zero values (`""` / `nil`). Terraform's
  post-apply consistency check flagged this as "Provider produced inconsistent
  result after apply". Affected every `laravelcloud_*` resource create/read.
- Custom `Envelope.UnmarshalJSON` now transparently flattens the JSON:API
  envelope: hoists `data.attributes.*` up + copies `data.id`, then unmarshals
  into `T`. The JSON:API resource-type discriminator (`data.type = "caches"`)
  is deliberately dropped so `Cache.Type` reads the engine value
  (`"laravel_valkey"`) from `data.attributes.type` instead of the discriminator.
- Supports singleton (`data` is object) + list (`data` is array) responses.
- Flat REST responses (no `attributes` sub-object) pass through unchanged for
  forward compat.

## [0.4.0] — 2026-08-04 — Cloud v2 attribute expansion

### Added

- `laravelcloud_bucket` — four new attributes matching Cloud's v2 API:
  - `visibility` — canonical private/public flag; supersedes `mode`.
  - `jurisdiction` — geographic zone slug (`eu`, `us`, `ap`, ...).
  - `key_name` — identifier of the auto-generated access key Cloud
    mints alongside the bucket.
  - `key_permission` — permission level of the generated key
    (`read_write` default, `read`, `write`).
- `laravelcloud_application.root_directory` — sub-directory inside the
  repo Cloud treats as the build root (`apps/api`, `packages/dashboard`).
  Consumers previously had to work around this by re-rooting the git
  repository per app; now expressible per-application via the provider.
- Data sources (`data.laravelcloud_bucket`, `data.laravelcloud_application`)
  hydrate every new attribute.

### Changed

- `laravelcloud_bucket.mode` — kept for backward compatibility but
  formally deprecated. Consumers should migrate to `visibility`. The
  provider aliases both fields onto the wire so callers using either
  shape produce byte-identical Cloud API requests.
- `laravelcloud_bucket.region` — now Optional + Computed. Cloud derives
  from the organisation when omitted. Previously Required (breaking-ish
  relaxation — no plan diff for callers still setting it).
- `laravelcloud_application.organization_id` — now Optional + Computed.
  Cloud infers from the token when omitted. Previously Required
  (breaking-ish relaxation — no plan diff for callers still setting it).

### Notes

- v0.4.0 is the FIRST release with the Cloud v2 API surface. Consumers
  bumping from `~> 0.3` → `~> 0.4` see NO breaking changes; every
  pre-v0.4 HCL keeps working.
- The workspace's `figentra/service/laravelcloud` module + sibling
  static-site module (both at v0.1.3) require `~> 0.4` to consume the
  new bucket + application attributes.

## [0.3.0] — 2026-08-07 — Phase 2 data-source coverage + provider hygiene

## [0.3.0] — 2026-08-07 — Phase 2 data-source coverage + provider hygiene

### Added

- **Nine new data sources** — one per resource type introduced in v0.2.0
  plus a token-scoped organisation lookup:
  - `data.laravelcloud_environment`
  - `data.laravelcloud_database_cluster`
  - `data.laravelcloud_database_schema`
  - `data.laravelcloud_cache`
  - `data.laravelcloud_bucket`
  - `data.laravelcloud_websocket_cluster`
  - `data.laravelcloud_websocket_app`
  - `data.laravelcloud_domain`
  - `data.laravelcloud_organization` — accepts `slug`, `id`, or falls
    back to `GET /meta/organization` when both are omitted.
- **Per-resource examples** under `examples/resources/laravelcloud_*/` +
  matching **per-data-source examples** under
  `examples/data-sources/laravelcloud_*/` — every resource + data source
  ships a runnable HCL example.
- **`docs/` directory generated by `tfplugindocs`** — 20 doc pages
  (index + 9 resources + 10 data sources), ready for Terraform Registry
  indexing.
- **Acceptance-test scaffolds** for every resource in
  `internal/provider/resource_*_test.go` — TF_ACC-gated, covers Create →
  Read → Update → Import → Delete.
- **Mock-server unit tests** in `internal/api/client_test.go` — round-
  trips every resource type's envelope through an in-process
  `httptest.NewServer`, plus verifies retry-on-429 + typed `*APIError`
  surface.
- **Hygiene files**: `GNUmakefile` (build/test/lint/docs/release
  targets), `CONTRIBUTING.md`, `CODEOWNERS`, `.golangci.yml`,
  `.github/dependabot.yml`, and the `.github/ISSUE_TEMPLATE/` +
  `.github/PULL_REQUEST_TEMPLATE.md` set.

### Changed

- `provider "laravelcloud"` example now declares
  `version = "~> 0.3"` and pins `required_version = ">= 1.5.0"`.
- `internal/api/organizations.go` — extended the API client with
  `GetOrganization(slug)` + `GetDefaultOrganization()`.
- `internal/provider/provider_test.go` — extracted shared
  `testAccProtoV6ProviderFactories` + `testAccPreCheck` helpers so
  every acceptance test consumes the same wiring.

### Notes

- Every new file carries a package-level docblock + per-field JSDoc +
  section comments per the workspace's docblock rule.
- No breaking changes vs. v0.2.0 — every existing HCL keeps working.
  Consumers can bump `~> 0.2` → `~> 0.3` without config edits.

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
