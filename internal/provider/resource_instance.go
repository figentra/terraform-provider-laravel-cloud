package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/figentra/terraform-provider-laravel-cloud/internal/api"
)

// InstanceResource manages a `laravelcloud_instance` — one compute unit
// on an environment. An environment carries at least one Instance
// (`type = app`) plus optionally `queue` / `service` / `serverless_queue`
// instances for horizon workers, custom services, or serverless jobs.
type InstanceResource struct {
	client *api.Client
}

type InstanceResourceModel struct {
	ID            types.String `tfsdk:"id"`
	EnvironmentID types.String `tfsdk:"environment_id"`
	Name          types.String `tfsdk:"name"`
	Type          types.String `tfsdk:"type"`
	Size          types.String `tfsdk:"size"`
	ScalingType   types.String `tfsdk:"scaling_type"`
	MinReplicas   types.Int64  `tfsdk:"min_replicas"`
	MaxReplicas   types.Int64  `tfsdk:"max_replicas"`

	UsesScheduler   types.Bool  `tfsdk:"uses_scheduler"`
	UsesSleepMode   types.Bool  `tfsdk:"uses_sleep_mode"`
	SleepTimeout    types.Int64 `tfsdk:"sleep_timeout"`
	UsesOctane      types.Bool  `tfsdk:"uses_octane"`
	UsesInertiaSsr  types.Bool  `tfsdk:"uses_inertia_ssr"`

	ScalingCpuThresholdPercentage    types.Int64 `tfsdk:"scaling_cpu_threshold_percentage"`
	ScalingMemoryThresholdPercentage types.Int64 `tfsdk:"scaling_memory_threshold_percentage"`

	CreatedAt types.String `tfsdk:"created_at"`
}

func NewInstanceResource() resource.Resource { return &InstanceResource{} }

func (r *InstanceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_instance"
}

func (r *InstanceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Laravel Cloud compute instance on an " +
			"environment. Every environment has at least one `type = app` " +
			"instance running the HTTP surface + optionally `queue` / " +
			"`service` / `serverless_queue` instances for horizon workers, " +
			"custom daemons, or serverless jobs. Size + autoscale + Octane + " +
			"hibernation are all authored here. Added in v0.5.0.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Cloud-assigned instance ID.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"environment_id": schema.StringAttribute{
				MarkdownDescription: "Parent environment. Immutable post-create.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable instance name shown in the Cloud dashboard.",
				Required:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Instance type — `app`, `service`, `queue`, or `serverless_queue`. " +
					"Immutable post-create.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"size": schema.StringAttribute{
				MarkdownDescription: "Instance size slug — see the SDK's `InstanceSize` enum " +
					"for the canonical set. Common values: `flex.g-1vcpu-512mb`, " +
					"`flex.g-2vcpu-1gb`, `pro.g-2vcpu-4gb`. Mutable — Cloud performs " +
					"a rolling replace to the new size.",
				Required: true,
			},
			"scaling_type": schema.StringAttribute{
				MarkdownDescription: "Scaling strategy — `none`, `custom`, or `auto`. " +
					"When `none`, exactly `min_replicas` instances run. When " +
					"`custom`, replicas scale between `min_replicas` and `max_replicas` " +
					"driven by the operator. When `auto`, Cloud drives replicas based " +
					"on `scaling_cpu_threshold_percentage` + " +
					"`scaling_memory_threshold_percentage`.",
				Required: true,
			},
			"min_replicas": schema.Int64Attribute{
				MarkdownDescription: "Minimum replica count.",
				Required:            true,
			},
			"max_replicas": schema.Int64Attribute{
				MarkdownDescription: "Maximum replica count. Must be >= `min_replicas`.",
				Required:            true,
			},

			// Optional-ish toggles (Cloud provides sensible defaults; expose
			// them so callers can flip explicitly).
			"uses_scheduler": schema.BoolAttribute{
				MarkdownDescription: "Whether this instance runs `php artisan schedule:work`. " +
					"Typically set on a single per-env instance (usually the `app` " +
					"type) so cron doesn't multiply. Defaults to false.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"uses_sleep_mode": schema.BoolAttribute{
				MarkdownDescription: "Whether this instance hibernates when idle (Cloud " +
					"scale-to-zero). Defaults to false. Only usable when " +
					"`scaling_type = none` and `min_replicas = 1`.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"sleep_timeout": schema.Int64Attribute{
				MarkdownDescription: "Seconds of idle before hibernation kicks in. Only " +
					"meaningful when `uses_sleep_mode = true`.",
				Optional: true,
				Computed: true,
			},
			"uses_octane": schema.BoolAttribute{
				MarkdownDescription: "Whether the app runs under Laravel Octane. Cloud " +
					"provisions the Swoole/Roadrunner worker pool + reuses PHP " +
					"processes across requests when true. Defaults to false.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"uses_inertia_ssr": schema.BoolAttribute{
				MarkdownDescription: "Whether the app uses Inertia server-side rendering. " +
					"Provisions the SSR Node worker alongside PHP. Defaults to false.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"scaling_cpu_threshold_percentage": schema.Int64Attribute{
				MarkdownDescription: "CPU threshold (0-100) driving autoscale-up when " +
					"`scaling_type = auto`. Null when scaling_type is `none` or `custom`.",
				Optional: true,
				Computed: true,
			},
			"scaling_memory_threshold_percentage": schema.Int64Attribute{
				MarkdownDescription: "Memory threshold (0-100) driving autoscale-up when " +
					"`scaling_type = auto`. Null when scaling_type is `none` or `custom`.",
				Optional: true,
				Computed: true,
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp of instance creation.",
			},
		},
	}
}

