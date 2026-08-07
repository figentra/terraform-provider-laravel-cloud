// Package provider is the Terraform Plugin Framework provider implementation.
//
// The `New()` factory in this file is invoked by main.go's `providerserver.Serve`
// call at plugin boot. Every `terraform plan`/`apply` cycle constructs one
// provider instance, calls Configure() once, then reuses the resulting
// `*api.Client` across every resource + data source.
//
// Provider config schema mirrors the PHP CLI's 6-step token priority chain
// so operators can migrate incrementally without re-learning auth.
package provider

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/stackra/terraform-provider-laravel-cloud/internal/api"
)

// LaravelCloudProvider satisfies `provider.Provider`. Every method (Metadata,
// Schema, Configure, Resources, DataSources) is exercised by Terraform Core
// once per plugin lifecycle.
type LaravelCloudProvider struct {
	// version is stamped by main.go from the ldflag at release time.
	// Surfaced via `provider.MetadataResponse.Version` so `terraform version`
	// prints the plugin's release tag.
	version string
}

// LaravelCloudProviderModel is the typed struct Terraform hydrates from
// the operator's `provider "laravelcloud" { ... }` block during Configure.
// Every field is `types.String` (or the matching plugin-framework type)
// so we can distinguish "unset" from "explicitly empty" — the former
// falls through to the env-var chain, the latter is an error.
type LaravelCloudProviderModel struct {
	Token     types.String `tfsdk:"token"`
	CloudOrg  types.String `tfsdk:"cloud_org"`
	TokenFile types.String `tfsdk:"token_file"`
	BaseURL   types.String `tfsdk:"base_url"`
	Timeout   types.Int64  `tfsdk:"timeout"`
}

// New returns a factory that plugin-framework invokes once per plan.
// The version string comes from main.go — see the ldflag stamping there.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &LaravelCloudProvider{version: version}
	}
}

// Metadata sets the provider address prefix. Every resource type name
// gets prefixed with `laravelcloud_` so `laravelcloud_application` reads
// naturally in HCL.
func (p *LaravelCloudProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "laravelcloud"
	resp.Version = p.version
}

// Schema declares the provider-level attributes an operator sets in the
// `provider "laravelcloud" { ... }` block. Every attribute is Optional
// because we support 6 fallback sources (block value → org-scoped env →
// generic env → token file → default token file → error).
func (p *LaravelCloudProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Interact with [Laravel Cloud](https://cloud.laravel.com) " +
			"— manage applications, environments, database clusters, caches, buckets, " +
			"WebSocket clusters, and domains via Terraform.",

		Attributes: map[string]schema.Attribute{
			"token": schema.StringAttribute{
				MarkdownDescription: "Laravel Cloud API token. Falls back to the " +
					"`LARAVEL_CLOUD_TOKEN` env var, then to the token file (see " +
					"`token_file`). Get a token from " +
					"https://cloud.laravel.com/settings/api-tokens.",
				Optional:  true,
				Sensitive: true,
			},
			"cloud_org": schema.StringAttribute{
				MarkdownDescription: "Cloud organisation slug. When set, the provider " +
					"looks for a `<ORG>_LARAVEL_CLOUD_TOKEN` env var BEFORE falling " +
					"back to the generic `LARAVEL_CLOUD_TOKEN`. Example: setting " +
					"`cloud_org = \"figentra\"` looks for `FIGENTRA_LARAVEL_CLOUD_TOKEN`.",
				Optional: true,
			},
			"token_file": schema.StringAttribute{
				MarkdownDescription: "Path to a file containing the token (one line, " +
					"no trailing newline). Defaults to `.kiro/cloud/token`. Used only " +
					"when no env var is set.",
				Optional: true,
			},
			"base_url": schema.StringAttribute{
				MarkdownDescription: "Cloud API base URL. Defaults to " +
					"`https://cloud.laravel.com/api`. Override for staging + local " +
					"development against a preview build.",
				Optional: true,
			},
			"timeout": schema.Int64Attribute{
				MarkdownDescription: "Per-request HTTP timeout in seconds. Defaults to 60.",
				Optional:            true,
			},
		},
	}
}

