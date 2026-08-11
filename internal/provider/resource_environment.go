package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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

	Color types.String `tfsdk:"color"`

	// PHP major version — one of "8.2", "8.3", "8.4", "8.5". Added in v0.7.0.
	// Cloud's read field is `php_major_version` (plain "8.4"); write field
	// is `php_version` (with `:1` suffix). Encoding happens in
	// Create/Update; the model surfaces only the plain "8.4" shape.
	PhpMajorVersion types.String `tfsdk:"php_major_version"`

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
				MarkdownDescription: "Visual identifier in the Cloud dashboard chip. One of: " +
					"`gray`, `slate`, `zinc`, `red`, `rose`, `orange`, `amber`, `yellow`, " +
					"`lime`, `green`, `emerald`, `teal`, `cyan`, `sky`, `blue`, `indigo`, " +
					"`violet`, `purple`, `fuchsia`, `pink`. Cloud picks a default when unset. " +
					"Validated enum added in v0.6.0.\n\n" +
					"**Known limitation (v0.6.0+):** Cloud's PATCH endpoint " +
					"silently accepts this field but does NOT persist or " +
					"return it — the dashboard color picker uses a separate " +
					"undocumented endpoint. Every apply is best-effort; " +
					"visible drift is expected until the vendor exposes the " +
					"read side.",
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.OneOf(api.ValidColors...),
				},
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"php_major_version": schema.StringAttribute{
				MarkdownDescription: "PHP major version pinned for this environment — one of " +
					"`8.2`, `8.3`, `8.4`, `8.5`. Added in v0.7.0. Cloud defaults " +
					"to the latest available (`8.5` today) when unset; pinning " +
					"unblocks apps whose composer.json constraint excludes the " +
					"newest.\n\n" +
					"**Cloud API contract quirk (documented in v0.7.0):** the " +
					"read field is `php_major_version` (plain `\"8.4\"`); the " +
					"write field is `php_version` with a mandatory `:1` suffix " +
					"(`\"8.4:1\"`). The provider handles the encode / decode " +
					"internally — the HCL surface uses only the plain `\"8.4\"` " +
					"shape.\n\n" +
					"**Create-then-update:** Cloud's POST " +
					"`/applications/:id/environments` endpoint ignores " +
					"`php_version` — it accepts only `name` / `branch` / " +
					"`cluster_id`. When set on Create, the provider fires a " +
					"post-create PATCH to apply the pin, then re-reads.",
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.OneOf("8.2", "8.3", "8.4", "8.5"),
				},
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
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

	// Post-create PATCH — Cloud's POST endpoint accepts only a narrow
	// slice of fields (name / branch / cluster_id per the vendor SDK's
	// CreateEnvironmentData). Anything the operator sets that requires
	// a PATCH goes through here immediately after the env exists.
	//
	// Today this covers php_major_version — Cloud silently drops it on
	// POST but honours it on PATCH. Extend the block when we find
	// another field with the same asymmetry.
	if !plan.PhpMajorVersion.IsNull() && !plan.PhpMajorVersion.IsUnknown() {
		phpWrite := plan.PhpMajorVersion.ValueString() + ":1"
		updated, uerr := r.client.UpdateEnvironment(ctx, env.ID, api.UpdateEnvironmentRequest{
			PhpVersion: &phpWrite,
		})
		if uerr != nil {
			resp.Diagnostics.AddError(
				"Failed to pin php_major_version on new environment",
				"Environment was created, but the post-create PATCH to set php_version failed. "+
					"The env now exists at its Cloud-default PHP version and will need a "+
					"terraform apply retry OR a manual dashboard fix.\n\nUnderlying error: "+uerr.Error(),
			)
			return
		}
		env = updated
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
	if !plan.PhpMajorVersion.IsNull() && !plan.PhpMajorVersion.IsUnknown() {
		// Encode plain "8.4" → Cloud's write-side "8.4:1" shape. The
		// suffix is mandatory — see the PHP-VERSION CONTRACT ASYMMETRY
		// comment on api.UpdateEnvironmentRequest.
		phpWrite := plan.PhpMajorVersion.ValueString() + ":1"
		apiReq.PhpVersion = &phpWrite
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
	// ID + Name always come from the API — Cloud is the source of
	// truth for these (ID is server-generated; Name is echoed back).
	m.ID = types.StringValue(env.ID)
	if env.ApplicationID != "" {
		m.ApplicationID = types.StringValue(env.ApplicationID)
	}
	if env.Name != "" {
		m.Name = types.StringValue(env.Name)
	}

	// Cloud's POST /environments response is intentionally sparse — many
	// fields we send in the create body come back as null. Terraform's
	// post-apply consistency check compares plan (user-set) vs state
	// (this model). If plan had a known value + Cloud returned null,
	// blindly setting null produces "was X, but now null" errors.
	//
	// Contract for every Optional+Computed field:
	//   - Cloud returned a value → use it (overrides plan).
	//   - Cloud returned nil + plan had a value → preserve the plan
	//     value. A subsequent Read surfaces genuine drift.
	//   - Cloud returned nil + plan had no value → keep null.
	//
	// The helper handles the three-way ternary in one place.
	preserveOrAssign(&m.Branch, env.Branch)
	preserveOrAssign(&m.InheritsID, env.InheritsID)
	preserveOrAssign(&m.DatabaseSchemaID, env.DatabaseSchemaID)
	preserveOrAssign(&m.CacheID, env.CacheID)
	preserveOrAssign(&m.WebsocketApplicationID, env.WebsocketApplicationID)
	preserveOrAssign(&m.Color, env.Color)
	preserveOrAssign(&m.PhpMajorVersion, env.PhpMajorVersion)
	preserveOrAssign(&m.VanityDomain, env.VanityDomain)
	preserveOrAssign(&m.CreatedAt, env.CreatedAt)
	preserveOrAssign(&m.NodeVersion, env.NodeVersion)
	preserveOrAssign(&m.BuildCommand, env.BuildCommand)
	preserveOrAssign(&m.DeployCommand, env.DeployCommand)

	preserveOrAssignBool(&m.UsesPushToDeploy, env.UsesPushToDeploy)
	preserveOrAssignBool(&m.UsesDeployHook, env.UsesDeployHook)
	preserveOrAssignBool(&m.UsesOctane, env.UsesOctane)
	preserveOrAssignBool(&m.UsesHibernation, env.UsesHibernation)

	// Variables: preserve plan map when Cloud returns nil.
	if env.Variables != nil {
		vars, d := types.MapValueFrom(ctx, types.StringType, env.Variables)
		diags.Append(d...)
		m.Variables = vars
	} else if m.Variables.IsUnknown() || m.Variables.IsNull() {
		m.Variables = types.MapNull(types.StringType)
	}
}

// preserveOrAssign writes the API value into the destination ONLY when
// the destination is currently Null or Unknown. If the plan set a
// concrete value, it wins over the API response — the caller's HCL is
// the source of truth for Optional+Computed attributes at Create time.
// A subsequent Read refreshes from Cloud + surfaces any drift.
func preserveOrAssign(dst *types.String, src *string) {
	if !dst.IsNull() && !dst.IsUnknown() {
		return // plan set a value, keep it
	}
	if src != nil {
		*dst = types.StringValue(*src)
	} else {
		*dst = types.StringNull()
	}
}

// preserveOrAssignBool mirrors preserveOrAssign for bool fields.
func preserveOrAssignBool(dst *types.Bool, src *bool) {
	if !dst.IsNull() && !dst.IsUnknown() {
		return
	}
	if src != nil {
		*dst = types.BoolValue(*src)
	} else {
		*dst = types.BoolNull()
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
