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

// WebsocketClusterResource manages `laravelcloud_websocket_cluster` —
// Reverb-compatible cluster hosting one WsApp per environment.
type WebsocketClusterResource struct{ client *api.Client }

type WebsocketClusterResourceModel struct {
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	Name           types.String `tfsdk:"name"`
	Region         types.String `tfsdk:"region"`
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
			"id":              schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"organization_id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"name":            schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"region":          schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"size":            schema.StringAttribute{Required: true, MarkdownDescription: "Cluster size — e.g. `ws.s-1vcpu-1gb`, `ws.m-4vcpu-4gb`."},
			"max_connections": schema.Int64Attribute{Required: true, MarkdownDescription: "Global cap on concurrent connections."},
			"created_at":      schema.StringAttribute{Computed: true},
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
	cluster, err := r.client.CreateWebsocketCluster(ctx, api.CreateWebsocketClusterRequest{
		OrganizationID: plan.OrganizationID.ValueString(),
		Name:           plan.Name.ValueString(),
		Region:         plan.Region.ValueString(),
		Size:           plan.Size.ValueString(),
		MaxConnections: int(plan.MaxConnections.ValueInt64()),
	})
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
	m.OrganizationID = types.StringValue(c.OrganizationID)
	m.Name = types.StringValue(c.Name)
	m.Region = types.StringValue(c.Region)
	m.Size = types.StringValue(c.Size)
	m.MaxConnections = types.Int64Value(int64(c.MaxConnections))
	if c.CreatedAt != nil {
		m.CreatedAt = types.StringValue(*c.CreatedAt)
	} else {
		m.CreatedAt = types.StringNull()
	}
}

var (
	_ resource.Resource                = (*WebsocketClusterResource)(nil)
	_ resource.ResourceWithImportState = (*WebsocketClusterResource)(nil)
	_ resource.ResourceWithConfigure   = (*WebsocketClusterResource)(nil)
)
