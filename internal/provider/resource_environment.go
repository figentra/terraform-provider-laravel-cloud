package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/figentra/terraform-provider-laravel-cloud/internal/api"
)

// EnvironmentResource manages `laravelcloud_environment` — a per-app env
// (dev / stg / prd / preview-*) with branch binding + env-vars map.
//
// v0.4.0 attribute expansion:
//   - node_version / build_command / deploy_command — deploy runtime
//   - uses_push_to_deploy / uses_deploy_hook / uses_octane / uses_hibernation
//     — deploy toggles
//   - color — visual identifier in Cloud dashboard
//   - database_schema_id / cache_id / websocket_application_id — FK
//     bindings to sibling resources
type EnvironmentResource struct{ client *api.Client }

// EnvironmentResourceModel maps HCL <-> API DTO.
type EnvironmentResourceModel struct {
	ID                     types.String `tfsdk:"id"`
	ApplicationID          types.String `tfsdk:"application_id"`
	Name                   types.String `tfsdk:"name"`
	Branch                 types.String `tfsdk:"branch"`
	Variables              types.Map    `tfsdk:"variables"`
	InheritsID             types.String `tfsdk:"inherits_id"`
	DatabaseSchemaID       types.String `tfsdk:"database_schema_id"`
	CacheID                types.String `tfsdk:"cache_id"`
	WebsocketApplicationID types.String `tfsdk:"websocket_application_id"`

	// Runtime + build config (v0.4.0)
	NodeVersion   types.String `tfsdk:"node_version"`
	BuildCommand  types.String `tfsdk:"build_command"`
	DeployCommand types.String `tfsdk:"deploy_command"`

	// Deploy toggles (v0.4.0)
	UsesPushToDeploy types.Bool `tfsdk:"uses_push_to_deploy"`
	UsesDeployHook   types.Bool `tfsdk:"uses_deploy_hook"`
	UsesOctane       types.Bool `tfsdk:"uses_octane"`
	UsesHibernation  types.Bool `tfsdk:"uses_hibernation"`

	Color        types.String `tfsdk:"color"`
	VanityDomain types.String `tfsdk:"vanity_domain"`
	CreatedAt    types.String `tfsdk:"created_at"`
}

// NewEnvironmentResource is the plugin-framework factory.
func NewEnvironmentResource() resource.Resource { return &EnvironmentResource{} }

func (r *EnvironmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment"
}

func (r *EnvironmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Cloud environment. One env per branch (`dev` → develop, `stg` → staging, `prd` → main). Every deploy-time knob (node version, build/deploy commands, push-to-deploy toggles, hibernation, octane, color) is expressible via the v0.4.0 attribute expansion.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Cloud-assigned environment ID.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"application_id": schema.StringAttribute{
				MarkdownDescription: "Parent application ID. Immutable — forces replace.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Env slug — `dev`, `stg`, `prd`, or `preview-*`. Immutable.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"branch": schema.StringAttribute{
				MarkdownDescription: "Git branch the env auto-deploys from. Nullable.",
				Optional:            true,
			},
			"variables": schema.MapAttribute{
				MarkdownDescription: "Env-var map merged into the deployed process' environment. Secrets flow via Doppler binding, not here.",
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
			},
			"inherits_id": schema.StringAttribute{
				MarkdownDescription: "Optional parent env ID — this env inherits vars + defaults from the parent.",
				Optional:            true,
			},
			"database_schema_id": schema.StringAttribute{
				MarkdownDescription: "Database schema this env writes to. Nullable — envs without a DB attach skip this.",
				Optional:            true,
			},
			"cache_id": schema.StringAttribute{
				MarkdownDescription: "Cache instance bound to this env. Nullable.",
				Optional:            true,
			},
			"websocket_application_id": schema.StringAttribute{
				MarkdownDescription: "WebSocket app binding for this env. Nullable — envs without WS attach skip this.",
				Optional:            true,
			},
			"node_version": schema.StringAttribute{
				MarkdownDescription: "Node runtime version for the build — `18`, `20`, `22`, `24`. Added in v0.4.0. Nullable — Cloud picks a default.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"build_command": schema.StringAttribute{
				MarkdownDescription: "Build command run before deploy. Added in v0.4.0. Nullable — Cloud auto-detects when unset.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"deploy_command": schema.StringAttribute{
				MarkdownDescription: "Deploy command run after build. Added in v0.4.0. Empty string is valid for static sites.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"uses_push_to_deploy": schema.BoolAttribute{
				MarkdownDescription: "When true, Cloud auto-deploys on every push to the env's branch. Added in v0.4.0.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"uses_deploy_hook": schema.BoolAttribute{
				MarkdownDescription: "When true, Cloud exposes a webhook deploy hook for this env. Added in v0.4.0.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"uses_octane": schema.BoolAttribute{
				MarkdownDescription: "When true, Cloud boots the app under Laravel Octane. False for static sites + traditional Laravel. Added in v0.4.0.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"uses_hibernation": schema.BoolAttribute{
				MarkdownDescription: "When true, Cloud hibernates the env after idle. Cost-saving for low-traffic envs. Added in v0.4.0.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"color": schema.StringAttribute{
				MarkdownDescription: "Visual identifier in Cloud dashboard — `green`, `orange`, `red`, `blue`, `purple`. Added in v0.4.0.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"vanity_domain": schema.StringAttribute{
				MarkdownDescription: "Cloud-generated `<app>-<env>.laravel.cloud` fallback hostname. Read-only. Added in v0.4.0.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp of creation.",
				Computed:            true,
			},
		},
	}
}

