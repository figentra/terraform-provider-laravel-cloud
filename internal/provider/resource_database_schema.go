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

// DatabaseSchemaResource manages a logical database (schema) inside a cluster.
// Schemas are immutable — no Update path (Cloud REST spec).
type DatabaseSchemaResource struct{ client *api.Client }

type DatabaseSchemaResourceModel struct {
	ID        types.String `tfsdk:"id"`
	ClusterID types.String `tfsdk:"cluster_id"`
	Name      types.String `tfsdk:"name"`
	CreatedAt types.String `tfsdk:"created_at"`
}

func NewDatabaseSchemaResource() resource.Resource { return &DatabaseSchemaResource{} }

func (r *DatabaseSchemaResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_database_schema"
}

func (r *DatabaseSchemaResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A logical database (schema) inside a `laravelcloud_database_cluster`. Immutable post-create — rename requires destroy+recreate.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"cluster_id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"name":       schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, MarkdownDescription: "Schema name. Must match `[a-z_][a-z0-9_]*` per Postgres/MySQL naming."},
			"created_at": schema.StringAttribute{Computed: true},
		},
	}
}

func (r *DatabaseSchemaResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *DatabaseSchemaResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DatabaseSchemaResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	schemaOut, err := r.client.CreateDatabaseSchema(ctx, plan.ClusterID.ValueString(),
		api.CreateDatabaseSchemaRequest{Name: plan.Name.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create database schema", err.Error())
		return
	}
	applyDBSchemaToModel(schemaOut, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DatabaseSchemaResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DatabaseSchemaResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	schemaOut, err := r.client.GetDatabaseSchema(ctx, state.ID.ValueString())
	if err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read database schema", err.Error())
		return
	}
	applyDBSchemaToModel(schemaOut, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *DatabaseSchemaResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Immutable — every field carries RequiresReplace. Should never fire.
	resp.Diagnostics.AddError("Update not supported", "database schemas are immutable; every attribute forces replace")
}

func (r *DatabaseSchemaResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DatabaseSchemaResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteDatabaseSchema(ctx, state.ID.ValueString()); err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError("Failed to delete database schema", err.Error())
	}
}

func (r *DatabaseSchemaResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func applyDBSchemaToModel(schemaOut *api.DatabaseSchema, m *DatabaseSchemaResourceModel) {
	m.ID = types.StringValue(schemaOut.ID)
	m.ClusterID = types.StringValue(schemaOut.ClusterID)
	m.Name = types.StringValue(schemaOut.Name)
	if schemaOut.CreatedAt != nil {
		m.CreatedAt = types.StringValue(*schemaOut.CreatedAt)
	} else {
		m.CreatedAt = types.StringNull()
	}
}

var (
	_ resource.Resource                = (*DatabaseSchemaResource)(nil)
	_ resource.ResourceWithImportState = (*DatabaseSchemaResource)(nil)
	_ resource.ResourceWithConfigure   = (*DatabaseSchemaResource)(nil)
)
