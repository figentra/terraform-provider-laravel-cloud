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
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/figentra/terraform-provider-laravel-cloud/internal/api"
)

// ApplicationResource manages a `laravelcloud_application` resource — the
// top-level Cloud deploy unit. One resource maps to one Cloud application.
type ApplicationResource struct {
	client *api.Client
}

// ApplicationResourceModel is the typed struct Terraform state hydrates
// on Read + writes on Create/Update. Field names match the schema
// attribute names via `tfsdk` tags.
type ApplicationResourceModel struct {
	ID                        types.String `tfsdk:"id"`
	OrganizationID            types.String `tfsdk:"organization_id"`
	Name                      types.String `tfsdk:"name"`
	Slug                      types.String `tfsdk:"slug"`
	Region                    types.String `tfsdk:"region"`
	SourceControlProviderType types.String `tfsdk:"source_control_provider_type"`
	Repository                types.String `tfsdk:"repository"`
	ClusterID                 types.String `tfsdk:"cluster_id"`
	SlackChannel              types.String `tfsdk:"slack_channel"`
	AvatarURL                 types.String `tfsdk:"avatar_url"`
	CreatedAt                 types.String `tfsdk:"created_at"`
}

// NewApplicationResource is the plugin-framework factory registered from
// `provider.Resources()`. Called once per resource block in the plan.
func NewApplicationResource() resource.Resource {
	return &ApplicationResource{}
}

// Metadata sets the resource type name — `laravelcloud_application`
// (the provider's TypeName prefix + this suffix).
func (r *ApplicationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application"
}

// Schema declares the resource attributes. Every attribute carries
// MarkdownDescription so tfplugindocs generates readable Registry docs.
//
// Design notes:
//   - `id` — computed, Cloud-assigned ULID.
//   - `organization_id` + `region` + `source_control_provider_type` are
//     immutable post-create; the `RequiresReplace` plan modifier forces
//     destroy+create when they change.
//   - `slug` is computed from `name` server-side; Terraform reads it
//     back but never writes it.
//   - `repository` + `cluster_id` + `slack_channel` are nullable +
//     mutable — pointer semantics on the API-side, `types.String` with
//     IsNull() checks on the Terraform side.
//   - `avatar_url` + `created_at` are computed metadata — read-only.
func (r *ApplicationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Laravel Cloud application. One resource " +
			"maps to one Cloud application — the top-level deploy unit under an " +
			"organisation. Applications own environments, database bindings, cache " +
			"bindings, WebSocket app bindings, and domain bindings.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Cloud-assigned application ID (ULID).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "The organisation this application belongs to. " +
					"Immutable post-create — changing this forces a replace.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable application name. Displayed in " +
					"the Cloud dashboard + used to derive the slug.",
				Required: true,
			},
			"slug": schema.StringAttribute{
				MarkdownDescription: "URL-safe slug derived from `name` by Cloud. " +
					"Read-only.",
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "Deploy region. Immutable post-create — changing " +
					"this forces a replace. Common values: `us-east-1`, `eu-west-1`, " +
					"`ap-southeast-1`. See Cloud docs for the current list.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"source_control_provider_type": schema.StringAttribute{
				MarkdownDescription: "Source control provider — `github`, `gitlab`, " +
					"or `bitbucket`. Immutable post-create.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"repository": schema.StringAttribute{
				MarkdownDescription: "Repository identifier in `owner/repo` shape. " +
					"Nullable — leave unset for a manually-deployed application.",
				Optional: true,
			},
			"cluster_id": schema.StringAttribute{
				MarkdownDescription: "Deploy cluster ID. Nullable — Cloud picks a " +
					"default when unset.",
				Optional: true,
				Computed: true,
			},
			"slack_channel": schema.StringAttribute{
				MarkdownDescription: "Slack channel for deploy notifications. " +
					"Nullable.",
				Optional: true,
			},
			"avatar_url": schema.StringAttribute{
				MarkdownDescription: "URL of the Cloud-generated avatar. Read-only.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp of application creation. " +
					"Read-only.",
				Computed: true,
			},
		},
	}
}

// Configure pulls the shared `*api.Client` from ProviderData. Called once
// per resource instance, right after New*Resource() returns.
func (r *ApplicationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		// Provider not yet configured — plugin-framework calls Configure
		// on every resource before Configure on the provider itself
		// completes; the second-pass call carries the client.
		return
	}

	client, ok := req.ProviderData.(*api.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *api.Client, got: %T. Please report this issue.",
				req.ProviderData),
		)
		return
	}

	r.client = client
}

