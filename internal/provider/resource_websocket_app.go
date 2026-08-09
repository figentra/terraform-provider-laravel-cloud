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

// WebsocketAppResource manages `laravelcloud_websocket_app` — binds an
// environment to a WS cluster with a per-env max_connections cap.
type WebsocketAppResource struct{ client *api.Client }

type WebsocketAppResourceModel struct {
	ID             types.String `tfsdk:"id"`
	ClusterID      types.String `tfsdk:"cluster_id"`
	Name           types.String `tfsdk:"name"`
	EnvironmentID  types.String `tfsdk:"environment_id"`
	MaxConnections types.Int64  `tfsdk:"max_connections"`
	AppKey         types.String `tfsdk:"app_key"`
	AppSecret      types.String `tfsdk:"app_secret"`
	CreatedAt      types.String `tfsdk:"created_at"`
}

func NewWebsocketAppResource() resource.Resource { return &WebsocketAppResource{} }

func (r *WebsocketAppResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_websocket_app"
}

func (r *WebsocketAppResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Binds one environment to a `laravelcloud_websocket_cluster` with a per-env max_connections cap.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"cluster_id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable WS app label. Added in v0.4.0.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"environment_id": schema.StringAttribute{
				MarkdownDescription: "Pre-v0.4 environment binding. v0.4 modules bind via `laravelcloud_environment.websocket_application_id` instead — this field remains Optional for backward compat.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"max_connections": schema.Int64Attribute{Optional: true, Computed: true, MarkdownDescription: "Per-env connection cap; falls back to cluster default when unset."},
			"app_key":         schema.StringAttribute{Computed: true, MarkdownDescription: "Reverb app key issued by Cloud. Read-only."},
			"app_secret":      schema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "Reverb app secret issued by Cloud. Read-only, sensitive."},
			"created_at":      schema.StringAttribute{Computed: true},
		},
	}
}

func (r *WebsocketAppResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *WebsocketAppResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan WebsocketAppResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	apiReq := api.CreateWebsocketAppRequest{}
	if !plan.EnvironmentID.IsNull() && !plan.EnvironmentID.IsUnknown() {
		apiReq.EnvironmentID = plan.EnvironmentID.ValueString()
	}
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		apiReq.Name = plan.Name.ValueString()
	}
	if !plan.MaxConnections.IsNull() && !plan.MaxConnections.IsUnknown() {
		apiReq.MaxConnections = int(plan.MaxConnections.ValueInt64())
	}
	app, err := r.client.CreateWebsocketApp(ctx, plan.ClusterID.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create websocket app", err.Error())
		return
	}
	applyWSAppToModel(app, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *WebsocketAppResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state WebsocketAppResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	app, err := r.client.GetWebsocketApp(ctx, state.ID.ValueString())
	if err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read websocket app", err.Error())
		return
	}
	applyWSAppToModel(app, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *WebsocketAppResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan WebsocketAppResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	apiReq := api.UpdateWebsocketAppRequest{}
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		v := plan.Name.ValueString()
		apiReq.Name = &v
	}
	if !plan.MaxConnections.IsNull() && !plan.MaxConnections.IsUnknown() {
		v := int(plan.MaxConnections.ValueInt64())
		apiReq.MaxConnections = &v
	}
	app, err := r.client.UpdateWebsocketApp(ctx, plan.ID.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update websocket app", err.Error())
		return
	}
	applyWSAppToModel(app, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *WebsocketAppResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state WebsocketAppResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteWebsocketApp(ctx, state.ID.ValueString()); err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError("Failed to delete websocket app", err.Error())
	}
}

func (r *WebsocketAppResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func applyWSAppToModel(a *api.WebsocketApp, m *WebsocketAppResourceModel) {
	m.ID = types.StringValue(a.ID)
	m.ClusterID = types.StringValue(a.ClusterID)
	if a.Name != "" {
		m.Name = types.StringValue(a.Name)
	} else {
		m.Name = types.StringNull()
	}
	if a.EnvironmentID != "" {
		m.EnvironmentID = types.StringValue(a.EnvironmentID)
	} else {
		m.EnvironmentID = types.StringNull()
	}
	if a.MaxConnections != nil {
		m.MaxConnections = types.Int64Value(int64(*a.MaxConnections))
	} else {
		m.MaxConnections = types.Int64Null()
	}
	setStringPtr(&m.AppKey, a.AppKey)
	setStringPtr(&m.AppSecret, a.AppSecret)
	setStringPtr(&m.CreatedAt, a.CreatedAt)
}

var (
	_ resource.Resource                = (*WebsocketAppResource)(nil)
	_ resource.ResourceWithImportState = (*WebsocketAppResource)(nil)
	_ resource.ResourceWithConfigure   = (*WebsocketAppResource)(nil)
)
