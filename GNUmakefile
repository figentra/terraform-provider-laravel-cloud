# -----------------------------------------------------------------------------
# GNUmakefile — terraform-provider-laravel-cloud
# -----------------------------------------------------------------------------
#
# Targets:
#   build          Compile the provider binary into the local dir.
#   install        Install the binary to $GOBIN.
#   test           Run unit tests (no external state).
#   testacc        Run acceptance tests (requires TF_ACC=1 + real Cloud creds).
#   lint           Run golangci-lint (install via `make install-tools`).
#   docs           Regenerate docs/ via tfplugindocs (install via install-tools).
#   fmt            Format Go source + terraform examples.
#   install-tools  Install every dev-time tool the repo depends on.
#   release-check  Dry-run a GoReleaser build to catch config drift.
#   clean          Remove build artefacts.
#
# The file is named `GNUmakefile` (not `Makefile`) so GNU make finds it on
# every platform without a preference conflict with BSD make.
# -----------------------------------------------------------------------------

# The provider name published on the Terraform Registry.
NAME       = laravel-cloud
NAMESPACE  = figentra
BINARY     = terraform-provider-$(NAME)

# The version stamp GoReleaser injects at release time. Overrides on local
# builds via `make build VERSION=0.4.0-dev`.
VERSION   ?= dev

# LDFLAGS stamps the version into main.go's ldflag pin.
LDFLAGS    = -X main.version=$(VERSION)

# Go module path — matches go.mod. Every target references this so the
# provider name stays a single source of truth.
MODULE     = github.com/$(NAMESPACE)/terraform-provider-$(NAME)

# Test flags — surface race conditions + coverage on every unit run.
TEST_FLAGS ?= -race -count=1

# -----------------------------------------------------------------------------
# Primary targets
# -----------------------------------------------------------------------------

.PHONY: build
build:
	@echo "==> Building $(BINARY) ($(VERSION))"
	@go build -ldflags "$(LDFLAGS)" -o $(BINARY)

.PHONY: install
install:
	@echo "==> Installing $(BINARY) to \$GOBIN"
	@go install -ldflags "$(LDFLAGS)"

.PHONY: test
test:
	@echo "==> Running unit tests"
	@go test $(TEST_FLAGS) ./...

.PHONY: testacc
testacc:
	@echo "==> Running acceptance tests (TF_ACC=1)"
	@TF_ACC=1 go test $(TEST_FLAGS) -timeout 30m ./internal/provider/...

.PHONY: lint
lint:
	@echo "==> Running golangci-lint"
	@golangci-lint run ./...

.PHONY: fmt
fmt:
	@echo "==> Formatting Go source"
	@gofmt -w .
	@echo "==> Formatting Terraform examples"
	@terraform fmt -recursive ./examples

# -----------------------------------------------------------------------------
# Docs
# -----------------------------------------------------------------------------

.PHONY: docs
docs:
	@echo "==> Regenerating docs/ via tfplugindocs"
	@tfplugindocs generate --provider-name $(NAME)

.PHONY: docs-check
docs-check:
	@echo "==> Verifying docs/ is up-to-date (fails if drift detected)"
	@tfplugindocs generate --provider-name $(NAME)
	@git diff --exit-code docs/ && echo "docs/ is up-to-date" || \
		(echo "docs/ has drift — run 'make docs' + commit"; exit 1)

# -----------------------------------------------------------------------------
# Tooling
# -----------------------------------------------------------------------------

.PHONY: install-tools
install-tools:
	@echo "==> Installing dev tools"
	@go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/goreleaser/goreleaser/v2@latest
	@echo "==> Tools installed:"
	@echo "    tfplugindocs   — docs generator"
	@echo "    golangci-lint  — Go linter"
	@echo "    goreleaser     — release builder"

# -----------------------------------------------------------------------------
# Release
# -----------------------------------------------------------------------------

.PHONY: release-check
release-check:
	@echo "==> Dry-running GoReleaser build"
	@goreleaser check
	@goreleaser build --snapshot --clean --single-target

# -----------------------------------------------------------------------------
# Housekeeping
# -----------------------------------------------------------------------------

.PHONY: clean
clean:
	@echo "==> Removing build artefacts"
	@rm -f $(BINARY)
	@rm -rf dist/

.PHONY: tidy
tidy:
	@go mod tidy
