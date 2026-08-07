package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/stackra/terraform-provider-laravel-cloud/internal/api"
)

// EnvironmentResource manages `laravelcloud_environment` — a per-app env
// (dev / stg / prd / preview-*) with branch binding + env-vars map.
type EnvironmentResource struct{ client *api.Client }

// EnvironmentResourceModel maps HCL <-> API DTO.
type EnvironmentResourceModel struct {
	ID            types.String `tfsdk:"id"`
	ApplicationID types.String `tfsdk:"application_id"`
	Name          types.String `tfsdk:"name"`
	Branch        types.String `tfsdk:"branch"`
	Variables     types.Map    `tfsdk:"variables"`
	InheritsID    types.String `tfsdk:"inherits_id"`
	CreatedAt     types.String `tfsdk:"created_at"`
}

// NewEnvironmentResource is the plugin-framework factory.
func NewEnvironmentResource() resource.Resource { return &EnvironmentResource{} }

func (r *EnvironmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment"
}

func (r *EnvironmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Cloud environment. One env per branch (`dev` → develop, `stg` → staging, `prd` → main). Env-vars are set as a map; inheritance links two envs so `stg` picks up `dev`'s vars.",
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
				MarkdownDescription: "Env slug — one of `dev`, `stg`, `prd`, or `preview-*`. Immutable — forces replace.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"branch": schema.StringAttribute{
				MarkdownDescription: "Git branch the env auto-deploys from. Nullable when using manual deploys.",
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

// applyEnvToModel copies API DTO -> Terraform state model. Diags are
// appended for any conversion errors so the caller can bail early.
func applyEnvToModel(ctx context.Context, env *api.Environment, m *EnvironmentResourceModel, diags *diag.Diagnostics) {
	m.ID = types.StringValue(env.ID)
	m.ApplicationID = types.StringValue(env.ApplicationID)
	m.Name = types.StringValue(env.Name)

	if env.Branch != nil {
		m.Branch = types.StringValue(*env.Branch)
	} else {
		m.Branch = types.StringNull()
	}
	if env.InheritsID != nil {
		m.InheritsID = types.StringValue(*env.InheritsID)
	} else {
		m.InheritsID = types.StringNull()
	}
	if env.CreatedAt != nil {
		m.CreatedAt = types.StringValue(*env.CreatedAt)
	} else {
		m.CreatedAt = types.StringNull()
	}

	if env.Variables != nil {
		vars, d := types.MapValueFrom(ctx, types.StringType, env.Variables)
		diags.Append(d...)
		m.Variables = vars
	} else {
		m.Variables = types.MapNull(types.StringType)
	}
}

var (
	_ resource.Resource                = (*EnvironmentResource)(nil)
	_ resource.ResourceWithImportState = (*EnvironmentResource)(nil)
	_ resource.ResourceWithConfigure   = (*EnvironmentResource)(nil)
)
