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

// DomainVerifyResource fires Cloud's DNS-verification pass on a domain.
// Cloud polls the domain's CNAME/A + moves it from "pending" to
// "verified" (or "failed"). Terraform authors this as a separate
// resource so callers can attach a `depends_on` chain: DNS record →
// domain → domain-verify. Trigger uses `verify_trigger` for re-verify.
//
// Read is a passthrough refresh of the domain's status; Delete is a
// no-op (verifying doesn't create anything to delete).
//
// Added in v0.5.0.
type DomainVerifyResource struct {
	client *api.Client
}

type DomainVerifyResourceModel struct {
	ID            types.String `tfsdk:"id"`
	DomainID      types.String `tfsdk:"domain_id"`
	VerifyTrigger types.String `tfsdk:"verify_trigger"`
	Status        types.String `tfsdk:"status"`
}

func NewDomainVerifyResource() resource.Resource {
	return &DomainVerifyResource{}
}

func (r *DomainVerifyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain_verify"
}

func (r *DomainVerifyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fires `POST /domains/:id/verify` — triggers Cloud's " +
			"DNS-verification pass on a domain. Attach a `depends_on` chain " +
			"from the terraform-managed DNS record so verification only fires " +
			"after DNS propagates. Bump `verify_trigger` to re-verify. Delete " +
			"is a no-op — verifying doesn't create anything to remove. Added " +
			"in v0.5.0.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"domain_id": schema.StringAttribute{
				MarkdownDescription: "Target domain. Immutable — a new domain means a new verify resource.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"verify_trigger": schema.StringAttribute{
				MarkdownDescription: "Bump this to re-run verification. Common pattern: " +
					"`timestamp()` in HCL, or a manual bump when DNS changes.",
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Post-verify domain status — `verified`, `pending`, or `failed`.",
				Computed:            true,
			},
		},
	}
}

func (r *DomainVerifyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *DomainVerifyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DomainVerifyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	domainID := plan.DomainID.ValueString()
	tflog.Info(ctx, "Triggering Cloud domain verify", map[string]any{"domain_id": domainID})

	d, err := r.client.VerifyDomain(ctx, domainID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to verify domain", err.Error())
		return
	}
	plan.ID = types.StringValue(domainID)
	if d.Status != nil {
		plan.Status = types.StringValue(*d.Status)
	} else {
		plan.Status = types.StringNull()
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DomainVerifyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DomainVerifyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	d, err := r.client.GetDomain(ctx, state.DomainID.ValueString())
	if err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to refresh domain status", err.Error())
		return
	}
	if d.Status != nil {
		state.Status = types.StringValue(*d.Status)
	} else {
		state.Status = types.StringNull()
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *DomainVerifyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Every field is RequiresReplace — Update never routes here.
	var plan DomainVerifyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DomainVerifyResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
	// No-op — verifying doesn't create anything to delete.
}

func (r *DomainVerifyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("domain_id"), req, resp)
}

var (
	_ resource.Resource                = (*DomainVerifyResource)(nil)
	_ resource.ResourceWithImportState = (*DomainVerifyResource)(nil)
	_ resource.ResourceWithConfigure   = (*DomainVerifyResource)(nil)
)
