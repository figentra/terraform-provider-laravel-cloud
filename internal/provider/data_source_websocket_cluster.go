// Package provider — data-source implementation for
// `laravelcloud_websocket_cluster`. Reads an existing Cloud WebSocket
// cluster (Reverb-compatible) by ID + hydrates every attribute the
// resource-side surfaces (org binding, region, size, max connections,
// status, timestamps).
//
// Consumers use this data source when composing Terraform modules that
// need to REFERENCE a shared WS cluster provisioned out-of-band.
// Read-only — no CRUD.
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/figentra/terraform-provider-laravel-cloud/internal/api"
)

// WebsocketClusterDataSource implements the plugin-framework `DataSource`
// contract.
type WebsocketClusterDataSource struct {
	// client is the shared authenticated Cloud API wrapper.
	client *api.Client
}

// WebsocketClusterDataSourceModel mirrors the resource-side model. Only
// `id` is Required; every other field is Computed.
type WebsocketClusterDataSourceModel struct {
	// ID is the Cloud-assigned WS cluster ULID. REQUIRED input.
	ID types.String `tfsdk:"id"`

	// OrganizationID is the owning Cloud organisation.
	OrganizationID types.String `tfsdk:"organization_id"`

	// Name is the operator-facing cluster name.
	Name types.String `tfsdk:"name"`

	// Region is the deploy region.
	Region types.String `tfsdk:"region"`

	// Size is Cloud's WS cluster size slug.
	Size types.String `tfsdk:"size"`

	// MaxConnections is the concurrent-connection cap for the cluster.
	MaxConnections types.Int64 `tfsdk:"max_connections"`

	// Status is Cloud's lifecycle state.
	Status types.String `tfsdk:"status"`

	// CreatedAt is the RFC3339 timestamp of cluster creation.
	CreatedAt types.String `tfsdk:"created_at"`
}

// NewWebsocketClusterDataSource is the factory the provider registers.
func NewWebsocketClusterDataSource() datasource.DataSource {
	return &WebsocketClusterDataSource{}
}

// Metadata sets the data-source type name.
func (d *WebsocketClusterDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_websocket_cluster"
}

// Schema declares the read-only surface.
func (d *WebsocketClusterDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads an existing Laravel Cloud WebSocket cluster by ID. " +
			"Useful when composing modules that reference a shared Reverb-compatible " +
			"cluster provisioned in a separate state OR via the Cloud dashboard.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Cloud-assigned WebSocket cluster ID (ULID).",
				Required:            true,
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "ID of the owning Cloud organisation.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable cluster name.",
				Computed:            true,
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "Deploy region.",
				Computed:            true,
			},
			"size": schema.StringAttribute{
				MarkdownDescription: "Cluster size slug.",
				Computed:            true,
			},
			"max_connections": schema.Int64Attribute{
				MarkdownDescription: "Concurrent-connection cap for the cluster.",
				Computed:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Cloud lifecycle status.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp of cluster creation.",
				Computed:            true,
			},
		},
	}
}

// Configure pulls the shared client from ProviderData.
func (d *WebsocketClusterDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (d *WebsocketClusterDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data WebsocketClusterDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Fire the read against Cloud.
	cluster, err := d.client.GetWebsocketCluster(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to read websocket cluster",
			"Cloud API returned an error. Original error: "+err.Error(),
		)
		return
	}

	// Hydrate. organization_id, name, region, size, max_connections are
	// non-nullable per the API contract.
	data.OrganizationID = types.StringValue(cluster.OrganizationID)
	data.Name = types.StringValue(cluster.Name)
	data.Region = types.StringValue(cluster.Region)
	data.Size = types.StringValue(cluster.Size)
	data.MaxConnections = types.Int64Value(int64(cluster.MaxConnections))

	if cluster.Status != nil {
		data.Status = types.StringValue(*cluster.Status)
	} else {
		data.Status = types.StringNull()
	}
	if cluster.CreatedAt != nil {
		data.CreatedAt = types.StringValue(*cluster.CreatedAt)
	} else {
		data.CreatedAt = types.StringNull()
	}

	// Persist to state.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Compile-time interface assertions.
var (
	_ datasource.DataSource              = (*WebsocketClusterDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*WebsocketClusterDataSource)(nil)
)
