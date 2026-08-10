// Package provider — data-source implementation for
// `laravelcloud_database_schema`. Reads an existing Cloud database schema
// (logical database inside a cluster) by ID + hydrates every attribute
// the resource-side surfaces (cluster binding, name, status, timestamps).
//
// Consumers use this data source when composing Terraform modules that
// need to REFERENCE a schema provisioned out-of-band. Read-only — no CRUD.
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/figentra/terraform-provider-laravel-cloud/internal/api"
)

// DatabaseSchemaDataSource implements the plugin-framework `DataSource`
// contract. The `*api.Client` is populated once via `Configure`.
type DatabaseSchemaDataSource struct {
	// client is the shared authenticated Cloud API wrapper.
	client *api.Client
}

// DatabaseSchemaDataSourceModel mirrors the resource-side model. Only `id`
// is Required; every other field is Computed.
type DatabaseSchemaDataSourceModel struct {
	// ID is the Cloud-assigned schema ULID. REQUIRED input.
	ID types.String `tfsdk:"id"`

	// ClusterID is the parent DatabaseCluster this schema lives in.
	ClusterID types.String `tfsdk:"cluster_id"`

	// Name is the logical database name (Postgres schema, MySQL database).
	Name types.String `tfsdk:"name"`

	// Status is Cloud's lifecycle state — `active`, `provisioning`, etc.
	Status types.String `tfsdk:"status"`

	// CreatedAt is the RFC3339 timestamp of schema creation.
	CreatedAt types.String `tfsdk:"created_at"`
}

// NewDatabaseSchemaDataSource is the factory the provider registers.
func NewDatabaseSchemaDataSource() datasource.DataSource {
	return &DatabaseSchemaDataSource{}
}

// Metadata sets the data-source type name.
func (d *DatabaseSchemaDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_database_schema"
}

// Schema declares the read-only surface.
func (d *DatabaseSchemaDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads an existing Laravel Cloud database schema by ID. " +
			"Useful when composing modules that reference a schema provisioned " +
			"in a separate state OR via the Cloud dashboard.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Cloud-assigned database schema ID (ULID).",
				Required:            true,
			},
			"cluster_id": schema.StringAttribute{
				MarkdownDescription: "ID of the parent database cluster. Required — Cloud's schema endpoints are cluster-scoped.",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Logical database name inside the cluster.",
				Computed:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Cloud lifecycle status.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp of schema creation.",
				Computed:            true,
			},
		},
	}
}

// Configure pulls the shared client from ProviderData.
func (d *DatabaseSchemaDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (d *DatabaseSchemaDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DatabaseSchemaDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Fire the read against Cloud. Cluster ID is required — schema
	// endpoints are cluster-scoped (see api.GetDatabaseSchema).
	schemaObj, err := d.client.GetDatabaseSchema(ctx, data.ClusterID.ValueString(), data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to read database schema",
			"Cloud API returned an error. Original error: "+err.Error(),
		)
		return
	}

	// Hydrate. cluster_id and name are non-nullable per the API contract.
	if schemaObj.ClusterID != "" {
		data.ClusterID = types.StringValue(schemaObj.ClusterID)
	}
	data.Name = types.StringValue(schemaObj.Name)

	if schemaObj.Status != nil {
		data.Status = types.StringValue(*schemaObj.Status)
	} else {
		data.Status = types.StringNull()
	}
	if schemaObj.CreatedAt != nil {
		data.CreatedAt = types.StringValue(*schemaObj.CreatedAt)
	} else {
		data.CreatedAt = types.StringNull()
	}

	// Persist to state.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Compile-time interface assertions.
var (
	_ datasource.DataSource              = (*DatabaseSchemaDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*DatabaseSchemaDataSource)(nil)
)
