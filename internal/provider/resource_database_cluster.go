package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/figentra/terraform-provider-laravel-cloud/internal/api"
)

// DatabaseClusterResource manages `laravelcloud_database_cluster` —
// a shared Postgres, MySQL, or Neon serverless cluster hosting one or
// more schemas.
//
// v0.4.0 attribute expansion:
//   - `type` — Cloud v2 cluster-type slug (`neon_serverless_postgres_18`,
//     `postgres_16`, `mysql_8`).
//   - `config` — v2 tuning bag (cu_min, cu_max, suspend_seconds,
//     retention_days). Stored as a JSON string on the Terraform side so
//     schema stays flat + supports arbitrary nested values.
//   - `organization_id` relaxed to Optional/Computed — Cloud infers from
//     the token when unset.
//   - Pre-v0.4 attributes (`engine`, `size`, `high_availability`,
//     `backup_retention_days`) remain Optional/Computed for backward
//     compatibility.
type DatabaseClusterResource struct{ client *api.Client }

// DatabaseClusterResourceModel maps HCL <-> API DTO. `config` is a
// dynamic map — kept as a JSON-encoded string internally to support
// arbitrary nested / mixed-type values without a per-key schema.
type DatabaseClusterResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	OrganizationID      types.String `tfsdk:"organization_id"`
	Name                types.String `tfsdk:"name"`
	Region              types.String `tfsdk:"region"`
	Type                types.String `tfsdk:"type"`
	Config              types.Map    `tfsdk:"config"`
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
		MarkdownDescription: "Shared database cluster. v0.4.0 supports Cloud's v2 cluster-type slugs (`neon_serverless_postgres_18`, `postgres_16`, `mysql_8`) via the `type` + `config` attribute pair. Pre-v0.4 attributes (`engine`, `size`, `high_availability`) remain functional for backward compatibility.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "Owning Cloud organisation. Optional in v0.4.0 — Cloud infers from token when unset.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Cluster name.",
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "Deploy region. Optional in v0.4.0 — Cloud derives from organisation when unset.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Cluster type slug — `neon_serverless_postgres_18`, `postgres_16`, `mysql_8`. Added in v0.4.0. Immutable — forces replace.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"config": schema.MapAttribute{
				MarkdownDescription: "Type-specific tuning bag — `cu_min`, `cu_max`, `suspend_seconds`, `retention_days`. Stringified values (Terraform maps require homogeneous element types; consumers can pass numbers as strings). Added in v0.4.0.",
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
			},
			"engine": schema.StringAttribute{
				MarkdownDescription: "**Deprecated in v0.4.0** — use `type`. Pre-v0.4 engine slug (`postgres-15`, `postgres-16`, `mysql-8`).",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
				DeprecationMessage:  "Use `type` instead — v0.4.0 exposes Cloud's v2 cluster-type slugs.",
			},
			"size": schema.StringAttribute{
				MarkdownDescription: "Cluster size — used with pre-v0.4 `engine`. Optional in v0.4.0.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"high_availability": schema.BoolAttribute{
				MarkdownDescription: "When true, cluster runs with a hot standby. Optional in v0.4.0.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"backup_retention_days": schema.Int64Attribute{
				MarkdownDescription: "Backup retention in days.",
				Optional:            true,
				Computed:            true,
			},
			"created_at": schema.StringAttribute{Computed: true},
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

	apiReq := api.CreateDatabaseClusterRequest{Name: plan.Name.ValueString()}
	if !plan.OrganizationID.IsNull() && !plan.OrganizationID.IsUnknown() {
		apiReq.OrganizationID = plan.OrganizationID.ValueString()
	}
	if !plan.Region.IsNull() && !plan.Region.IsUnknown() {
		apiReq.Region = plan.Region.ValueString()
	}
	if !plan.Type.IsNull() && !plan.Type.IsUnknown() {
		v := plan.Type.ValueString()
		apiReq.Type = &v
	}
	if !plan.Config.IsNull() && !plan.Config.IsUnknown() {
		cfg, d := extractConfigMap(ctx, plan.Config)
		resp.Diagnostics.Append(d...)
		if resp.Diagnostics.HasError() {
			return
		}
		apiReq.Config = cfg
	}
	if !plan.Engine.IsNull() && !plan.Engine.IsUnknown() {
		apiReq.Engine = plan.Engine.ValueString()
	}
	if !plan.Size.IsNull() && !plan.Size.IsUnknown() {
		apiReq.Size = plan.Size.ValueString()
	}
	if !plan.HighAvailability.IsNull() && !plan.HighAvailability.IsUnknown() {
		apiReq.HighAvailability = plan.HighAvailability.ValueBool()
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
	if !plan.Config.IsNull() && !plan.Config.IsUnknown() {
		cfg, d := extractConfigMap(ctx, plan.Config)
		resp.Diagnostics.Append(d...)
		if resp.Diagnostics.HasError() {
			return
		}
		apiReq.Config = cfg
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

// extractConfigMap converts a Terraform map<string, string> to Go's
// map[string]any, decoding JSON-serialised values when possible so
// numbers/bools round-trip cleanly. Consumers pass every value as a
// string on the HCL side; extractConfigMap best-effort parses each
// value as JSON (numbers, bools, arrays, nested objects), falling back
// to the raw string when parse fails.
func extractConfigMap(ctx context.Context, m types.Map) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	raw := make(map[string]string, len(m.Elements()))
	d := m.ElementsAs(ctx, &raw, false)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}
	out := make(map[string]any, len(raw))
	for k, v := range raw {
		// Try to parse as JSON scalar/nested; fall back to raw string.
		var parsed any
		if err := json.Unmarshal([]byte(v), &parsed); err == nil {
			out[k] = parsed
		} else {
			out[k] = v
		}
	}
	return out, diags
}

// applyDBClusterToModel copies API DTO -> Terraform state model.
func applyDBClusterToModel(cluster *api.DatabaseCluster, m *DatabaseClusterResourceModel) {
	m.ID = types.StringValue(cluster.ID)
	m.OrganizationID = types.StringValue(cluster.OrganizationID)
	m.Name = types.StringValue(cluster.Name)
	m.Region = types.StringValue(cluster.Region)
	setStringPtr(&m.Type, cluster.Type)

	// Config — encode back to string map for state.
	if cluster.Config != nil {
		vals := make(map[string]attr.Value, len(cluster.Config))
		for k, v := range cluster.Config {
			// Serialise back to JSON so numbers/bools survive round-trip.
			b, err := json.Marshal(v)
			if err == nil {
				vals[k] = types.StringValue(string(b))
			} else {
				vals[k] = types.StringValue(fmt.Sprintf("%v", v))
			}
		}
		m.Config = types.MapValueMust(types.StringType, vals)
	} else {
		m.Config = types.MapNull(types.StringType)
	}

	if cluster.Engine != "" {
		m.Engine = types.StringValue(cluster.Engine)
	} else {
		m.Engine = types.StringNull()
	}
	if cluster.Size != "" {
		m.Size = types.StringValue(cluster.Size)
	} else {
		m.Size = types.StringNull()
	}
	m.HighAvailability = types.BoolValue(cluster.HighAvailability)
	m.BackupRetentionDays = types.Int64Value(int64(cluster.BackupRetentionDays))
	setStringPtr(&m.CreatedAt, cluster.CreatedAt)
}

var (
	_ resource.Resource                = (*DatabaseClusterResource)(nil)
	_ resource.ResourceWithImportState = (*DatabaseClusterResource)(nil)
	_ resource.ResourceWithConfigure   = (*DatabaseClusterResource)(nil)
)