func (r *EnvironmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *EnvironmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan EnvironmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := api.CreateEnvironmentRequest{Name: plan.Name.ValueString()}
	if !plan.Branch.IsNull() && !plan.Branch.IsUnknown() {
		v := plan.Branch.ValueString()
		apiReq.Branch = &v
	}
	if !plan.InheritsID.IsNull() && !plan.InheritsID.IsUnknown() {
		v := plan.InheritsID.ValueString()
		apiReq.InheritsID = &v
	}
	if !plan.DatabaseSchemaID.IsNull() && !plan.DatabaseSchemaID.IsUnknown() {
		v := plan.DatabaseSchemaID.ValueString()
		apiReq.DatabaseSchemaID = &v
	}
	if !plan.CacheID.IsNull() && !plan.CacheID.IsUnknown() {
		v := plan.CacheID.ValueString()
		apiReq.CacheID = &v
	}
	if !plan.WebsocketApplicationID.IsNull() && !plan.WebsocketApplicationID.IsUnknown() {
		v := plan.WebsocketApplicationID.ValueString()
		apiReq.WebsocketApplicationID = &v
	}
	if !plan.NodeVersion.IsNull() && !plan.NodeVersion.IsUnknown() {
		v := plan.NodeVersion.ValueString()
		apiReq.NodeVersion = &v
	}
	if !plan.BuildCommand.IsNull() && !plan.BuildCommand.IsUnknown() {
		v := plan.BuildCommand.ValueString()
		apiReq.BuildCommand = &v
	}
	if !plan.DeployCommand.IsNull() && !plan.DeployCommand.IsUnknown() {
		v := plan.DeployCommand.ValueString()
		apiReq.DeployCommand = &v
	}
	if !plan.UsesPushToDeploy.IsNull() && !plan.UsesPushToDeploy.IsUnknown() {
		v := plan.UsesPushToDeploy.ValueBool()
		apiReq.UsesPushToDeploy = &v
	}
	if !plan.UsesDeployHook.IsNull() && !plan.UsesDeployHook.IsUnknown() {
		v := plan.UsesDeployHook.ValueBool()
		apiReq.UsesDeployHook = &v
	}
	if !plan.UsesOctane.IsNull() && !plan.UsesOctane.IsUnknown() {
		v := plan.UsesOctane.ValueBool()
		apiReq.UsesOctane = &v
	}
	if !plan.UsesHibernation.IsNull() && !plan.UsesHibernation.IsUnknown() {
		v := plan.UsesHibernation.ValueBool()
		apiReq.UsesHibernation = &v
	}
	if !plan.Color.IsNull() && !plan.Color.IsUnknown() {
		v := plan.Color.ValueString()
		apiReq.Color = &v
	}
	if !plan.Variables.IsNull() && !plan.Variables.IsUnknown() {
		vars := make(map[string]string, len(plan.Variables.Elements()))
		resp.Diagnostics.Append(plan.Variables.ElementsAs(ctx, &vars, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		apiReq.Variables = vars
	}

	env, err := r.client.CreateEnvironment(ctx, plan.ApplicationID.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create environment", err.Error())
		return
	}

	applyEnvToModel(ctx, env, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *EnvironmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state EnvironmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	env, err := r.client.GetEnvironment(ctx, state.ID.ValueString())
	if err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read environment", err.Error())
		return
	}

	applyEnvToModel(ctx, env, &state, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *EnvironmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan EnvironmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := api.UpdateEnvironmentRequest{}
	if !plan.Branch.IsNull() && !plan.Branch.IsUnknown() {
		v := plan.Branch.ValueString()
		apiReq.Branch = &v
	}
	if !plan.DatabaseSchemaID.IsNull() && !plan.DatabaseSchemaID.IsUnknown() {
		v := plan.DatabaseSchemaID.ValueString()
		apiReq.DatabaseSchemaID = &v
	}
	if !plan.CacheID.IsNull() && !plan.CacheID.IsUnknown() {
		v := plan.CacheID.ValueString()
		apiReq.CacheID = &v
	}
	if !plan.WebsocketApplicationID.IsNull() && !plan.WebsocketApplicationID.IsUnknown() {
		v := plan.WebsocketApplicationID.ValueString()
		apiReq.WebsocketApplicationID = &v
	}
	if !plan.NodeVersion.IsNull() && !plan.NodeVersion.IsUnknown() {
		v := plan.NodeVersion.ValueString()
		apiReq.NodeVersion = &v
	}
	if !plan.BuildCommand.IsNull() && !plan.BuildCommand.IsUnknown() {
		v := plan.BuildCommand.ValueString()
		apiReq.BuildCommand = &v
	}
	if !plan.DeployCommand.IsNull() && !plan.DeployCommand.IsUnknown() {
		v := plan.DeployCommand.ValueString()
		apiReq.DeployCommand = &v
	}
	if !plan.UsesPushToDeploy.IsNull() && !plan.UsesPushToDeploy.IsUnknown() {
		v := plan.UsesPushToDeploy.ValueBool()
		apiReq.UsesPushToDeploy = &v
	}
	if !plan.UsesDeployHook.IsNull() && !plan.UsesDeployHook.IsUnknown() {
		v := plan.UsesDeployHook.ValueBool()
		apiReq.UsesDeployHook = &v
	}
	if !plan.UsesOctane.IsNull() && !plan.UsesOctane.IsUnknown() {
		v := plan.UsesOctane.ValueBool()
		apiReq.UsesOctane = &v
	}
	if !plan.UsesHibernation.IsNull() && !plan.UsesHibernation.IsUnknown() {
		v := plan.UsesHibernation.ValueBool()
		apiReq.UsesHibernation = &v
	}
	if !plan.Color.IsNull() && !plan.Color.IsUnknown() {
		v := plan.Color.ValueString()
		apiReq.Color = &v
	}
	if !plan.Variables.IsNull() && !plan.Variables.IsUnknown() {
		vars := make(map[string]string, len(plan.Variables.Elements()))
		resp.Diagnostics.Append(plan.Variables.ElementsAs(ctx, &vars, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		apiReq.Variables = vars
	}

	env, err := r.client.UpdateEnvironment(ctx, plan.ID.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update environment", err.Error())
		return
	}

	applyEnvToModel(ctx, env, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *EnvironmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state EnvironmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteEnvironment(ctx, state.ID.ValueString()); err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError("Failed to delete environment", err.Error())
	}
}

func (r *EnvironmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// applyEnvToModel copies API DTO -> Terraform state model.
func applyEnvToModel(ctx context.Context, env *api.Environment, m *EnvironmentResourceModel, diags *diag.Diagnostics) {
	m.ID = types.StringValue(env.ID)
	m.ApplicationID = types.StringValue(env.ApplicationID)
	m.Name = types.StringValue(env.Name)

	setStringPtr(&m.Branch, env.Branch)
	setStringPtr(&m.InheritsID, env.InheritsID)
	setStringPtr(&m.DatabaseSchemaID, env.DatabaseSchemaID)
	setStringPtr(&m.CacheID, env.CacheID)
	setStringPtr(&m.WebsocketApplicationID, env.WebsocketApplicationID)
	setStringPtr(&m.NodeVersion, env.NodeVersion)
	setStringPtr(&m.BuildCommand, env.BuildCommand)
	setStringPtr(&m.DeployCommand, env.DeployCommand)
	setStringPtr(&m.Color, env.Color)
	setStringPtr(&m.VanityDomain, env.VanityDomain)
	setStringPtr(&m.CreatedAt, env.CreatedAt)

	setBoolPtr(&m.UsesPushToDeploy, env.UsesPushToDeploy)
	setBoolPtr(&m.UsesDeployHook, env.UsesDeployHook)
	setBoolPtr(&m.UsesOctane, env.UsesOctane)
	setBoolPtr(&m.UsesHibernation, env.UsesHibernation)

	if env.Variables != nil {
		vars, d := types.MapValueFrom(ctx, types.StringType, env.Variables)
		diags.Append(d...)
		m.Variables = vars
	} else {
		m.Variables = types.MapNull(types.StringType)
	}
}

// setStringPtr writes a nullable string API value into a state field.
// Central helper so every resource applies the same nil-check pattern.
func setStringPtr(dst *types.String, src *string) {
	if src != nil {
		*dst = types.StringValue(*src)
	} else {
		*dst = types.StringNull()
	}
}

// setBoolPtr writes a nullable bool API value into a state field.
func setBoolPtr(dst *types.Bool, src *bool) {
	if src != nil {
		*dst = types.BoolValue(*src)
	} else {
		*dst = types.BoolNull()
	}
}

var (
	_ resource.Resource                = (*EnvironmentResource)(nil)
	_ resource.ResourceWithImportState = (*EnvironmentResource)(nil)
	_ resource.ResourceWithConfigure   = (*EnvironmentResource)(nil)
)
