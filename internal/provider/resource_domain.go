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

	"github.com/stackra/terraform-provider-laravel-cloud/internal/api"
)

// DomainResource manages `laravelcloud_domain` — a custom hostname binding.
type DomainResource struct{ client *api.Client }

type DomainResourceModel struct {
	ID                types.String `tfsdk:"id"`
	EnvironmentID     types.String `tfsdk:"environment_id"`
	Name              types.String `tfsdk:"name"`
	RedirectFromWWW   types.Bool   `tfsdk:"redirect_from_www"`
	WildcardEnabled   types.Bool   `tfsdk:"wildcard_enabled"`
	CloudflareManaged types.Bool   `tfsdk:"cloudflare_managed"`
	Verification      types.String `tfsdk:"verification"`
	CreatedAt         types.String `tfsdk:"created_at"`
}

func NewDomainResource() resource.Resource { return &DomainResource{} }

func (r *DomainResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (r *DomainResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Custom hostname bound to a Cloud environment. Verification is `real_time` when using cloudflare_managed; otherwise operators verify a TXT record manually.",
		Attributes: map[string]schema.Attribute{
			"id":                 schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"environment_id":     schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"name":               schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, MarkdownDescription: "Fully-qualified hostname (immutable)."},
			"redirect_from_www":  schema.BoolAttribute{Optional: true, Computed: true},
			"wildcard_enabled":   schema.BoolAttribute{Optional: true, Computed: true},
			"cloudflare_managed": schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "When true, Cloud manages the DNS record via the Cloudflare integration."},
			"verification":       schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "`real_time` or `manual`."},
			"created_at":         schema.StringAttribute{Computed: true},
		},
	}
}

func (r *DomainResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *DomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DomainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	apiReq := api.CreateDomainRequest{Name: plan.Name.ValueString()}
	if !plan.RedirectFromWWW.IsNull() && !plan.RedirectFromWWW.IsUnknown() {
		apiReq.RedirectFromWWW = plan.RedirectFromWWW.ValueBool()
	}
	if !plan.WildcardEnabled.IsNull() && !plan.WildcardEnabled.IsUnknown() {
		apiReq.WildcardEnabled = plan.WildcardEnabled.ValueBool()
	}
	if !plan.CloudflareManaged.IsNull() && !plan.CloudflareManaged.IsUnknown() {
		apiReq.CloudflareManaged = plan.CloudflareManaged.ValueBool()
	}
	if !plan.Verification.IsNull() && !plan.Verification.IsUnknown() {
		apiReq.Verification = plan.Verification.ValueString()
	}
	domain, err := r.client.CreateDomain(ctx, plan.EnvironmentID.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create domain", err.Error())
		return
	}
	applyDomainToModel(domain, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DomainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	domain, err := r.client.GetDomain(ctx, state.ID.ValueString())
	if err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read domain", err.Error())
		return
	}
	applyDomainToModel(domain, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *DomainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan DomainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	apiReq := api.UpdateDomainRequest{}
	if !plan.RedirectFromWWW.IsNull() && !plan.RedirectFromWWW.IsUnknown() {
		v := plan.RedirectFromWWW.ValueBool()
		apiReq.RedirectFromWWW = &v
	}
	if !plan.WildcardEnabled.IsNull() && !plan.WildcardEnabled.IsUnknown() {
		v := plan.WildcardEnabled.ValueBool()
		apiReq.WildcardEnabled = &v
	}
	domain, err := r.client.UpdateDomain(ctx, plan.ID.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update domain", err.Error())
		return
	}
	applyDomainToModel(domain, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DomainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteDomain(ctx, state.ID.ValueString()); err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError("Failed to delete domain", err.Error())
	}
}

func (r *DomainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func applyDomainToModel(d *api.Domain, m *DomainResourceModel) {
	m.ID = types.StringValue(d.ID)
	m.EnvironmentID = types.StringValue(d.EnvironmentID)
	m.Name = types.StringValue(d.Name)
	m.RedirectFromWWW = types.BoolValue(d.RedirectFromWWW)
	m.WildcardEnabled = types.BoolValue(d.WildcardEnabled)
	m.CloudflareManaged = types.BoolValue(d.CloudflareManaged)
	m.Verification = types.StringValue(d.Verification)
	if d.CreatedAt != nil {
		m.CreatedAt = types.StringValue(*d.CreatedAt)
	} else {
		m.CreatedAt = types.StringNull()
	}
}

var (
	_ resource.Resource                = (*DomainResource)(nil)
	_ resource.ResourceWithImportState = (*DomainResource)(nil)
	_ resource.ResourceWithConfigure   = (*DomainResource)(nil)
)