// Create POSTs a new application to Cloud, then reads state back into
// the Terraform-facing model.
func (r *ApplicationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ApplicationResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Creating Laravel Cloud application", map[string]any{
		"name":            plan.Name.ValueString(),
		"organization_id": plan.OrganizationID.ValueString(),
	})

	apiReq := api.CreateApplicationRequest{
		OrganizationID:            plan.OrganizationID.ValueString(),
		Name:                      plan.Name.ValueString(),
		Region:                    plan.Region.ValueString(),
		SourceControlProviderType: plan.SourceControlProviderType.ValueString(),
	}

	if !plan.Repository.IsNull() && !plan.Repository.IsUnknown() {
		v := plan.Repository.ValueString()
		apiReq.Repository = &v
	}
	if !plan.ClusterID.IsNull() && !plan.ClusterID.IsUnknown() {
		v := plan.ClusterID.ValueString()
		apiReq.ClusterID = &v
	}
	if !plan.SlackChannel.IsNull() && !plan.SlackChannel.IsUnknown() {
		v := plan.SlackChannel.ValueString()
		apiReq.SlackChannel = &v
	}

	app, err := r.client.CreateApplication(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to create application",
			"Cloud API returned an error while creating the application. "+
				"Original error: "+err.Error(),
		)
		return
	}

	// Hydrate plan with computed values.
	applyAPIToModel(app, &plan)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read hydrates state from Cloud. Called during `terraform plan` +
// `refresh`. If the application 404s (drift), the resource is dropped
// from state without a diagnostic — Terraform's canonical drift-tolerant
// behaviour.
func (r *ApplicationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ApplicationResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	app, err := r.client.GetApplication(ctx, state.ID.ValueString())
	if err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			// Resource was deleted out-of-band. Drop from state.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Failed to read application",
			"Cloud API returned an error while reading the application. "+
				"Original error: "+err.Error(),
		)
		return
	}

	applyAPIToModel(app, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update PATCHes mutable fields (name, repository, slack channel).
// Immutable fields (organization_id, region, source_control_provider_type)
// carry `RequiresReplace` plan modifiers so Terraform destroys + recreates
// instead of routing here.
func (r *ApplicationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ApplicationResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Updating Laravel Cloud application", map[string]any{
		"id": plan.ID.ValueString(),
	})

	apiReq := api.UpdateApplicationRequest{}

	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		v := plan.Name.ValueString()
		apiReq.Name = &v
	}
	if !plan.Repository.IsNull() && !plan.Repository.IsUnknown() {
		v := plan.Repository.ValueString()
		apiReq.Repository = &v
	}
	if !plan.SlackChannel.IsNull() && !plan.SlackChannel.IsUnknown() {
		v := plan.SlackChannel.ValueString()
		apiReq.SlackChannel = &v
	}

	app, err := r.client.UpdateApplication(ctx, plan.ID.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to update application",
			"Cloud API returned an error while updating the application. "+
				"Original error: "+err.Error(),
		)
		return
	}

	applyAPIToModel(app, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete tears down the Cloud application. Terraform's dependency graph
// destroys child resources (environments, database bindings, etc.) first
// — matches Cloud's own delete constraint (409 when children exist).
func (r *ApplicationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ApplicationResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Deleting Laravel Cloud application", map[string]any{
		"id": state.ID.ValueString(),
	})

	if err := r.client.DeleteApplication(ctx, state.ID.ValueString()); err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			// Already gone — that's the desired end state, no error.
			return
		}
		resp.Diagnostics.AddError(
			"Failed to delete application",
			"Cloud API returned an error while deleting the application. "+
				"Original error: "+err.Error(),
		)
		return
	}
}

// ImportState hydrates the resource by ID — `terraform import
// laravelcloud_application.foo <cloud-id>`. The subsequent Read() populates
// the rest of state.
func (r *ApplicationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// applyAPIToModel copies API DTO fields into the Terraform-facing model.
// Called from Create, Read, Update — every path lands here to keep the
// mapping in one place.
func applyAPIToModel(app *api.Application, model *ApplicationResourceModel) {
	model.ID = types.StringValue(app.ID)
	model.Name = types.StringValue(app.Name)
	model.Slug = types.StringValue(app.Slug)
	model.Region = types.StringValue(app.Region)
	model.SourceControlProviderType = types.StringValue(app.SourceControlProviderType)

	if app.Organization != nil {
		model.OrganizationID = types.StringValue(app.Organization.ID)
	}

	if app.Repository != nil {
		model.Repository = types.StringValue(*app.Repository)
	} else {
		model.Repository = types.StringNull()
	}

	if app.ClusterID != nil {
		model.ClusterID = types.StringValue(*app.ClusterID)
	} else {
		model.ClusterID = types.StringNull()
	}

	if app.SlackChannel != nil {
		model.SlackChannel = types.StringValue(*app.SlackChannel)
	} else {
		model.SlackChannel = types.StringNull()
	}

	if app.AvatarURL != nil {
		model.AvatarURL = types.StringValue(*app.AvatarURL)
	} else {
		model.AvatarURL = types.StringNull()
	}

	if app.CreatedAt != nil {
		model.CreatedAt = types.StringValue(app.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	} else {
		model.CreatedAt = types.StringNull()
	}
}

// Compile-time interface assertions.
var (
	_ resource.Resource                = (*ApplicationResource)(nil)
	_ resource.ResourceWithImportState = (*ApplicationResource)(nil)
	_ resource.ResourceWithConfigure   = (*ApplicationResource)(nil)
)