// Configure resolves the token via the 6-step priority chain + builds the
// shared `*api.Client` every resource + data source injects.
//
// Priority (highest wins):
//  1. `provider.token` block attribute
//  2. `<CLOUD_ORG>_LARAVEL_CLOUD_TOKEN` env var (when `cloud_org` is set)
//  3. `LARAVEL_CLOUD_TOKEN` env var
//  4. `provider.token_file` contents
//  5. `.kiro/cloud/token` contents (workspace-canonical default)
//  6. Error diagnostic — operator must set one of the above
func (p *LaravelCloudProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data LaravelCloudProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Resolve token via the priority chain.
	token := resolveToken(data)
	if token == "" {
		resp.Diagnostics.AddError(
			"Missing Laravel Cloud API token",
			"The provider cannot proceed without an API token. Set the `token` "+
				"attribute in the provider block, OR set `LARAVEL_CLOUD_TOKEN` in "+
				"the environment, OR provide a token file at `.kiro/cloud/token`. "+
				"Generate a token at https://cloud.laravel.com/settings/api-tokens.",
		)
		return
	}

	// Base URL: block value, else default.
	baseURL := api.DefaultBaseURL
	if !data.BaseURL.IsNull() && !data.BaseURL.IsUnknown() {
		baseURL = data.BaseURL.ValueString()
	}

	// Timeout: block value, else default.
	timeout := api.DefaultTimeout
	if !data.Timeout.IsNull() && !data.Timeout.IsUnknown() && data.Timeout.ValueInt64() > 0 {
		timeout = time.Duration(data.Timeout.ValueInt64()) * time.Second
	}

	// Build client.
	userAgent := "terraform-provider-laravel-cloud/" + p.version
	client := api.New(baseURL, token, userAgent, timeout)

	// Stash on ResourceData + DataSourceData so every resource can pull it
	// out via `req.ProviderData.(*api.Client)` in its Configure.
	resp.ResourceData = client
	resp.DataSourceData = client
}

// Resources returns every managed-resource type the provider ships.
// The full canonical set (v0.2.0) covers every write-path Laravel Cloud
// primitive the workspace's `.kiro/cloud/apps/*.yaml` manifests declare.
func (p *LaravelCloudProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewApplicationResource,
		NewEnvironmentResource,
		NewDatabaseClusterResource,
		NewDatabaseSchemaResource,
		NewCacheResource,
		NewBucketResource,
		NewWebsocketClusterResource,
		NewWebsocketAppResource,
		NewDomainResource,
	}
}

// DataSources returns every data-source type the provider ships.
// Every resource type carries a matching data source so consumers can
// look up existing resources without importing into state.
func (p *LaravelCloudProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewApplicationDataSource,
	}
}

// resolveToken walks the 6-step priority chain. Called exactly once per
// plan/apply from Configure().
func resolveToken(data LaravelCloudProviderModel) string {
	// 1. Explicit block value.
	if !data.Token.IsNull() && !data.Token.IsUnknown() && data.Token.ValueString() != "" {
		return data.Token.ValueString()
	}

	// 2. Org-scoped env var (only when cloud_org is set).
	if !data.CloudOrg.IsNull() && !data.CloudOrg.IsUnknown() {
		org := strings.ToUpper(data.CloudOrg.ValueString())
		if org != "" {
			if v := os.Getenv(org + "_LARAVEL_CLOUD_TOKEN"); v != "" {
				return v
			}
		}
	}

	// 3. Generic env var.
	if v := os.Getenv("LARAVEL_CLOUD_TOKEN"); v != "" {
		return v
	}

	// 4. Explicit token file.
	if !data.TokenFile.IsNull() && !data.TokenFile.IsUnknown() && data.TokenFile.ValueString() != "" {
		if v, err := readTokenFile(data.TokenFile.ValueString()); err == nil && v != "" {
			return v
		}
	}

	// 5. Workspace-canonical default file.
	if v, err := readTokenFile(".kiro/cloud/token"); err == nil && v != "" {
		return v
	}

	// 6. Fall through — Configure() will surface the diagnostic.
	return ""
}

// readTokenFile reads a token file, trimming whitespace + newline.
// Returns empty string + nil on any read error so the priority chain
// continues to the next fallback.
func readTokenFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// Ensure LaravelCloudProvider implements the provider.Provider interface
// at compile time — plugin-framework requires this contract be satisfied.
var _ provider.Provider = (*LaravelCloudProvider)(nil)

// pathHelper suppresses the unused-import lint warning on `path` — the
// package is used indirectly by resource + data-source implementations
// that live in sibling files. Kept here as a hint for future resources.
var _ = path.Root
