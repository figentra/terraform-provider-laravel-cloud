# Contributing to `terraform-provider-laravel-cloud`

Thanks for helping improve the Laravel Cloud Terraform provider. This
document names the conventions every PR is measured against so
reviewers + contributors converge quickly.

## Prerequisites

- **Go 1.24+** — matches the `toolchain` directive in `go.mod`.
- **Terraform 1.5+** — the plugin-framework baseline.
- **A Laravel Cloud API token** — required for acceptance tests. Generate
  one at https://cloud.laravel.com/settings/api-tokens.

Install the dev-time tooling with:

```sh
make install-tools
```

This installs `tfplugindocs`, `golangci-lint`, and `goreleaser` to
`$GOBIN`.

## Development loop

```sh
# 1) Make your changes under internal/…

# 2) Format + lint.
make fmt lint

# 3) Run unit tests (no external state).
make test

# 4) Regenerate docs if you touched a schema.
make docs

# 5) Run acceptance tests (requires a real Cloud tenant).
export LARAVEL_CLOUD_TOKEN="…"
export LARAVEL_CLOUD_TEST_ORG_ID="org_…"
make testacc

# 6) Dry-run a GoReleaser build before opening the PR.
make release-check
```

## Adding a new resource

Every new resource ships in one PR with these five artefacts:

1. `internal/api/<name>.go` — typed request/response DTOs + Create /
   Get / Update / Delete methods.
2. `internal/provider/resource_<name>.go` — the Terraform Plugin
   Framework resource implementation. Every field carries a JSDoc-style
   comment + the Schema attribute carries a `MarkdownDescription`.
3. `internal/provider/data_source_<name>.go` — matching data source with
   every field Computed (except `id`, which is Required).
4. `examples/resources/laravelcloud_<name>/resource.tf` +
   `examples/data-sources/laravelcloud_<name>/data-source.tf` — real
   HCL callers can copy-paste.
5. `internal/provider/resource_<name>_test.go` — acceptance-test
   scaffold covering Create → Read → Update → Import → Delete.

Run `make docs` afterwards so `docs/resources/<name>.md` +
`docs/data-sources/<name>.md` regenerate.

Also register the resource in `internal/provider/provider.go`'s
`Resources()` + `DataSources()` slices.

## Code style

- **Docblocks on every file** — a package-level docblock naming what
  the file is for, then per-symbol JSDoc-style comments.
- **Detailed inline comments** on non-obvious branches, guard clauses,
  or gotchas — the reviewer should never have to grep to understand a
  decision.
- **Every export gets a name-suffixed file** — a service class lives in
  `<name>.service.go`, an interface in `<name>.interface.go`. Don't
  bundle mixed exports in one file.
- **`golangci-lint` clean** — enforced by CI. Run `make lint` locally
  before pushing.
- **`gofmt` clean** — enforced by CI. Run `make fmt` locally before
  pushing.

## Commit messages

Conventional commits — same as the parent workspace:

- `feat(provider): …` for new features
- `fix(provider): …` for bug fixes
- `docs(provider): …` for documentation-only changes
- `test(provider): …` for test-only changes
- `refactor(provider): …` for non-behavioural changes

The subject line is 72 chars max, imperative mood, no trailing period.

## Filing an issue

Use the appropriate template under `.github/ISSUE_TEMPLATE/`:

- **Bug report** — a resource that misbehaves against the Cloud API.
- **Feature request** — a new resource / data source / attribute.

## Opening a PR

Reference the issue you're closing in the PR description. If your PR
adds a new attribute or resource, include:

- The `terraform plan` output showing the new attribute rendering.
- A screenshot of the Cloud dashboard confirming the change landed.

## Releasing

Only maintainers can cut releases. The flow is:

1. Update `CHANGELOG.md` with the new version's delta.
2. `git tag -s vX.Y.Z -m "release vX.Y.Z"` — the `-s` flag GPG-signs.
3. `git push origin vX.Y.Z`.
4. GoReleaser publishes the release to GitHub + the Terraform Registry
   indexes it within ~5 minutes.

## Getting help

Open a discussion on the repo, or ping @akouta on the workspace
`#terraform` channel. Bug reports welcome via the issue templates.
