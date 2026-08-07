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

// CacheResource manages `laravelcloud_cache` — a Valkey/Redis cache instance.
type CacheResource struct{ client *api.Client }

type CacheResourceModel struct {
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	Name           types.String `tfsdk:"name"`
	Region         types.String `tfsdk:"region"`
	Size           types.String `tfsdk:"size"`
	CreatedAt      types.String `tfsdk:"created_at"`
}

func NewCacheResource() resource.Resource { return &CacheResource{} }

func (r *CacheResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cache"
}

func (r *CacheResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Valkey/Redis cache instance. Bind to environments via each env's `cache_id`.",
		Attributes: map[string]schema.Attribute{
			"id":              schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"organization_id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"name":            schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"region":          schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"size":            schema.StringAttribute{Required: true, MarkdownDescription: "Cache size — e.g. `valkey-pro.1gb`, `valkey-pro.5gb`."},
			"created_at":      schema.StringAttribute{Computed: true},
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
	cache, err := r.client.CreateCache(ctx, api.CreateCacheRequest{
		OrganizationID: plan.OrganizationID.ValueString(),
		Name:           plan.Name.ValueString(),
		Region:         plan.Region.ValueString(),
		Size:           plan.Size.ValueString(),
	})
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
	if cache.CreatedAt != nil {
		m.CreatedAt = types.StringValue(*cache.CreatedAt)
	} else {
		m.CreatedAt = types.StringNull()
	}
}

var (
	_ resource.Resource                = (*CacheResource)(nil)
	_ resource.ResourceWithImportState = (*CacheResource)(nil)
	_ resource.ResourceWithConfigure   = (*CacheResource)(nil)
)
