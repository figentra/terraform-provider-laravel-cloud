// Package provider — data-source implementation for `laravelcloud_cache`.
// Reads an existing Cloud cache (Valkey/Redis) by ID + hydrates the org
// binding, region, size, status, timestamps.
//
// Consumers use this data source when composing Terraform modules that
// need to REFERENCE a shared cache provisioned out-of-band. Read-only —
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

// CacheDataSource implements the plugin-framework `DataSource` contract.
type CacheDataSource struct {
	// client is the shared authenticated Cloud API wrapper.
	client *api.Client
}

// CacheDataSourceModel mirrors the resource-side model. Only `id` is
// Required; every other field is Computed.
type CacheDataSourceModel struct {
	// ID is the Cloud-assigned cache ULID. REQUIRED input.
	ID types.String `tfsdk:"id"`

	// OrganizationID is the owning Cloud organisation.
	OrganizationID types.String `tfsdk:"organization_id"`

	// Name is the operator-facing cache name.
	Name types.String `tfsdk:"name"`

	// Region is the deploy region — matches the app's region.
	Region types.String `tfsdk:"region"`

	// Size is Cloud's cache size slug — `valkey-pro.1gb`, `valkey-pro.5gb`, etc.
	Size types.String `tfsdk:"size"`

	// Status is Cloud's lifecycle state.
	Status types.String `tfsdk:"status"`

	// CreatedAt is the RFC3339 timestamp of cache creation.
	CreatedAt types.String `tfsdk:"created_at"`
}

// NewCacheDataSource is the factory the provider registers.
func NewCacheDataSource() datasource.DataSource {
	return &CacheDataSource{}
}

// Metadata sets the data-source type name.
func (d *CacheDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cache"
}

// Schema declares the read-only surface.
func (d *CacheDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads an existing Laravel Cloud cache instance by ID. " +
			"Useful when composing modules that reference a shared Valkey/Redis " +
			"cache provisioned in a separate state OR via the Cloud dashboard.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Cloud-assigned cache ID (ULID).",
				Required:            true,
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "ID of the owning Cloud organisation.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable cache name.",
				Computed:            true,
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "Deploy region.",
				Computed:            true,
			},
			"size": schema.StringAttribute{
				MarkdownDescription: "Cache size slug — e.g. `valkey-pro.1gb`.",
				Computed:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Cloud lifecycle status.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp of cache creation.",
				Computed:            true,
			},
		},
	}
}

// Configure pulls the shared client from ProviderData.
func (d *CacheDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (d *CacheDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data CacheDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Fire the read against Cloud.
	cache, err := d.client.GetCache(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to read cache",
			"Cloud API returned an error. Original error: "+err.Error(),
		)
		return
	}

	// Hydrate the model. organization_id, name, region, size are all
	// non-nullable per the API contract.
	data.OrganizationID = types.StringValue(cache.OrganizationID)
	data.Name = types.StringValue(cache.Name)
	data.Region = types.StringValue(cache.Region)
	data.Size = types.StringValue(cache.Size)

	if cache.Status != nil {
		data.Status = types.StringValue(*cache.Status)
	} else {
		data.Status = types.StringNull()
	}
	if cache.CreatedAt != nil {
		data.CreatedAt = types.StringValue(*cache.CreatedAt)
	} else {
		data.CreatedAt = types.StringNull()
	}

	// Persist to state.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Compile-time interface assertions.
var (
	_ datasource.DataSource              = (*CacheDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*CacheDataSource)(nil)
)
