// Terraform Provider for Laravel Cloud
//
// Module path matches the eventual GitHub repo location. Publishing to
// the Terraform Registry as `stackra/laravel-cloud` requires the source
// repo be tagged with a semver release (v0.1.0, v0.2.0, ...) and signed
// via GPG per the `.goreleaser.yml` config.
//
// The `stackra` namespace on GitHub is authoritative for the workspace's
// OSS surface (framework packages + this provider).

module github.com/stackra/terraform-provider-laravel-cloud

go 1.22

require (
	// HashiCorp's official Terraform Plugin Framework — the modern SDK
	// mandated for every new provider per HashiCorp's 2023 policy.
	// terraform-plugin-sdk/v2 is legacy and forbidden for greenfield.
	github.com/hashicorp/terraform-plugin-framework v1.10.0
	// gRPC transport the framework uses to talk to Terraform. Pinned
	// through the framework itself but declared here for clarity.
	github.com/hashicorp/terraform-plugin-go v0.23.0
	// Log adapter — routes `tflog.Info(...)` calls to Terraform's
	// verbose-mode logging surface.
	github.com/hashicorp/terraform-plugin-log v0.9.0
)

// Acceptance-test framework — only pulled in for test builds via
// TF_ACC=1. Not part of the runtime provider binary.
require github.com/hashicorp/terraform-plugin-testing v1.10.0

require (
	github.com/ProtonMail/go-crypto v1.1.0-alpha.2 // indirect
	github.com/agext/levenshtein v1.2.2 // indirect
	github.com/apparentlymart/go-textseg/v15 v15.0.0 // indirect
	github.com/cloudflare/circl v1.3.7 // indirect
	github.com/fatih/color v1.16.0 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/hashicorp/go-checkpoint v0.5.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-cty v1.4.1-0.20200414143053-d3edf31b6320 // indirect
	github.com/hashicorp/go-hclog v1.6.3 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-plugin v1.6.0 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.7 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/hashicorp/hc-install v0.8.0 // indirect
	github.com/hashicorp/hcl/v2 v2.21.0 // indirect
	github.com/hashicorp/logutils v1.0.0 // indirect
	github.com/hashicorp/terraform-exec v0.21.0 // indirect
	github.com/hashicorp/terraform-json v0.22.1 // indirect
	github.com/hashicorp/terraform-plugin-sdk/v2 v2.34.0 // indirect
	github.com/hashicorp/terraform-registry-address v0.2.3 // indirect
	github.com/hashicorp/terraform-svchost v0.1.1 // indirect
	github.com/hashicorp/yamux v0.1.1 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-testing-interface v1.14.1 // indirect
	github.com/mitchellh/go-wordwrap v1.0.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/oklog/run v1.0.0 // indirect
	github.com/vmihailenco/msgpack v4.0.4+incompatible // indirect
	github.com/vmihailenco/msgpack/v5 v5.4.1 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	github.com/zclconf/go-cty v1.15.0 // indirect
	golang.org/x/crypto v0.26.0 // indirect
	golang.org/x/mod v0.19.0 // indirect
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/sync v0.8.0 // indirect
	golang.org/x/sys v0.23.0 // indirect
	golang.org/x/text v0.17.0 // indirect
	golang.org/x/tools v0.21.1-0.20240508182429-e35e4ccd0d2d // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240227224415-6ceb2ff114de // indirect
	google.golang.org/grpc v1.63.2 // indirect
	google.golang.org/protobuf v1.34.0 // indirect
)
