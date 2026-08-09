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
// Required; every other field is Computed. As of v0.4.0 the data source
// surfaces the full expanded attribute set alongside the legacy `mode`.
type BucketDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	Name           types.String `tfsdk:"name"`
	Region         types.String `tfsdk:"region"`

	// Mode is the pre-v0.4.0 access flag.
	Mode types.String `tfsdk:"mode"`

	// Visibility is the v0.4.0 canonical access flag.
	Visibility types.String `tfsdk:"visibility"`

	// Jurisdiction is the geographic zone slug.
	Jurisdiction types.String `tfsdk:"jurisdiction"`

	// KeyName is the identifier of the auto-generated access key.
	KeyName types.String `tfsdk:"key_name"`

	// KeyPermission is the permission level of the generated key.
	KeyPermission types.String `tfsdk:"key_permission"`

	Status    types.String `tfsdk:"status"`
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
				MarkdownDescription: "**Deprecated in v0.4.0** — use `visibility`. `private` or `public`.",
				Computed:            true,
				DeprecationMessage:  "Use `visibility` instead.",
			},
			"visibility": schema.StringAttribute{
				MarkdownDescription: "Access model — `private` or `public`. Added in v0.4.0.",
				Computed:            true,
			},
			"jurisdiction": schema.StringAttribute{
				MarkdownDescription: "Geographic zone slug. Added in v0.4.0.",
				Computed:            true,
			},
			"key_name": schema.StringAttribute{
				MarkdownDescription: "Identifier of the auto-generated access key. Added in v0.4.0.",
				Computed:            true,
			},
			"key_permission": schema.StringAttribute{
				MarkdownDescription: "Permission level of the auto-generated key. Added in v0.4.0.",
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

	data.OrganizationID = types.StringValue(bucket.OrganizationID)
	data.Name = types.StringValue(bucket.Name)
	data.Region = types.StringValue(bucket.Region)

	// mode↔visibility aliasing — same rule as the resource layer.
	visibility := bucket.Visibility
	if visibility == "" {
		visibility = bucket.Mode
	}
	mode := bucket.Mode
	if mode == "" {
		mode = bucket.Visibility
	}
	if visibility != "" {
		data.Visibility = types.StringValue(visibility)
	} else {
		data.Visibility = types.StringNull()
	}
	if mode != "" {
		data.Mode = types.StringValue(mode)
	} else {
		data.Mode = types.StringNull()
	}

	if bucket.Jurisdiction != nil {
		data.Jurisdiction = types.StringValue(*bucket.Jurisdiction)
	} else {
		data.Jurisdiction = types.StringNull()
	}
	if bucket.KeyName != nil {
		data.KeyName = types.StringValue(*bucket.KeyName)
	} else {
		data.KeyName = types.StringNull()
	}
	if bucket.KeyPermission != nil {
		data.KeyPermission = types.StringValue(*bucket.KeyPermission)
	} else {
		data.KeyPermission = types.StringNull()
	}
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