func (r *InstanceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *InstanceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan InstanceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := api.CreateInstanceRequest{
		Name:        plan.Name.ValueString(),
		Type:        plan.Type.ValueString(),
		Size:        plan.Size.ValueString(),
		ScalingType: plan.ScalingType.ValueString(),
		MinReplicas: int(plan.MinReplicas.ValueInt64()),
		MaxReplicas: int(plan.MaxReplicas.ValueInt64()),
	}

	if !plan.UsesScheduler.IsNull() && !plan.UsesScheduler.IsUnknown() {
		v := plan.UsesScheduler.ValueBool()
		apiReq.UsesScheduler = &v
	}
	if !plan.ScalingCpuThresholdPercentage.IsNull() && !plan.ScalingCpuThresholdPercentage.IsUnknown() {
		v := int(plan.ScalingCpuThresholdPercentage.ValueInt64())
		apiReq.ScalingCpuThresholdPercentage = &v
	}
	if !plan.ScalingMemoryThresholdPercentage.IsNull() && !plan.ScalingMemoryThresholdPercentage.IsUnknown() {
		v := int(plan.ScalingMemoryThresholdPercentage.ValueInt64())
		apiReq.ScalingMemoryThresholdPercentage = &v
	}

	tflog.Info(ctx, "Creating Laravel Cloud instance", map[string]any{
		"environment_id": plan.EnvironmentID.ValueString(),
		"name":           apiReq.Name,
		"size":           apiReq.Size,
		"scaling_type":   apiReq.ScalingType,
	})

	inst, err := r.client.CreateInstance(ctx, plan.EnvironmentID.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to create instance",
			"Cloud API returned an error while creating the instance. "+
				"Original error: "+err.Error(),
		)
		return
	}

	// Immediate follow-up PATCH — CreateInstance's schema doesn't accept
	// every mutable flag (uses_octane, uses_sleep_mode, sleep_timeout,
	// uses_inertia_ssr). PATCH them via UpdateInstance if the plan set
	// any non-default values.
	patch := api.UpdateInstanceRequest{}
	patchNeeded := false
	if !plan.UsesOctane.IsNull() && !plan.UsesOctane.IsUnknown() && plan.UsesOctane.ValueBool() {
		v := true
		patch.UsesOctane = &v
		patchNeeded = true
	}
	if !plan.UsesSleepMode.IsNull() && !plan.UsesSleepMode.IsUnknown() && plan.UsesSleepMode.ValueBool() {
		v := true
		patch.UsesSleepMode = &v
		patchNeeded = true
	}
	if !plan.SleepTimeout.IsNull() && !plan.SleepTimeout.IsUnknown() {
		v := int(plan.SleepTimeout.ValueInt64())
		patch.SleepTimeout = &v
		patchNeeded = true
	}
	if !plan.UsesInertiaSsr.IsNull() && !plan.UsesInertiaSsr.IsUnknown() && plan.UsesInertiaSsr.ValueBool() {
		v := true
		patch.UsesInertiaSsr = &v
		patchNeeded = true
	}
	if patchNeeded {
		if updated, patchErr := r.client.UpdateInstance(ctx, inst.ID, patch); patchErr == nil {
			inst = updated
		}
	}

	applyInstanceToModel(inst, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *InstanceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state InstanceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	inst, err := r.client.GetInstance(ctx, state.ID.ValueString())
	if err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read instance", err.Error())
		return
	}

	applyInstanceToModel(inst, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *InstanceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan InstanceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	patch := api.UpdateInstanceRequest{}
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		v := plan.Name.ValueString()
		patch.Name = &v
	}
	if !plan.Size.IsNull() && !plan.Size.IsUnknown() {
		v := plan.Size.ValueString()
		patch.Size = &v
	}
	if !plan.ScalingType.IsNull() && !plan.ScalingType.IsUnknown() {
		v := plan.ScalingType.ValueString()
		patch.ScalingType = &v
	}
	if !plan.MinReplicas.IsNull() && !plan.MinReplicas.IsUnknown() {
		v := int(plan.MinReplicas.ValueInt64())
		patch.MinReplicas = &v
	}
	if !plan.MaxReplicas.IsNull() && !plan.MaxReplicas.IsUnknown() {
		v := int(plan.MaxReplicas.ValueInt64())
		patch.MaxReplicas = &v
	}
	if !plan.UsesSleepMode.IsNull() && !plan.UsesSleepMode.IsUnknown() {
		v := plan.UsesSleepMode.ValueBool()
		patch.UsesSleepMode = &v
	}
	if !plan.SleepTimeout.IsNull() && !plan.SleepTimeout.IsUnknown() {
		v := int(plan.SleepTimeout.ValueInt64())
		patch.SleepTimeout = &v
	}
	if !plan.UsesScheduler.IsNull() && !plan.UsesScheduler.IsUnknown() {
		v := plan.UsesScheduler.ValueBool()
		patch.UsesScheduler = &v
	}
	if !plan.UsesOctane.IsNull() && !plan.UsesOctane.IsUnknown() {
		v := plan.UsesOctane.ValueBool()
		patch.UsesOctane = &v
	}
	if !plan.UsesInertiaSsr.IsNull() && !plan.UsesInertiaSsr.IsUnknown() {
		v := plan.UsesInertiaSsr.ValueBool()
		patch.UsesInertiaSsr = &v
	}
	if !plan.ScalingCpuThresholdPercentage.IsNull() && !plan.ScalingCpuThresholdPercentage.IsUnknown() {
		v := int(plan.ScalingCpuThresholdPercentage.ValueInt64())
		patch.ScalingCpuThresholdPercentage = &v
	}
	if !plan.ScalingMemoryThresholdPercentage.IsNull() && !plan.ScalingMemoryThresholdPercentage.IsUnknown() {
		v := int(plan.ScalingMemoryThresholdPercentage.ValueInt64())
		patch.ScalingMemoryThresholdPercentage = &v
	}

	inst, err := r.client.UpdateInstance(ctx, plan.ID.ValueString(), patch)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update instance", err.Error())
		return
	}
	applyInstanceToModel(inst, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *InstanceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state InstanceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteInstance(ctx, state.ID.ValueString()); err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError("Failed to delete instance", err.Error())
	}
}

