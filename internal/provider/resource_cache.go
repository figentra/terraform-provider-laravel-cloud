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

// CacheResource manages `laravelcloud_cache` — a Valkey/Redis cache instance.
//
// v0.4.0 attribute expansion:
//   - `type` — engine selector (valkey/redis).
//   - `auto_upgrade_enabled` — bool; Cloud-managed engine upgrades.
//   - `is_public` — bool; exposes the cache to non-Cloud clients.
//   - `eviction_policy` — string; Valkey/Redis eviction policy.
type CacheResource struct{ client *api.Client }

type CacheResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	OrganizationID     types.String `tfsdk:"organization_id"`
	Name               types.String `tfsdk:"name"`
	Region             types.String `tfsdk:"region"`
	Size               types.String `tfsdk:"size"`
	Type               types.String `tfsdk:"type"`
	AutoUpgradeEnabled types.Bool   `tfsdk:"auto_upgrade_enabled"`
	IsPublic           types.Bool   `tfsdk:"is_public"`
	EvictionPolicy     types.String `tfsdk:"eviction_policy"`
	CreatedAt          types.String `tfsdk:"created_at"`
}

func NewCacheResource() resource.Resource { return &CacheResource{} }

func (r *CacheResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cache"
}

func (r *CacheResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Valkey/Redis cache instance. Bind to environments via each env's `cache_id`. v0.4.0 exposes every Cloud dashboard tuning knob (type, auto-upgrade, is_public, eviction policy).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "Owning organisation. Optional in v0.4.0 — Cloud infers from token when unset.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "Deploy region. Optional in v0.4.0 — Cloud derives from organisation when unset.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"size": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Cache size — e.g. `valkey-pro.1gb`, `valkey-pro.5gb`.",
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Engine selector — `valkey` (default) or `redis`. Added in v0.4.0. Immutable — forces replace.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"auto_upgrade_enabled": schema.BoolAttribute{
				MarkdownDescription: "When true, Cloud auto-upgrades the cache engine on new releases. Added in v0.4.0.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"is_public": schema.BoolAttribute{
				MarkdownDescription: "When true, the cache accepts connections from non-Cloud clients. Added in v0.4.0.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"eviction_policy": schema.StringAttribute{
				MarkdownDescription: "Valkey/Redis eviction policy — `allkeys-lru` (default), `volatile-lru`, `noeviction`, `allkeys-lfu`, `volatile-lfu`. Added in v0.4.0.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"created_at": schema.StringAttribute{Computed: true},
		},
	}
}

func (r *CacheResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *CacheResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan CacheResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	apiReq := api.CreateCacheRequest{
		Name: plan.Name.ValueString(),
		Size: plan.Size.ValueString(),
	}
	if !plan.OrganizationID.IsNull() && !plan.OrganizationID.IsUnknown() {
		apiReq.OrganizationID = plan.OrganizationID.ValueString()
	}
	if !plan.Region.IsNull() && !plan.Region.IsUnknown() {
		apiReq.Region = plan.Region.ValueString()
	}
	if !plan.Type.IsNull() && !plan.Type.IsUnknown() {
		v := plan.Type.ValueString()
		apiReq.Type = &v
	}
	if !plan.AutoUpgradeEnabled.IsNull() && !plan.AutoUpgradeEnabled.IsUnknown() {
		v := plan.AutoUpgradeEnabled.ValueBool()
		apiReq.AutoUpgradeEnabled = &v
	}
	if !plan.IsPublic.IsNull() && !plan.IsPublic.IsUnknown() {
		v := plan.IsPublic.ValueBool()
		apiReq.IsPublic = &v
	}
	if !plan.EvictionPolicy.IsNull() && !plan.EvictionPolicy.IsUnknown() {
		v := plan.EvictionPolicy.ValueString()
		apiReq.EvictionPolicy = &v
	}
	cache, err := r.client.CreateCache(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create cache", err.Error())
		return
	}
	applyCacheToModel(cache, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *CacheResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state CacheResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cache, err := r.client.GetCache(ctx, state.ID.ValueString())
	if err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read cache", err.Error())
		return
	}
	applyCacheToModel(cache, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *CacheResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan CacheResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	apiReq := api.UpdateCacheRequest{}
	if !plan.Size.IsNull() && !plan.Size.IsUnknown() {
		v := plan.Size.ValueString()
		apiReq.Size = &v
	}
	if !plan.AutoUpgradeEnabled.IsNull() && !plan.AutoUpgradeEnabled.IsUnknown() {
		v := plan.AutoUpgradeEnabled.ValueBool()
		apiReq.AutoUpgradeEnabled = &v
	}
	if !plan.IsPublic.IsNull() && !plan.IsPublic.IsUnknown() {
		v := plan.IsPublic.ValueBool()
		apiReq.IsPublic = &v
	}
	if !plan.EvictionPolicy.IsNull() && !plan.EvictionPolicy.IsUnknown() {
		v := plan.EvictionPolicy.ValueString()
		apiReq.EvictionPolicy = &v
	}
	cache, err := r.client.UpdateCache(ctx, plan.ID.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update cache", err.Error())
		return
	}
	applyCacheToModel(cache, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *CacheResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state CacheResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteCache(ctx, state.ID.ValueString()); err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError("Failed to delete cache", err.Error())
	}
}

func (r *CacheResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func applyCacheToModel(cache *api.Cache, m *CacheResourceModel) {
	m.ID = types.StringValue(cache.ID)
	m.OrganizationID = types.StringValue(cache.OrganizationID)
	m.Name = types.StringValue(cache.Name)
	m.Region = types.StringValue(cache.Region)
	m.Size = types.StringValue(cache.Size)
	setStringPtr(&m.Type, cache.Type)
	setBoolPtr(&m.AutoUpgradeEnabled, cache.AutoUpgradeEnabled)
	setBoolPtr(&m.IsPublic, cache.IsPublic)
	setStringPtr(&m.EvictionPolicy, cache.EvictionPolicy)
	setStringPtr(&m.CreatedAt, cache.CreatedAt)
}

var (
	_ resource.Resource                = (*CacheResource)(nil)
	_ resource.ResourceWithImportState = (*CacheResource)(nil)
	_ resource.ResourceWithConfigure   = (*CacheResource)(nil)
)
