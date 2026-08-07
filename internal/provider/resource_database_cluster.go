package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/figentra/terraform-provider-laravel-cloud/internal/api"
)

// DatabaseClusterResource manages `laravelcloud_database_cluster` —
// a shared Postgres or MySQL cluster hosting one or more schemas.
type DatabaseClusterResource struct{ client *api.Client }

// DatabaseClusterResourceModel maps HCL <-> API DTO.
type DatabaseClusterResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	OrganizationID      types.String `tfsdk:"organization_id"`
	Name                types.String `tfsdk:"name"`
	Region              types.String `tfsdk:"region"`
	Engine              types.String `tfsdk:"engine"`
	Size                types.String `tfsdk:"size"`
	HighAvailability    types.Bool   `tfsdk:"high_availability"`
	BackupRetentionDays types.Int64  `tfsdk:"backup_retention_days"`
	CreatedAt           types.String `tfsdk:"created_at"`
}

func NewDatabaseClusterResource() resource.Resource { return &DatabaseClusterResource{} }

func (r *DatabaseClusterResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_database_cluster"
}

func (r *DatabaseClusterResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Shared database cluster. One cluster per region per org, hosting multiple `laravelcloud_database_schema` resources.",
		Attributes: map[string]schema.Attribute{
			"id":              schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"organization_id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"name":            schema.StringAttribute{Required: true, MarkdownDescription: "Cluster name (displayed in dashboard)."},
			"region":          schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, MarkdownDescription: "Immutable region."},
			"engine": schema.StringAttribute{
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				MarkdownDescription: "One of `postgres-15`, `postgres-16`, `mysql-8`. Immutable — changing forces replace.",
			},
			"size":                  schema.StringAttribute{Required: true, MarkdownDescription: "Cluster size — e.g. `db.s-1vcpu-1gb`, `db.m-4vcpu-8gb`."},
			"high_availability":     schema.BoolAttribute{Required: true, MarkdownDescription: "When true, cluster runs with a hot standby."},
			"backup_retention_days": schema.Int64Attribute{Optional: true, Computed: true, MarkdownDescription: "Backup retention in days. Cloud picks a default when unset."},
			"created_at":            schema.StringAttribute{Computed: true},
		},
	}
}

func (r *DatabaseClusterResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*api.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Configure Type", fmt.Sprintf("expected *api.Client, got %T", req.ProviderData))
		return
	}
	r.client = client
}

func (r *DatabaseClusterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DatabaseClusterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := api.CreateDatabaseClusterRequest{
		OrganizationID:   plan.OrganizationID.ValueString(),
		Name:             plan.Name.ValueString(),
		Region:           plan.Region.ValueString(),
		Engine:           plan.Engine.ValueString(),
		Size:             plan.Size.ValueString(),
		HighAvailability: plan.HighAvailability.ValueBool(),
	}
	if !plan.BackupRetentionDays.IsNull() && !plan.BackupRetentionDays.IsUnknown() {
		apiReq.BackupRetentionDays = int(plan.BackupRetentionDays.ValueInt64())
	}

	cluster, err := r.client.CreateDatabaseCluster(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create database cluster", err.Error())
		return
	}
	applyDBClusterToModel(cluster, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DatabaseClusterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DatabaseClusterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cluster, err := r.client.GetDatabaseCluster(ctx, state.ID.ValueString())
	if err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read database cluster", err.Error())
		return
	}
	applyDBClusterToModel(cluster, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *DatabaseClusterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan DatabaseClusterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := api.UpdateDatabaseClusterRequest{}
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		v := plan.Name.ValueString()
		apiReq.Name = &v
	}
	if !plan.Size.IsNull() && !plan.Size.IsUnknown() {
		v := plan.Size.ValueString()
		apiReq.Size = &v
	}
	if !plan.HighAvailability.IsNull() && !plan.HighAvailability.IsUnknown() {
		v := plan.HighAvailability.ValueBool()
		apiReq.HighAvailability = &v
	}
	if !plan.BackupRetentionDays.IsNull() && !plan.BackupRetentionDays.IsUnknown() {
		v := int(plan.BackupRetentionDays.ValueInt64())
		apiReq.BackupRetentionDays = &v
	}

	cluster, err := r.client.UpdateDatabaseCluster(ctx, plan.ID.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update database cluster", err.Error())
		return
	}
	applyDBClusterToModel(cluster, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DatabaseClusterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DatabaseClusterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteDatabaseCluster(ctx, state.ID.ValueString()); err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError("Failed to delete database cluster", err.Error())
	}
}

func (r *DatabaseClusterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func applyDBClusterToModel(cluster *api.DatabaseCluster, m *DatabaseClusterResourceModel) {
	m.ID = types.StringValue(cluster.ID)
	m.OrganizationID = types.StringValue(cluster.OrganizationID)
	m.Name = types.StringValue(cluster.Name)
	m.Region = types.StringValue(cluster.Region)
	m.Engine = types.StringValue(cluster.Engine)
	m.Size = types.StringValue(cluster.Size)
	m.HighAvailability = types.BoolValue(cluster.HighAvailability)
	m.BackupRetentionDays = types.Int64Value(int64(cluster.BackupRetentionDays))
	if cluster.CreatedAt != nil {
		m.CreatedAt = types.StringValue(*cluster.CreatedAt)
	} else {
		m.CreatedAt = types.StringNull()
	}
}

var (
	_ resource.Resource                = (*DatabaseClusterResource)(nil)
	_ resource.ResourceWithImportState = (*DatabaseClusterResource)(nil)
	_ resource.ResourceWithConfigure   = (*DatabaseClusterResource)(nil)
)
