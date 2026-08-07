// Package provider — data-source implementation for
// `laravelcloud_database_cluster`. Reads an existing Cloud database
// cluster by ID + hydrates every attribute the resource-side surfaces
// (org binding, engine, size, HA, backup retention, status, timestamps).
//
// Consumers use this data source when composing Terraform modules that
// need to REFERENCE a shared database cluster provisioned out-of-band
// (Cloud dashboard, a separate Terraform state managed by a platform
// team). Read-only — no CRUD.
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/figentra/terraform-provider-laravel-cloud/internal/api"
)

// DatabaseClusterDataSource implements the Terraform Plugin Framework
// `datasource.DataSource` contract. The `*api.Client` field is populated
// once via `Configure` from ProviderData stashed by the provider.
type DatabaseClusterDataSource struct {
	// client is the shared authenticated Cloud API wrapper.
	client *api.Client
}

// DatabaseClusterDataSourceModel mirrors the resource-side model. Only
// `id` is Required so operators lookup by primary key; every other field
// is Computed.
type DatabaseClusterDataSourceModel struct {
	// ID is the Cloud-assigned cluster ULID. REQUIRED input.
	ID types.String `tfsdk:"id"`

	// OrganizationID is the owning Cloud organisation.
	OrganizationID types.String `tfsdk:"organization_id"`

	// Name is the operator-facing cluster name — e.g. `shared-dev`.
	Name types.String `tfsdk:"name"`

	// Region is the deploy region — `us-east-1`, `eu-west-1`, etc.
	Region types.String `tfsdk:"region"`

	// Engine is the DB flavour + major version — `postgres-16`, `mysql-8`.
	Engine types.String `tfsdk:"engine"`

	// Size is Cloud's size slug — `db.small`, `db.medium`, etc.
	Size types.String `tfsdk:"size"`

	// HighAvailability enables the Cloud HA replica.
	HighAvailability types.Bool `tfsdk:"high_availability"`

	// BackupRetentionDays sets how long Cloud retains automatic backups.
	BackupRetentionDays types.Int64 `tfsdk:"backup_retention_days"`

	// Status is Cloud's lifecycle state — `active`, `provisioning`, etc.
	Status types.String `tfsdk:"status"`

	// CreatedAt / UpdatedAt are RFC3339 timestamps.
	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`
}

// NewDatabaseClusterDataSource is the factory the provider registers.
func NewDatabaseClusterDataSource() datasource.DataSource {
	return &DatabaseClusterDataSource{}
}

// Metadata sets the data-source type name. Callers reference this via
// `data.laravelcloud_database_cluster.<name>`.
func (d *DatabaseClusterDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_database_cluster"
}

// Schema declares the read-only surface — matches the resource shape
// with every field Computed except `id` which is Required.
func (d *DatabaseClusterDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads an existing Laravel Cloud database cluster by ID. " +
			"Useful when a Terraform module needs to reference a shared cluster " +
			"provisioned in a separate state OR via the Cloud dashboard.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Cloud-assigned database cluster ID (ULID).",
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
				MarkdownDescription: "Deploy region — matches the app's region.",
				Computed:            true,
			},
			"engine": schema.StringAttribute{
				MarkdownDescription: "Database engine — `postgres-16`, `mysql-8`, etc.",
				Computed:            true,
			},
			"size": schema.StringAttribute{
				MarkdownDescription: "Cloud size slug — `db.small`, `db.medium`, ...",
				Computed:            true,
			},
			"high_availability": schema.BoolAttribute{
				MarkdownDescription: "True when the HA replica is enabled.",
				Computed:            true,
			},
			"backup_retention_days": schema.Int64Attribute{
				MarkdownDescription: "Automatic backup retention window in days.",
				Computed:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Cloud lifecycle status — `active`, `provisioning`, etc.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp of cluster creation.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp of last mutation.",
				Computed:            true,
			},
		},
	}
}

// Configure pulls the shared client from ProviderData. Called once per
// data-source instance before Read fires.
func (d *DatabaseClusterDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// ProviderData is nil during framework-side validation phases.
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

// Read hydrates state from Cloud. Called during every `terraform plan`.
func (d *DatabaseClusterDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Pull the ID the operator supplied into a typed model.
	var data DatabaseClusterDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read the cluster from Cloud. Data sources don't tolerate 404 —
	// missing = operator gave a wrong ID → surface as error.
	cluster, err := d.client.GetDatabaseCluster(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to read database cluster",
			"Cloud API returned an error. Original error: "+err.Error(),
		)
		return
	}

	// Hydrate every field. Non-nullable fields → direct StringValue /
	// BoolValue / Int64Value. Nullable → typed null when unset.
	data.OrganizationID = types.StringValue(cluster.OrganizationID)
	data.Name = types.StringValue(cluster.Name)
	data.Region = types.StringValue(cluster.Region)
	data.Engine = types.StringValue(cluster.Engine)
	data.Size = types.StringValue(cluster.Size)
	data.HighAvailability = types.BoolValue(cluster.HighAvailability)
	data.BackupRetentionDays = types.Int64Value(int64(cluster.BackupRetentionDays))

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
	if cluster.UpdatedAt != nil {
		data.UpdatedAt = types.StringValue(*cluster.UpdatedAt)
	} else {
		data.UpdatedAt = types.StringNull()
	}

	// Persist to state so downstream references resolve.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Compile-time interface assertions — fail the build at authoring time
// if the data source ever loses a required method.
var (
	_ datasource.DataSource              = (*DatabaseClusterDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*DatabaseClusterDataSource)(nil)
)