func (r *InstanceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func applyInstanceToModel(inst *api.Instance, model *InstanceResourceModel) {
	model.ID = types.StringValue(inst.ID)
	if inst.Name != "" {
		model.Name = types.StringValue(inst.Name)
	}
	if inst.Type != "" {
		model.Type = types.StringValue(inst.Type)
	}
	if inst.Size != "" {
		model.Size = types.StringValue(inst.Size)
	}
	if inst.ScalingType != "" {
		model.ScalingType = types.StringValue(inst.ScalingType)
	}
	if inst.EnvironmentID != "" {
		model.EnvironmentID = types.StringValue(inst.EnvironmentID)
	}
	model.MinReplicas = types.Int64Value(int64(inst.MinReplicas))
	model.MaxReplicas = types.Int64Value(int64(inst.MaxReplicas))
	model.UsesScheduler = types.BoolValue(inst.UsesScheduler)
	model.UsesSleepMode = types.BoolValue(inst.UsesSleepMode)
	if inst.SleepTimeout != nil {
		model.SleepTimeout = types.Int64Value(int64(*inst.SleepTimeout))
	} else {
		model.SleepTimeout = types.Int64Null()
	}
	model.UsesOctane = types.BoolValue(inst.UsesOctane)
	model.UsesInertiaSsr = types.BoolValue(inst.UsesInertiaSsr)
	if inst.ScalingCpuThresholdPercentage != nil {
		model.ScalingCpuThresholdPercentage = types.Int64Value(int64(*inst.ScalingCpuThresholdPercentage))
	} else {
		model.ScalingCpuThresholdPercentage = types.Int64Null()
	}
	if inst.ScalingMemoryThresholdPercentage != nil {
		model.ScalingMemoryThresholdPercentage = types.Int64Value(int64(*inst.ScalingMemoryThresholdPercentage))
	} else {
		model.ScalingMemoryThresholdPercentage = types.Int64Null()
	}
	if inst.CreatedAt != nil {
		model.CreatedAt = types.StringValue(inst.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	} else {
		model.CreatedAt = types.StringNull()
	}
}

var (
	_ resource.Resource                = (*InstanceResource)(nil)
	_ resource.ResourceWithImportState = (*InstanceResource)(nil)
	_ resource.ResourceWithConfigure   = (*InstanceResource)(nil)
)
