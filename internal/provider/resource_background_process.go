package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/figentra/terraform-provider-laravel-cloud/internal/api"
)

// BackgroundProcessResource manages a `laravelcloud_background_process`
// — one daemon on an Instance. Two shapes:
//
//  1. `type = "worker"` — Laravel queue worker managed by Horizon.
//     `command` is null; `config` carries queue-worker knobs
//     (connection, queue, tries, backoff, sleep, rest, timeout, force).
//
//  2. `type = "custom"` — any long-lived process. `command` REQUIRED;
//     `config` typically null.
//
// Added in v0.5.0.
type BackgroundProcessResource struct {
	client *api.Client
}

// BackgroundProcessConfigModel maps to the nested `config` object.
type BackgroundProcessConfigModel struct {
	Connection types.String `tfsdk:"connection"`
	Queue      types.String `tfsdk:"queue"`
	Tries      types.Int64  `tfsdk:"tries"`
	Backoff    types.Int64  `tfsdk:"backoff"`
	Sleep      types.Int64  `tfsdk:"sleep"`
	Rest       types.Int64  `tfsdk:"rest"`
	Timeout    types.Int64  `tfsdk:"timeout"`
	Force      types.Bool   `tfsdk:"force"`
}

type BackgroundProcessResourceModel struct {
	ID         types.String `tfsdk:"id"`
	InstanceID types.String `tfsdk:"instance_id"`
	Type       types.String `tfsdk:"type"`
	Processes  types.Int64  `tfsdk:"processes"`
	Command    types.String `tfsdk:"command"`
	Config     types.Object `tfsdk:"config"`
	CreatedAt  types.String `tfsdk:"created_at"`
}

func NewBackgroundProcessResource() resource.Resource {
	return &BackgroundProcessResource{}
}

func (r *BackgroundProcessResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_background_process"
}

// configAttrTypes returns the framework type shape for the `config` nested
// object — used to construct null values + hydrate from state.
func configAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"connection": types.StringType,
		"queue":      types.StringType,
		"tries":      types.Int64Type,
		"backoff":    types.Int64Type,
		"sleep":      types.Int64Type,
		"rest":       types.Int64Type,
		"timeout":    types.Int64Type,
		"force":      types.BoolType,
	}
}

func (r *BackgroundProcessResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a background process (daemon) on a " +
			"Laravel Cloud instance. Two flavors: `type = worker` for Laravel " +
			"queue workers managed by Horizon (config carries queue-worker " +
			"knobs), or `type = custom` for any long-lived process (command " +
			"REQUIRED). Added in v0.5.0.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Cloud-assigned daemon ID.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"instance_id": schema.StringAttribute{
				MarkdownDescription: "Parent instance. Immutable — changing forces replace.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Daemon type — `worker` (Horizon-managed queue worker) " +
					"or `custom` (long-lived process; requires `command`).",
				Required: true,
			},
			"processes": schema.Int64Attribute{
				MarkdownDescription: "Number of concurrent processes to run. For workers, " +
					"this multiplies Horizon's throughput.",
				Required: true,
			},
			"command": schema.StringAttribute{
				MarkdownDescription: "Shell command to run. REQUIRED when `type = custom`. " +
					"Null when `type = worker` — Cloud generates the Horizon command " +
					"from the `config` block.",
				Optional: true,
			},
			"config": schema.SingleNestedAttribute{
				MarkdownDescription: "Queue-worker tuning. Meaningful when `type = worker`. " +
					"Field names match `php artisan queue:work` flags.",
				Optional: true,
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"connection": schema.StringAttribute{
						MarkdownDescription: "Queue connection name (from `config/queue.php`).",
						Optional:            true,
					},
					"queue": schema.StringAttribute{
						MarkdownDescription: "Queue name — comma-separated priority list " +
							"is accepted (e.g. `high,default,low`).",
						Optional: true,
					},
					"tries": schema.Int64Attribute{
						MarkdownDescription: "Max attempts per job.",
						Optional:            true,
					},
					"backoff": schema.Int64Attribute{
						MarkdownDescription: "Seconds between retries.",
						Optional:            true,
					},
					"sleep": schema.Int64Attribute{
						MarkdownDescription: "Seconds to sleep between empty polls.",
						Optional:            true,
					},
					"rest": schema.Int64Attribute{
						MarkdownDescription: "Seconds to rest between jobs.",
						Optional:            true,
					},
					"timeout": schema.Int64Attribute{
						MarkdownDescription: "Max seconds per job before Horizon kills it.",
						Optional:            true,
					},
					"force": schema.BoolAttribute{
						MarkdownDescription: "Force queue workers to run in maintenance mode.",
						Optional:            true,
					},
				},
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp of creation.",
			},
		},
	}
}

