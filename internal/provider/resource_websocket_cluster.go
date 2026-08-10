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

	"github.com/figentra/terraform-provider-laravel-cloud/internal/api"
)

// WebsocketClusterResource manages `laravelcloud_websocket_cluster` —
// Reverb-compatible cluster hosting one WsApp per environment.
type WebsocketClusterResource struct{ client *api.Client }

type WebsocketClusterResourceModel struct {
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	Name           types.String `tfsdk:"name"`
	Region         types.String `tfsdk:"region"`
	Type           types.String `tfsdk:"type"`
	Size           types.String `tfsdk:"size"`
	MaxConnections types.Int64  `tfsdk:"max_connections"`
	CreatedAt      types.String `tfsdk:"created_at"`
}

func NewWebsocketClusterResource() resource.Resource { return &WebsocketClusterResource{} }

func (r *WebsocketClusterResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_websocket_cluster"
}

func (r *WebsocketClusterResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reverb-compatible WebSocket cluster hosting one WS app per environment.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "Owning organisation. Optional in v0.4.0 — Cloud infers from token when unset.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"region": schema.StringAttribute{
				MarkdownDescription: "Deploy region. Optional in v0.4.0 — Cloud derives from organisation.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Cluster type — `reverb` (Reverb-native). Added in v0.4.0. Immutable — forces replace.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"size": schema.StringAttribute{
				MarkdownDescription: "Cluster size — e.g. `ws.s-1vcpu-1gb`. Optional in v0.4.0.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"max_connections": schema.Int64Attribute{
				MarkdownDescription: "Global cap on concurrent connections.",
				Optional:            true,
				Computed:            true,
			},
			"created_at": schema.StringAttribute{Computed: true},
		},
	}
}

func (r *WebsocketClusterResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *WebsocketClusterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan WebsocketClusterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	apiReq := api.CreateWebsocketClusterRequest{Name: plan.Name.ValueString()}
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
	if !plan.Size.IsNull() && !plan.Size.IsUnknown() {
		apiReq.Size = plan.Size.ValueString()
	}
	if !plan.MaxConnections.IsNull() && !plan.MaxConnections.IsUnknown() {
		apiReq.MaxConnections = int(plan.MaxConnections.ValueInt64())
	}
	cluster, err := r.client.CreateWebsocketCluster(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create websocket cluster", err.Error())
		return
	}
	applyWSClusterToModel(cluster, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *WebsocketClusterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state WebsocketClusterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cluster, err := r.client.GetWebsocketCluster(ctx, state.ID.ValueString())
	if err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read websocket cluster", err.Error())
		return
	}
	applyWSClusterToModel(cluster, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *WebsocketClusterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan WebsocketClusterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	apiReq := api.UpdateWebsocketClusterRequest{}
	if !plan.Size.IsNull() && !plan.Size.IsUnknown() {
		v := plan.Size.ValueString()
		apiReq.Size = &v
	}
	if !plan.MaxConnections.IsNull() && !plan.MaxConnections.IsUnknown() {
		v := int(plan.MaxConnections.ValueInt64())
		apiReq.MaxConnections = &v
	}
	cluster, err := r.client.UpdateWebsocketCluster(ctx, plan.ID.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update websocket cluster", err.Error())
		return
	}
	applyWSClusterToModel(cluster, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *WebsocketClusterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state WebsocketClusterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteWebsocketCluster(ctx, state.ID.ValueString()); err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError("Failed to delete websocket cluster", err.Error())
	}
}

func (r *WebsocketClusterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func applyWSClusterToModel(c *api.WebsocketCluster, m *WebsocketClusterResourceModel) {
	m.ID = types.StringValue(c.ID)
	// Cloud omits organization_id + basic scalars in the POST response;
	// preserve the plan value / collapse Unknown → Null when API leaves
	// them empty (see resource_application.go for the fuller rationale).
	if c.OrganizationID != "" {
		m.OrganizationID = types.StringValue(c.OrganizationID)
	} else if m.OrganizationID.IsUnknown() {
		m.OrganizationID = types.StringNull()
	}
	if c.Name != "" {
		m.Name = types.StringValue(c.Name)
	}
	if c.Region != "" {
		m.Region = types.StringValue(c.Region)
	}
	setStringPtr(&m.Type, c.Type)
	if c.Size != "" {
		m.Size = types.StringValue(c.Size)
	} else {
		m.Size = types.StringNull()
	}
	m.MaxConnections = types.Int64Value(int64(c.MaxConnections))
	setStringPtr(&m.CreatedAt, c.CreatedAt)
}

var (
	_ resource.Resource                = (*WebsocketClusterResource)(nil)
	_ resource.ResourceWithImportState = (*WebsocketClusterResource)(nil)
	_ resource.ResourceWithConfigure   = (*WebsocketClusterResource)(nil)
)
