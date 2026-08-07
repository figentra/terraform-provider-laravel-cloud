// Package provider — data-source implementation for `laravelcloud_domain`.
// Reads an existing Cloud custom-hostname binding by ID + hydrates the
// environment binding, name, redirect + wildcard + Cloudflare flags,
// verification mode, status, timestamps.
//
// Consumers use this data source when composing Terraform modules that
// need to REFERENCE a domain binding provisioned out-of-band. Read-only —
// no CRUD.
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/figentra/terraform-provider-laravel-cloud/internal/api"
)

// DomainDataSource implements the plugin-framework `DataSource` contract.
type DomainDataSource struct {
	// client is the shared authenticated Cloud API wrapper.
	client *api.Client
}

// DomainDataSourceModel mirrors the resource-side model. Only `id` is
// Required; every other field is Computed.
type DomainDataSourceModel struct {
	// ID is the Cloud-assigned domain binding ULID. REQUIRED input.
	ID types.String `tfsdk:"id"`

	// EnvironmentID is the Cloud environment this hostname routes to.
	EnvironmentID types.String `tfsdk:"environment_id"`

	// Name is the fully-qualified hostname — e.g. `app.example.com`.
	Name types.String `tfsdk:"name"`

	// RedirectFromWWW enables the Cloud edge to 301-redirect
	// `www.<name>` to `<name>`.
	RedirectFromWWW types.Bool `tfsdk:"redirect_from_www"`

	// WildcardEnabled routes every `*.<name>` subdomain to this env.
	WildcardEnabled types.Bool `tfsdk:"wildcard_enabled"`

	// CloudflareManaged toggles Cloudflare orange-cloud proxying (Cloud
	// coordinates the CF record when true).
	CloudflareManaged types.Bool `tfsdk:"cloudflare_managed"`

	// Verification is the DNS verification mode — `real_time` (poll DNS
	// automatically) or `manual` (operator flips a switch).
	Verification types.String `tfsdk:"verification"`

	// Status is Cloud's lifecycle state.
	Status types.String `tfsdk:"status"`

	// CreatedAt is the RFC3339 timestamp of domain binding creation.
	CreatedAt types.String `tfsdk:"created_at"`
}

// NewDomainDataSource is the factory the provider registers.
func NewDomainDataSource() datasource.DataSource {
	return &DomainDataSource{}
}

// Metadata sets the data-source type name.
func (d *DomainDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

// Schema declares the read-only surface.
func (d *DomainDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads an existing Laravel Cloud domain binding by ID. " +
			"Useful when composing modules that reference a custom hostname " +
			"binding provisioned in a separate state OR via the Cloud dashboard.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Cloud-assigned domain binding ID (ULID).",
				Required:            true,
			},
			"environment_id": schema.StringAttribute{
				MarkdownDescription: "ID of the environment this hostname routes to.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Fully-qualified hostname bound to the environment.",
				Computed:            true,
			},
			"redirect_from_www": schema.BoolAttribute{
				MarkdownDescription: "True when Cloud 301-redirects `www.<name>` to `<name>`.",
				Computed:            true,
			},
			"wildcard_enabled": schema.BoolAttribute{
				MarkdownDescription: "True when `*.<name>` routes to this environment.",
				Computed:            true,
			},
			"cloudflare_managed": schema.BoolAttribute{
				MarkdownDescription: "True when Cloud coordinates a Cloudflare " +
					"proxied record for this hostname.",
				Computed: true,
			},
			"verification": schema.StringAttribute{
				MarkdownDescription: "DNS verification mode — `real_time` or `manual`.",
				Computed:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Cloud lifecycle status.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp of binding creation.",
				Computed:            true,
			},
		},
	}
}

// Configure pulls the shared client from ProviderData.
func (d *DomainDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*api.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected DataSource Configure Type",
			fmt.Sprintf("Expected *api.Client, got: %T. Please report this issue.",
				req.ProviderData),
		)
		return
	}

	d.client = client
}

// Read hydrates state from Cloud.
func (d *DomainDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DomainDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Fire the read against Cloud.
	domain, err := d.client.GetDomain(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to read domain binding",
			"Cloud API returned an error. Original error: "+err.Error(),
		)
		return
	}

	// Hydrate. Every field except status + created_at is non-nullable per
	// the API contract.
	data.EnvironmentID = types.StringValue(domain.EnvironmentID)
	data.Name = types.StringValue(domain.Name)
	data.RedirectFromWWW = types.BoolValue(domain.RedirectFromWWW)
	data.WildcardEnabled = types.BoolValue(domain.WildcardEnabled)
	data.CloudflareManaged = types.BoolValue(domain.CloudflareManaged)
	data.Verification = types.StringValue(domain.Verification)

	if domain.Status != nil {
		data.Status = types.StringValue(*domain.Status)
	} else {
		data.Status = types.StringNull()
	}
	if domain.CreatedAt != nil {
		data.CreatedAt = types.StringValue(*domain.CreatedAt)
	} else {
		data.CreatedAt = types.StringNull()
	}

	// Persist to state.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Compile-time interface assertions.
var (
	_ datasource.DataSource              = (*DomainDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*DomainDataSource)(nil)
)
