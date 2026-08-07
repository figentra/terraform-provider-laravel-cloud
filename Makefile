# Makefile — Terraform Provider for Laravel Cloud
#
# Every target is idempotent + composable. The `install` target places
# the built binary into Terraform's per-user plugin cache so operators
# can iterate on the provider locally without publishing to Registry.
#
# Common workflows:
#   make build           — compile the provider binary
#   make install         — build + place under ~/.terraform.d/plugins/
#   make test            — run unit tests (no TF_ACC)
#   make testacc         — run acceptance tests (requires LARAVEL_CLOUD_TOKEN)
#   make generate        — regenerate docs via tfplugindocs
#   make lint            — golangci-lint check
#   make release         — GoReleaser dry-run (real releases run in CI)

# The Registry namespace + name. Matches the go.mod path + Registry publish.
NAMESPACE   := figentra
NAME        := laravel-cloud
BINARY      := terraform-provider-$(NAME)

# Version stamped into the binary via ldflags. Overridden by GoReleaser at
# release time to the git tag.
VERSION     ?= dev

# Host + arch for the `install` target — plugin-framework's dev override.
HOSTNAME    := registry.terraform.io
OS_ARCH     := $(shell go env GOOS)_$(shell go env GOARCH)

# Terraform's per-user plugin cache root. `terraform init` looks here
# before hitting Registry when a `dev_overrides` block is set in
# `~/.terraformrc`.
PLUGIN_DIR  := ~/.terraform.d/plugins/$(HOSTNAME)/$(NAMESPACE)/$(NAME)/$(VERSION)/$(OS_ARCH)

.PHONY: default build install test testacc generate lint release clean

default: build

# Compile the provider. `-ldflags` stamps the version string into main.go.
build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY)

# Install into Terraform's per-user plugin cache.
# Operators wire `~/.terraformrc` to point Terraform at the local build:
#
#   provider_installation {
#     dev_overrides {
#       "figentra/laravel-cloud" = "~/.terraform.d/plugins/registry.terraform.io/figentra/laravel-cloud/dev/darwin_arm64"
#     }
#     direct {}
#   }
install: build
	mkdir -p $(PLUGIN_DIR)
	cp $(BINARY) $(PLUGIN_DIR)/

# Unit tests — no network access, no Cloud API calls.
test:
	go test -v -race ./...

# Acceptance tests — real Cloud API calls against the dev org.
# CI runs this with a dedicated `.kiro/cloud/token-acceptance-test` PAT
# scoped to the test-only Cloud org so accidental damage is contained.
testacc:
	TF_ACC=1 go test -v -race -timeout 30m ./internal/provider/...

# Regenerate the `docs/` directory via HashiCorp's tfplugindocs tool.
# Runs before every release; docs are committed to the repo so the
# Registry can display them without triggering a build.
generate:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate

# golangci-lint over the whole tree. Config lives at .golangci.yml (add
# per team standards — the default preset catches most issues).
lint:
	golangci-lint run ./...

# GoReleaser dry-run. Real releases fire from CI (see
# .github/workflows/release.yml when it lands in Phase 1.12).
release:
	goreleaser release --snapshot --clean --skip=publish

# Wipe build artefacts + local plugin cache for this provider.
clean:
	rm -f $(BINARY)
	rm -rf $(PLUGIN_DIR)
	rm -rf dist/