func (r *BackgroundProcessResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// hydrateConfigFromPlan turns the plan's `config` types.Object into a
// pointer-to-struct suitable for the API request. Returns nil when the
// plan omitted the block.
func hydrateConfigFromPlan(ctx context.Context, obj types.Object) (*api.BackgroundProcessConfig, error) {
	if obj.IsNull() || obj.IsUnknown() {
		return nil, nil
	}
	var m BackgroundProcessConfigModel
	if diags := obj.As(ctx, &m, basetypes.ObjectAsOptions{UnhandledNullAsEmpty: true, UnhandledUnknownAsEmpty: true}); diags.HasError() {
		return nil, fmt.Errorf("decode config: %v", diags)
	}
	out := &api.BackgroundProcessConfig{}
	if !m.Connection.IsNull() && !m.Connection.IsUnknown() {
		v := m.Connection.ValueString()
		out.Connection = &v
	}
	if !m.Queue.IsNull() && !m.Queue.IsUnknown() {
		v := m.Queue.ValueString()
		out.Queue = &v
	}
	if !m.Tries.IsNull() && !m.Tries.IsUnknown() {
		v := int(m.Tries.ValueInt64())
		out.Tries = &v
	}
	if !m.Backoff.IsNull() && !m.Backoff.IsUnknown() {
		v := int(m.Backoff.ValueInt64())
		out.Backoff = &v
	}
	if !m.Sleep.IsNull() && !m.Sleep.IsUnknown() {
		v := int(m.Sleep.ValueInt64())
		out.Sleep = &v
	}
	if !m.Rest.IsNull() && !m.Rest.IsUnknown() {
		v := int(m.Rest.ValueInt64())
		out.Rest = &v
	}
	if !m.Timeout.IsNull() && !m.Timeout.IsUnknown() {
		v := int(m.Timeout.ValueInt64())
		out.Timeout = &v
	}
	if !m.Force.IsNull() && !m.Force.IsUnknown() {
		v := m.Force.ValueBool()
		out.Force = &v
	}
	return out, nil
}

func (r *BackgroundProcessResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan BackgroundProcessResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := api.CreateBackgroundProcessRequest{
		Type:      plan.Type.ValueString(),
		Processes: int(plan.Processes.ValueInt64()),
	}
	if !plan.Command.IsNull() && !plan.Command.IsUnknown() {
		v := plan.Command.ValueString()
		apiReq.Command = &v
	}
	cfg, err := hydrateConfigFromPlan(ctx, plan.Config)
	if err != nil {
		resp.Diagnostics.AddError("Failed to hydrate config", err.Error())
		return
	}
	apiReq.Config = cfg

	tflog.Info(ctx, "Creating Laravel Cloud background process", map[string]any{
		"instance_id": plan.InstanceID.ValueString(),
		"type":        apiReq.Type,
		"processes":   apiReq.Processes,
	})

	bp, err := r.client.CreateBackgroundProcess(ctx, plan.InstanceID.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create background process", err.Error())
		return
	}

	applyBackgroundProcessToModel(ctx, bp, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *BackgroundProcessResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state BackgroundProcessResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	bp, err := r.client.GetBackgroundProcess(ctx, state.ID.ValueString())
	if err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read background process", err.Error())
		return
	}
	applyBackgroundProcessToModel(ctx, bp, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *BackgroundProcessResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan BackgroundProcessResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	patch := api.UpdateBackgroundProcessRequest{}
	if !plan.Type.IsNull() && !plan.Type.IsUnknown() {
		v := plan.Type.ValueString()
		patch.Type = &v
	}
	if !plan.Processes.IsNull() && !plan.Processes.IsUnknown() {
		v := int(plan.Processes.ValueInt64())
		patch.Processes = &v
	}
	if !plan.Command.IsNull() && !plan.Command.IsUnknown() {
		v := plan.Command.ValueString()
		patch.Command = &v
	}
	cfg, err := hydrateConfigFromPlan(ctx, plan.Config)
	if err != nil {
		resp.Diagnostics.AddError("Failed to hydrate config", err.Error())
		return
	}
	patch.Config = cfg

	bp, err := r.client.UpdateBackgroundProcess(ctx, plan.ID.ValueString(), patch)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update background process", err.Error())
		return
	}
	applyBackgroundProcessToModel(ctx, bp, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *BackgroundProcessResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state BackgroundProcessResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteBackgroundProcess(ctx, state.ID.ValueString()); err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError("Failed to delete background process", err.Error())
	}
}

func (r *BackgroundProcessResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func applyBackgroundProcessToModel(ctx context.Context, bp *api.BackgroundProcess, model *BackgroundProcessResourceModel) {
	model.ID = types.StringValue(bp.ID)
	if bp.InstanceID != "" {
		model.InstanceID = types.StringValue(bp.InstanceID)
	}
	if bp.Type != "" {
		model.Type = types.StringValue(bp.Type)
	}
	model.Processes = types.Int64Value(int64(bp.Processes))
	if bp.Command != nil {
		model.Command = types.StringValue(*bp.Command)
	} else {
		model.Command = types.StringNull()
	}

	// Hydrate config object.
	if bp.Config == nil {
		model.Config = types.ObjectNull(configAttrTypes())
	} else {
		vals := map[string]attr.Value{
			"connection": types.StringNull(),
			"queue":      types.StringNull(),
			"tries":      types.Int64Null(),
			"backoff":    types.Int64Null(),
			"sleep":      types.Int64Null(),
			"rest":       types.Int64Null(),
			"timeout":    types.Int64Null(),
			"force":      types.BoolNull(),
		}
		if bp.Config.Connection != nil {
			vals["connection"] = types.StringValue(*bp.Config.Connection)
		}
		if bp.Config.Queue != nil {
			vals["queue"] = types.StringValue(*bp.Config.Queue)
		}
		if bp.Config.Tries != nil {
			vals["tries"] = types.Int64Value(int64(*bp.Config.Tries))
		}
		if bp.Config.Backoff != nil {
			vals["backoff"] = types.Int64Value(int64(*bp.Config.Backoff))
		}
		if bp.Config.Sleep != nil {
			vals["sleep"] = types.Int64Value(int64(*bp.Config.Sleep))
		}
		if bp.Config.Rest != nil {
			vals["rest"] = types.Int64Value(int64(*bp.Config.Rest))
		}
		if bp.Config.Timeout != nil {
			vals["timeout"] = types.Int64Value(int64(*bp.Config.Timeout))
		}
		if bp.Config.Force != nil {
			vals["force"] = types.BoolValue(*bp.Config.Force)
		}
		obj, _ := types.ObjectValue(configAttrTypes(), vals)
		model.Config = obj
	}

	if bp.CreatedAt != nil {
		model.CreatedAt = types.StringValue(bp.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	} else {
		model.CreatedAt = types.StringNull()
	}
}

var (
	_ resource.Resource                = (*BackgroundProcessResource)(nil)
	_ resource.ResourceWithImportState = (*BackgroundProcessResource)(nil)
	_ resource.ResourceWithConfigure   = (*BackgroundProcessResource)(nil)
)
