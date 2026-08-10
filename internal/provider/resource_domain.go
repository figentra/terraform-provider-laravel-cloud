package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/figentra/terraform-provider-laravel-cloud/internal/api"
)

// DomainResource manages `laravelcloud_domain` — a custom hostname binding.
//
// v0.4.0 attribute expansion:
//   - `www_redirect` — new canonical name for redirect_from_www (bool)
//   - `verification_method` — new canonical name for verification (string)
//   - `cloudflare_strategy` — new canonical name for cloudflare_managed
//     (was bool, now string enum: "cloudflare-managed", "manual", etc.)
//
// Both name families work; the provider aliases them onto the wire.
type DomainResource struct{ client *api.Client }

type DomainResourceModel struct {
	ID            types.String `tfsdk:"id"`
	EnvironmentID types.String `tfsdk:"environment_id"`
	Name          types.String `tfsdk:"name"`

	// Pre-v0.4 names
	RedirectFromWWW   types.Bool   `tfsdk:"redirect_from_www"`
	CloudflareManaged types.Bool   `tfsdk:"cloudflare_managed"`
	Verification      types.String `tfsdk:"verification"`

	// v0.4 canonical names. WWWRedirect is a STRING enum in v2 —
	// "www_to_root", "root_to_www", "none".
	WWWRedirect        types.String `tfsdk:"www_redirect"`
	CloudflareStrategy types.String `tfsdk:"cloudflare_strategy"`
	VerificationMethod types.String `tfsdk:"verification_method"`

	WildcardEnabled types.Bool   `tfsdk:"wildcard_enabled"`
	CreatedAt       types.String `tfsdk:"created_at"`
}

func NewDomainResource() resource.Resource { return &DomainResource{} }

