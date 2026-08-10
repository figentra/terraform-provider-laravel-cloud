package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/figentra/terraform-provider-laravel-cloud/internal/api"
)

// DatabaseSnapshotResource manages a manual snapshot on a database
// cluster. Automated snapshots (Cloud's scheduled backups) surface in
// state via `data.laravelcloud_database_snapshots` but aren't managed
// here — Cloud owns their lifecycle.
//
// Import path: `<cluster_id>:<snapshot_id>` — Cloud's snapshot
// endpoints are cluster-scoped, so both IDs are required.
//
// Added in v0.5.0.
type DatabaseSnapshotResource struct {
	client *api.Client
}

type DatabaseSnapshotResourceModel struct {
	ID          types.String `tfsdk:"id"`
	ClusterID   types.String `tfsdk:"cluster_id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`

	Type         types.String `tfsdk:"type"`
	Status       types.String `tfsdk:"status"`
	StorageBytes types.Int64  `tfsdk:"storage_bytes"`
	PitrEnabled  types.Bool   `tfsdk:"pitr_enabled"`
	PitrEndsAt   types.String `tfsdk:"pitr_ends_at"`
	CompletedAt  types.String `tfsdk:"completed_at"`
	CreatedAt    types.String `tfsdk:"created_at"`
}

func NewDatabaseSnapshotResource() resource.Resource {
	return &DatabaseSnapshotResource{}
}

func (r *DatabaseSnapshotResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_database_snapshot"
}

func (r *DatabaseSnapshotResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a manual snapshot on a Laravel Cloud database " +
			"cluster. Automated (Cloud-scheduled) snapshots surface via " +
			"`data.laravelcloud_database_snapshots` but aren't managed here. " +
			"Added in v0.5.0.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"cluster_id": schema.StringAttribute{
				MarkdownDescription: "Parent cluster. Immutable.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable snapshot name. Immutable post-create.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Free-form description. Immutable post-create.",
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Snapshot type — `manual` or `automated`.",
				Computed:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Snapshot status — `pending`, `creating`, `available`, " +
					"`failed`, `deleting`.",
				Computed: true,
			},
			"storage_bytes": schema.Int64Attribute{
				MarkdownDescription: "Snapshot size in bytes. Null until status = available.",
				Computed:            true,
			},
			"pitr_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether point-in-time recovery is enabled for this snapshot.",
				Computed:            true,
			},
			"pitr_ends_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp when PITR expires.",
				Computed:            true,
			},
			"completed_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp when the snapshot finished. Null while creating.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (r *DatabaseSnapshotResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*api.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("expected *api.Client, got %T", req.ProviderData))
		return
	}
	r.client = client
}

func (r *DatabaseSnapshotResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DatabaseSnapshotResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	apiReq := api.CreateDatabaseSnapshotRequest{
		Name: plan.Name.ValueString(),
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		v := plan.Description.ValueString()
		apiReq.Description = &v
	}

	tflog.Info(ctx, "Creating database snapshot", map[string]any{
		"cluster_id": plan.ClusterID.ValueString(),
		"name":       apiReq.Name,
	})

	s, err := r.client.CreateDatabaseSnapshot(ctx, plan.ClusterID.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create database snapshot", err.Error())
		return
	}
	applyDatabaseSnapshotToModel(s, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DatabaseSnapshotResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DatabaseSnapshotResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	s, err := r.client.GetDatabaseSnapshot(ctx, state.ClusterID.ValueString(), state.ID.ValueString())
	if err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read database snapshot", err.Error())
		return
	}
	applyDatabaseSnapshotToModel(s, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *DatabaseSnapshotResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Every field is RequiresReplace — should never route here.
	var plan DatabaseSnapshotResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DatabaseSnapshotResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DatabaseSnapshotResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteDatabaseSnapshot(ctx, state.ClusterID.ValueString(), state.ID.ValueString()); err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError("Failed to delete database snapshot", err.Error())
	}
}

// ImportState splits `<cluster_id>:<snapshot_id>`.
func (r *DatabaseSnapshotResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			"Expected `<cluster_id>:<snapshot_id>`. Got: "+req.ID,
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cluster_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func applyDatabaseSnapshotToModel(s *api.DatabaseSnapshot, model *DatabaseSnapshotResourceModel) {
	model.ID = types.StringValue(s.ID)
	if s.ClusterID != "" {
		model.ClusterID = types.StringValue(s.ClusterID)
	}
	model.Name = types.StringValue(s.Name)
	if s.Description != nil {
		model.Description = types.StringValue(*s.Description)
	} else {
		model.Description = types.StringNull()
	}
	model.Type = types.StringValue(s.Type)
	model.Status = types.StringValue(s.Status)
	if s.StorageBytes != nil {
		model.StorageBytes = types.Int64Value(int64(*s.StorageBytes))
	} else {
		model.StorageBytes = types.Int64Null()
	}
	if s.PitrEnabled != nil {
		model.PitrEnabled = types.BoolValue(*s.PitrEnabled)
	} else {
		model.PitrEnabled = types.BoolNull()
	}
	if s.PitrEndsAt != nil {
		model.PitrEndsAt = types.StringValue(s.PitrEndsAt.Format(time.RFC3339))
	} else {
		model.PitrEndsAt = types.StringNull()
	}
	if s.CompletedAt != nil {
		model.CompletedAt = types.StringValue(s.CompletedAt.Format(time.RFC3339))
	} else {
		model.CompletedAt = types.StringNull()
	}
	if s.CreatedAt != nil {
		model.CreatedAt = types.StringValue(s.CreatedAt.Format(time.RFC3339))
	} else {
		model.CreatedAt = types.StringNull()
	}
}

var (
	_ resource.Resource                = (*DatabaseSnapshotResource)(nil)
	_ resource.ResourceWithImportState = (*DatabaseSnapshotResource)(nil)
	_ resource.ResourceWithConfigure   = (*DatabaseSnapshotResource)(nil)
)
