package provider

import (
	"context"
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

// EnvironmentNetworkSettingsResource manages the network-security surface
// of an environment — `cache_strategy`, `response_headers_frame`,
// `response_headers_content_type`, `response_headers_robots_tag`,
// `response_headers_hsts { max_age, include_subdomains, preload }`,
// `firewall_rate_limit_level`, `firewall_under_attack_mode`.
//
// Separate resource because these fields have a distinct lifecycle from
// env-level compute + FK bindings — network hardening is authored per
// env-tier (dev = permissive, stg = mid, prd = strict) and often lands
// in a separate PR from the initial env scaffold. On the wire this hits
// `PATCH /environments/:id` with the network-only field subset, so
// there's no conflict with the sibling `laravelcloud_environment`
// resource — both PATCH the same endpoint with disjoint field sets.
//
// Delete PATCHes the env back to Cloud's defaults so state stays
// consistent. The env itself is NOT deleted.
//
// Added in v0.5.0.
type EnvironmentNetworkSettingsResource struct {
	client *api.Client
}

// HstsModel maps the nested `response_headers_hsts` block.
type HstsModel struct {
	MaxAge            types.Int64 `tfsdk:"max_age"`
	IncludeSubdomains types.Bool  `tfsdk:"include_subdomains"`
	Preload           types.Bool  `tfsdk:"preload"`
}

type EnvironmentNetworkSettingsResourceModel struct {
	ID            types.String `tfsdk:"id"`
	EnvironmentID types.String `tfsdk:"environment_id"`

	CacheStrategy              types.String `tfsdk:"cache_strategy"`
	ResponseHeadersFrame       types.String `tfsdk:"response_headers_frame"`
	ResponseHeadersContentType types.String `tfsdk:"response_headers_content_type"`
	ResponseHeadersRobotsTag   types.String `tfsdk:"response_headers_robots_tag"`
	ResponseHeadersHsts        types.Object `tfsdk:"response_headers_hsts"`
	FirewallRateLimitLevel     types.String `tfsdk:"firewall_rate_limit_level"`
	FirewallUnderAttackMode    types.Bool   `tfsdk:"firewall_under_attack_mode"`
}

func NewEnvironmentNetworkSettingsResource() resource.Resource {
	return &EnvironmentNetworkSettingsResource{}
}

func (r *EnvironmentNetworkSettingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment_network_settings"
}

// hstsAttrTypes returns the framework type shape for the `hsts` nested
// object — used for null construction + hydration.
func hstsAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"max_age":            types.Int64Type,
		"include_subdomains": types.BoolType,
		"preload":            types.BoolType,
	}
}

func (r *EnvironmentNetworkSettingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages network-security settings on a Laravel " +
			"Cloud environment — cache strategy, HTTP security response " +
			"headers, HSTS, rate-limit tier, and under-attack mode. Separate " +
			"resource from `laravelcloud_environment` so network hardening " +
			"has its own lifecycle. Delete PATCHes the env back to Cloud's " +
			"defaults; the environment itself is untouched. Added in v0.5.0.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Composite ID — matches `environment_id`.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"environment_id": schema.StringAttribute{
				MarkdownDescription: "Target environment. Immutable — a new env means a new " +
					"network-settings resource.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"cache_strategy": schema.StringAttribute{
				MarkdownDescription: "Edge cache strategy — one of `default`, `passthrough`, or " +
					"`custom` (see Cloud's CacheStrategy enum). Null keeps Cloud's env-tier default.",
				Optional: true,
				Computed: true,
			},
			"response_headers_frame": schema.StringAttribute{
				MarkdownDescription: "X-Frame-Options header value — `deny`, `sameorigin`, or `disabled`.",
				Optional:            true,
				Computed:            true,
			},
			"response_headers_content_type": schema.StringAttribute{
				MarkdownDescription: "X-Content-Type-Options header value — typically `nosniff` " +
					"or `disabled`.",
				Optional: true,
				Computed: true,
			},
			"response_headers_robots_tag": schema.StringAttribute{
				MarkdownDescription: "X-Robots-Tag header value — `all`, `noindex`, `noindex,nofollow`, " +
					"`none`, or `disabled`. Set `noindex` on dev/stg envs; `all` (or omit) on prd.",
				Optional: true,
				Computed: true,
			},
			"response_headers_hsts": schema.SingleNestedAttribute{
				MarkdownDescription: "HSTS (HTTP Strict-Transport-Security) response header " +
					"configuration. Null disables the header.",
				Optional: true,
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"max_age": schema.Int64Attribute{
						MarkdownDescription: "max-age directive in seconds. Common values: " +
							"`31536000` (1 year, mid-strict) or `63072000` (2 years, prd-strict).",
						Optional: true,
						Computed: true,
					},
					"include_subdomains": schema.BoolAttribute{
						MarkdownDescription: "Whether to apply HSTS to every subdomain.",
						Optional:            true,
						Computed:            true,
					},
					"preload": schema.BoolAttribute{
						MarkdownDescription: "Whether the domain should be submitted to the browser " +
							"HSTS preload list. Requires max_age >= 31536000 AND include_subdomains.",
						Optional: true,
						Computed: true,
					},
				},
			},
			"firewall_rate_limit_level": schema.StringAttribute{
				MarkdownDescription: "Cloud's rate-limit tier — `disabled`, `low`, `medium`, `high`. " +
					"Null keeps Cloud's env-tier default.",
				Optional: true,
				Computed: true,
			},
			"firewall_under_attack_mode": schema.BoolAttribute{
				MarkdownDescription: "Whether Cloud's under-attack mode is engaged. Emergency " +
					"traffic-scrubbing switch — leave false in normal operation.",
				Optional: true,
				Computed: true,
			},
		},
	}
}

func (r *EnvironmentNetworkSettingsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// hydrateHstsFromPlan turns the plan's `response_headers_hsts` object
// into the API-side pointer struct. Returns nil when the plan omitted
// the block (which means "leave HSTS at Cloud's default").
func hydrateHstsFromPlan(ctx context.Context, obj types.Object) (*api.HstsSettings, error) {
	if obj.IsNull() || obj.IsUnknown() {
		return nil, nil
	}
	var m HstsModel
	if diags := obj.As(ctx, &m, basetypes.ObjectAsOptions{UnhandledNullAsEmpty: true, UnhandledUnknownAsEmpty: true}); diags.HasError() {
		return nil, fmt.Errorf("decode hsts: %v", diags)
	}
	out := &api.HstsSettings{}
	if !m.MaxAge.IsNull() && !m.MaxAge.IsUnknown() {
		v := int(m.MaxAge.ValueInt64())
		out.MaxAge = &v
	}
	if !m.IncludeSubdomains.IsNull() && !m.IncludeSubdomains.IsUnknown() {
		v := m.IncludeSubdomains.ValueBool()
		out.IncludeSubdomains = &v
	}
	if !m.Preload.IsNull() && !m.Preload.IsUnknown() {
		v := m.Preload.ValueBool()
		out.Preload = &v
	}
	return out, nil
}

// buildPatchFromModel packs a plan model into the API request. Every
// pointer-nil field is omitted from the wire body.
func buildPatchFromModel(ctx context.Context, m *EnvironmentNetworkSettingsResourceModel) (api.UpdateEnvironmentNetworkSettingsRequest, error) {
	patch := api.UpdateEnvironmentNetworkSettingsRequest{}
	if !m.CacheStrategy.IsNull() && !m.CacheStrategy.IsUnknown() {
		v := m.CacheStrategy.ValueString()
		patch.CacheStrategy = &v
	}
	if !m.ResponseHeadersFrame.IsNull() && !m.ResponseHeadersFrame.IsUnknown() {
		v := m.ResponseHeadersFrame.ValueString()
		patch.ResponseHeadersFrame = &v
	}
	if !m.ResponseHeadersContentType.IsNull() && !m.ResponseHeadersContentType.IsUnknown() {
		v := m.ResponseHeadersContentType.ValueString()
		patch.ResponseHeadersContentType = &v
	}
	if !m.ResponseHeadersRobotsTag.IsNull() && !m.ResponseHeadersRobotsTag.IsUnknown() {
		v := m.ResponseHeadersRobotsTag.ValueString()
		patch.ResponseHeadersRobotsTag = &v
	}
	if !m.FirewallRateLimitLevel.IsNull() && !m.FirewallRateLimitLevel.IsUnknown() {
		v := m.FirewallRateLimitLevel.ValueString()
		patch.FirewallRateLimitLevel = &v
	}
	if !m.FirewallUnderAttackMode.IsNull() && !m.FirewallUnderAttackMode.IsUnknown() {
		v := m.FirewallUnderAttackMode.ValueBool()
		patch.FirewallUnderAttackMode = &v
	}
	hsts, err := hydrateHstsFromPlan(ctx, m.ResponseHeadersHsts)
	if err != nil {
		return patch, err
	}
	patch.ResponseHeadersHsts = hsts
	return patch, nil
}

func (r *EnvironmentNetworkSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan EnvironmentNetworkSettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	envID := plan.EnvironmentID.ValueString()
	patch, err := buildPatchFromModel(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("Failed to build patch body", err.Error())
		return
	}

	tflog.Info(ctx, "PATCHing environment network settings", map[string]any{
		"environment_id": envID,
	})

	if err := r.client.UpdateEnvironmentNetworkSettings(ctx, envID, patch); err != nil {
		resp.Diagnostics.AddError(
			"Failed to update environment network settings",
			"Cloud API returned an error while updating network settings. "+
				"Original error: "+err.Error(),
		)
		return
	}

	// Read back to hydrate computed values.
	settings, err := r.client.GetEnvironmentNetworkSettings(ctx, envID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read environment network settings after update", err.Error())
		return
	}

	plan.ID = types.StringValue(envID)
	applyNetworkSettingsToModel(settings, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *EnvironmentNetworkSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state EnvironmentNetworkSettingsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	settings, err := r.client.GetEnvironmentNetworkSettings(ctx, state.EnvironmentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read environment network settings", err.Error())
		return
	}
	applyNetworkSettingsToModel(settings, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *EnvironmentNetworkSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan EnvironmentNetworkSettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch, err := buildPatchFromModel(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("Failed to build patch body", err.Error())
		return
	}
	envID := plan.EnvironmentID.ValueString()
	if err := r.client.UpdateEnvironmentNetworkSettings(ctx, envID, patch); err != nil {
		resp.Diagnostics.AddError("Failed to update environment network settings", err.Error())
		return
	}
	settings, err := r.client.GetEnvironmentNetworkSettings(ctx, envID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read environment network settings after update", err.Error())
		return
	}
	plan.ID = types.StringValue(envID)
	applyNetworkSettingsToModel(settings, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete PATCHes the env back to permissive defaults. The environment
// itself is untouched — we just release Terraform's ownership of the
// network settings.
func (r *EnvironmentNetworkSettingsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state EnvironmentNetworkSettingsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Reset to Cloud's most permissive defaults.
	defaultStrategy := "default"
	frame := "disabled"
	contentType := "disabled"
	robots := "all"
	rateLimit := "disabled"
	underAttack := false

	patch := api.UpdateEnvironmentNetworkSettingsRequest{
		CacheStrategy:              &defaultStrategy,
		ResponseHeadersFrame:       &frame,
		ResponseHeadersContentType: &contentType,
		ResponseHeadersRobotsTag:   &robots,
		FirewallRateLimitLevel:     &rateLimit,
		FirewallUnderAttackMode:    &underAttack,
		// HSTS reset: nil pointer → Cloud drops the header.
		ResponseHeadersHsts: nil,
	}

	envID := state.EnvironmentID.ValueString()
	if err := r.client.UpdateEnvironmentNetworkSettings(ctx, envID, patch); err != nil {
		// Fail-soft — the env may already be gone, or Cloud may have
		// tightened defaults. Emit a warning rather than blocking the
		// destroy plan.
		resp.Diagnostics.AddWarning(
			"Failed to reset network settings on delete",
			"Cloud API returned an error while resetting the env back to "+
				"defaults. Original error: "+err.Error(),
		)
	}
}

func (r *EnvironmentNetworkSettingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import path is the environment_id itself.
	resource.ImportStatePassthroughID(ctx, path.Root("environment_id"), req, resp)
}

func applyNetworkSettingsToModel(s *api.EnvironmentNetworkSettings, model *EnvironmentNetworkSettingsResourceModel) {
	model.CacheStrategy = types.StringValue(s.CacheStrategy)
	model.ResponseHeadersFrame = types.StringValue(s.ResponseHeadersFrame)
	model.ResponseHeadersContentType = types.StringValue(s.ResponseHeadersContentType)
	model.ResponseHeadersRobotsTag = types.StringValue(s.ResponseHeadersRobotsTag)
	if s.FirewallRateLimitLevel != nil {
		model.FirewallRateLimitLevel = types.StringValue(*s.FirewallRateLimitLevel)
	} else {
		model.FirewallRateLimitLevel = types.StringNull()
	}
	model.FirewallUnderAttackMode = types.BoolValue(s.FirewallUnderAttackMode)

	// HSTS object hydration.
	if s.ResponseHeadersHsts == nil {
		model.ResponseHeadersHsts = types.ObjectNull(hstsAttrTypes())
	} else {
		vals := map[string]attr.Value{
			"max_age":            types.Int64Null(),
			"include_subdomains": types.BoolNull(),
			"preload":            types.BoolNull(),
		}
		if s.ResponseHeadersHsts.MaxAge != nil {
			vals["max_age"] = types.Int64Value(int64(*s.ResponseHeadersHsts.MaxAge))
		}
		if s.ResponseHeadersHsts.IncludeSubdomains != nil {
			vals["include_subdomains"] = types.BoolValue(*s.ResponseHeadersHsts.IncludeSubdomains)
		}
		if s.ResponseHeadersHsts.Preload != nil {
			vals["preload"] = types.BoolValue(*s.ResponseHeadersHsts.Preload)
		}
		obj, _ := types.ObjectValue(hstsAttrTypes(), vals)
		model.ResponseHeadersHsts = obj
	}
}

var (
	_ resource.Resource                = (*EnvironmentNetworkSettingsResource)(nil)
	_ resource.ResourceWithImportState = (*EnvironmentNetworkSettingsResource)(nil)
	_ resource.ResourceWithConfigure   = (*EnvironmentNetworkSettingsResource)(nil)
)