func (r *DomainResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (r *DomainResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Custom hostname bound to a Cloud environment. v0.4.0 renames — use `www_redirect`, `verification_method`, `cloudflare_strategy`. Pre-v0.4 names (`redirect_from_www`, `verification`, `cloudflare_managed`) are kept for backward compatibility.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"environment_id": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				MarkdownDescription: "Fully-qualified hostname (immutable).",
			},
			"redirect_from_www": schema.BoolAttribute{
				MarkdownDescription: "**Deprecated in v0.4.0** — use `www_redirect`.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
				DeprecationMessage:  "Use `www_redirect` instead.",
			},
			"www_redirect": schema.StringAttribute{
				MarkdownDescription: "Redirect strategy — `www_to_root`, `root_to_www`, or `none`. Added in v0.4.0 (typed enum; was bool pre-v0.4 as `redirect_from_www`).",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"wildcard_enabled": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"cloudflare_managed": schema.BoolAttribute{
				MarkdownDescription: "**Deprecated in v0.4.0** — use `cloudflare_strategy`.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
				DeprecationMessage:  "Use `cloudflare_strategy` instead — v0.4.0 exposes a richer enum.",
			},
			"cloudflare_strategy": schema.StringAttribute{
				MarkdownDescription: "Cloudflare integration strategy — `cloudflare-managed`, `manual`, `origin`, or Cloud-defined. Added in v0.4.0.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"verification": schema.StringAttribute{
				MarkdownDescription: "**Deprecated in v0.4.0** — use `verification_method`.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				DeprecationMessage:  "Use `verification_method` instead.",
			},
			"verification_method": schema.StringAttribute{
				MarkdownDescription: "DNS verification method — `real_time`, `manual`, `dns-txt`, `http`. Added in v0.4.0.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"created_at": schema.StringAttribute{Computed: true},
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

	// www_redirect is a v2 string enum; redirect_from_www is a v1 bool.
	// Send whichever the caller set. Cloud accepts either family.
	if !plan.WWWRedirect.IsNull() && !plan.WWWRedirect.IsUnknown() {
		v := plan.WWWRedirect.ValueString()
		apiReq.WWWRedirect = &v
		apiReq.RedirectFromWWW = v == "www_to_root" || v == "root_to_www"
	} else if !plan.RedirectFromWWW.IsNull() && !plan.RedirectFromWWW.IsUnknown() {
		b := plan.RedirectFromWWW.ValueBool()
		apiReq.RedirectFromWWW = b
		if b {
			s := "www_to_root"
			apiReq.WWWRedirect = &s
		}
	}
	if !plan.WildcardEnabled.IsNull() && !plan.WildcardEnabled.IsUnknown() {
		apiReq.WildcardEnabled = plan.WildcardEnabled.ValueBool()
	}
	if !plan.CloudflareStrategy.IsNull() && !plan.CloudflareStrategy.IsUnknown() {
		v := plan.CloudflareStrategy.ValueString()
		apiReq.CloudflareStrategy = &v
		// If strategy is "cloudflare-managed" (or contains "cloudflare"), also
		// set the boolean flag for pre-v0.4 API compat.
		apiReq.CloudflareManaged = v != "manual" && v != "none"
	} else if !plan.CloudflareManaged.IsNull() && !plan.CloudflareManaged.IsUnknown() {
		apiReq.CloudflareManaged = plan.CloudflareManaged.ValueBool()
	}
	if !plan.VerificationMethod.IsNull() && !plan.VerificationMethod.IsUnknown() {
		v := plan.VerificationMethod.ValueString()
		apiReq.VerificationMethod = &v
		apiReq.Verification = v
	} else if !plan.Verification.IsNull() && !plan.Verification.IsUnknown() {
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
	if !plan.WWWRedirect.IsNull() && !plan.WWWRedirect.IsUnknown() {
		v := plan.WWWRedirect.ValueString()
		apiReq.WWWRedirect = &v
		b := v == "www_to_root" || v == "root_to_www"
		apiReq.RedirectFromWWW = &b
	} else if !plan.RedirectFromWWW.IsNull() && !plan.RedirectFromWWW.IsUnknown() {
		b := plan.RedirectFromWWW.ValueBool()
		apiReq.RedirectFromWWW = &b
		if b {
			s := "www_to_root"
			apiReq.WWWRedirect = &s
		}
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

// applyDomainToModel copies API DTO -> Terraform state model. Handles the
// pre-v0.4 <-> v0.4 name aliasing so consumers using either family see
// consistent state.
//
// Cloud's POST /environments/:id/domains response is sparse — every
// server-generated field (id, environment_id, verification, created_at)
// is present, but the caller-provided fields (www_redirect,
// cloudflare_strategy, verification_method) may come back as null or
// as different defaults. Terraform's post-apply consistency check rejects
// the diff; preserve plan values on nil/empty API responses.
func applyDomainToModel(d *api.Domain, m *DomainResourceModel) {
	m.ID = types.StringValue(d.ID)
	// EnvironmentID sometimes comes back as empty string on POST — preserve
	// plan value in that case (the plan set it from the parent env resource).
	if d.EnvironmentID != "" {
		m.EnvironmentID = types.StringValue(d.EnvironmentID)
	} else if m.EnvironmentID.IsNull() || m.EnvironmentID.IsUnknown() {
		m.EnvironmentID = types.StringNull()
	}
	if d.Name != "" {
		m.Name = types.StringValue(d.Name)
	}
	m.WildcardEnabled = types.BoolValue(d.WildcardEnabled)

	// www_redirect / redirect_from_www — alias. WWWRedirect is a v2
	// string enum; RedirectFromWWW is the v1 bool derived from it.
	// Cloud's Create response often returns "none" even when plan said
	// "www_to_root" — Cloud applies the setting async post-create.
	// Preserve plan value when set + non-default.
	planWWW := m.WWWRedirect
	if d.WWWRedirect != nil {
		apiVal := *d.WWWRedirect
		// If plan explicitly set a value + Cloud returned the default
		// ("none"), keep the plan value.
		if apiVal == "none" && !planWWW.IsNull() && !planWWW.IsUnknown() && planWWW.ValueString() != "none" {
			m.WWWRedirect = planWWW
			m.RedirectFromWWW = types.BoolValue(planWWW.ValueString() == "www_to_root" || planWWW.ValueString() == "root_to_www")
		} else {
			m.WWWRedirect = types.StringValue(apiVal)
			m.RedirectFromWWW = types.BoolValue(apiVal == "www_to_root" || apiVal == "root_to_www")
		}
	} else if d.RedirectFromWWW {
		m.WWWRedirect = types.StringValue("www_to_root")
		m.RedirectFromWWW = types.BoolValue(true)
	} else if !planWWW.IsNull() && !planWWW.IsUnknown() {
		// Preserve plan when API omits.
		m.WWWRedirect = planWWW
		v := planWWW.ValueString()
		m.RedirectFromWWW = types.BoolValue(v == "www_to_root" || v == "root_to_www")
	} else {
		m.WWWRedirect = types.StringValue("none")
		m.RedirectFromWWW = types.BoolValue(false)
	}

	// cloudflare_strategy / cloudflare_managed — alias.
	planCF := m.CloudflareStrategy
	if d.CloudflareStrategy != nil {
		m.CloudflareStrategy = types.StringValue(*d.CloudflareStrategy)
		m.CloudflareManaged = types.BoolValue(*d.CloudflareStrategy != "manual" && *d.CloudflareStrategy != "none")
	} else if d.CloudflareManaged {
		m.CloudflareStrategy = types.StringValue("cloudflare-managed")
		m.CloudflareManaged = types.BoolValue(true)
	} else if !planCF.IsNull() && !planCF.IsUnknown() {
		m.CloudflareStrategy = planCF
		v := planCF.ValueString()
		m.CloudflareManaged = types.BoolValue(v != "manual" && v != "none")
	} else {
		m.CloudflareStrategy = types.StringValue("manual")
		m.CloudflareManaged = types.BoolValue(false)
	}

	// verification_method / verification — alias. Cloud's response
	// omits both fields on Create; preserve plan value.
	planVM := m.VerificationMethod
	if d.VerificationMethod != nil {
		m.VerificationMethod = types.StringValue(*d.VerificationMethod)
		m.Verification = types.StringValue(*d.VerificationMethod)
	} else if d.Verification != "" {
		m.VerificationMethod = types.StringValue(d.Verification)
		m.Verification = types.StringValue(d.Verification)
	} else if !planVM.IsNull() && !planVM.IsUnknown() {
		m.VerificationMethod = planVM
		m.Verification = planVM
	} else {
		m.VerificationMethod = types.StringNull()
		m.Verification = types.StringNull()
	}

	if d.CreatedAt != nil {
		m.CreatedAt = types.StringValue(*d.CreatedAt)
	} else if m.CreatedAt.IsNull() || m.CreatedAt.IsUnknown() {
		m.CreatedAt = types.StringNull()
	}
}

var (
	_ resource.Resource                = (*DomainResource)(nil)
	_ resource.ResourceWithImportState = (*DomainResource)(nil)
	_ resource.ResourceWithConfigure   = (*DomainResource)(nil)
)
