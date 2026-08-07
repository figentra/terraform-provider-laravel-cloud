// Package provider — data-source implementation for `laravelcloud_bucket`.
// Reads an existing Cloud object-storage bucket by ID + hydrates the org
// binding, region, mode (private / public), status, timestamps.
//
// Consumers use this data source when composing Terraform modules that
// need to REFERENCE a shared bucket provisioned out-of-band. Read-only —
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

// BucketDataSource implements the plugin-framework `DataSource` contract.
type BucketDataSource struct {
	// client is the shared authenticated Cloud API wrapper.
	client *api.Client
}

// BucketDataSourceModel mirrors the resource-side model. Only `id` is
// Required; every other field is Computed.
type BucketDataSourceModel struct {
	// ID is the Cloud-assigned bucket ULID. REQUIRED input.
	ID types.String `tfsdk:"id"`

	// OrganizationID is the owning Cloud organisation.
	OrganizationID types.String `tfsdk:"organization_id"`

	// Name is the bucket name (globally-unique per Cloud).
	Name types.String `tfsdk:"name"`

	// Region is the deploy region.
	Region types.String `tfsdk:"region"`

	// Mode determines the access model — `private` (Cloud-signed URLs
	// required) or `public` (unauthenticated GET).
	Mode types.String `tfsdk:"mode"`

	// Status is Cloud's lifecycle state.
	Status types.String `tfsdk:"status"`

	// CreatedAt is the RFC3339 timestamp of bucket creation.
	CreatedAt types.String `tfsdk:"created_at"`
}

// NewBucketDataSource is the factory the provider registers.
func NewBucketDataSource() datasource.DataSource {
	return &BucketDataSource{}
}

// Metadata sets the data-source type name.
func (d *BucketDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bucket"
}

// Schema declares the read-only surface.
func (d *BucketDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads an existing Laravel Cloud bucket by ID. " +
			"Useful when composing modules that reference a shared object-store " +
			"bucket provisioned in a separate state OR via the Cloud dashboard.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Cloud-assigned bucket ID (ULID).",
				Required:            true,
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "ID of the owning Cloud organisation.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Bucket name (globally unique within Cloud).",
				Computed:            true,
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "Deploy region.",
				Computed:            true,
			},
			"mode": schema.StringAttribute{
				MarkdownDescription: "Access mode — `private` or `public`.",
				Computed:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Cloud lifecycle status.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp of bucket creation.",
				Computed:            true,
			},
		},
	}
}

// Configure pulls the shared client from ProviderData.
func (d *BucketDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (d *BucketDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data BucketDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Fire the read against Cloud.
	bucket, err := d.client.GetBucket(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to read bucket",
			"Cloud API returned an error. Original error: "+err.Error(),
		)
		return
	}

	// Hydrate the model. organization_id, name, region, mode are all
	// non-nullable per the API contract.
	data.OrganizationID = types.StringValue(bucket.OrganizationID)
	data.Name = types.StringValue(bucket.Name)
	data.Region = types.StringValue(bucket.Region)
	data.Mode = types.StringValue(bucket.Mode)

	if bucket.Status != nil {
		data.Status = types.StringValue(*bucket.Status)
	} else {
		data.Status = types.StringNull()
	}
	if bucket.CreatedAt != nil {
		data.CreatedAt = types.StringValue(*bucket.CreatedAt)
	} else {
		data.CreatedAt = types.StringNull()
	}

	// Persist to state.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Compile-time interface assertions.
var (
	_ datasource.DataSource              = (*BucketDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*BucketDataSource)(nil)
)
