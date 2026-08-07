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

// BucketResource manages `laravelcloud_bucket` — S3-compatible object storage.
type BucketResource struct{ client *api.Client }

type BucketResourceModel struct {
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	Name           types.String `tfsdk:"name"`
	Region         types.String `tfsdk:"region"`
	Mode           types.String `tfsdk:"mode"`
	CreatedAt      types.String `tfsdk:"created_at"`
}

func NewBucketResource() resource.Resource { return &BucketResource{} }

func (r *BucketResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bucket"
}

func (r *BucketResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "S3-compatible object storage bucket. Mode toggles between `private` (default) and `public`.",
		Attributes: map[string]schema.Attribute{
			"id":              schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"organization_id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"name":            schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, MarkdownDescription: "Globally unique bucket name."},
			"region":          schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"mode":            schema.StringAttribute{Required: true, MarkdownDescription: "`private` or `public`."},
			"created_at":      schema.StringAttribute{Computed: true},
		},
	}
}

func (r *BucketResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *BucketResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan BucketResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	bucket, err := r.client.CreateBucket(ctx, api.CreateBucketRequest{
		OrganizationID: plan.OrganizationID.ValueString(),
		Name:           plan.Name.ValueString(),
		Region:         plan.Region.ValueString(),
		Mode:           plan.Mode.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create bucket", err.Error())
		return
	}
	applyBucketToModel(bucket, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *BucketResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state BucketResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	bucket, err := r.client.GetBucket(ctx, state.ID.ValueString())
	if err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read bucket", err.Error())
		return
	}
	applyBucketToModel(bucket, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *BucketResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan BucketResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	apiReq := api.UpdateBucketRequest{}
	if !plan.Mode.IsNull() && !plan.Mode.IsUnknown() {
		v := plan.Mode.ValueString()
		apiReq.Mode = &v
	}
	bucket, err := r.client.UpdateBucket(ctx, plan.ID.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update bucket", err.Error())
		return
	}
	applyBucketToModel(bucket, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *BucketResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state BucketResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteBucket(ctx, state.ID.ValueString()); err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError("Failed to delete bucket", err.Error())
	}
}

func (r *BucketResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func applyBucketToModel(bucket *api.Bucket, m *BucketResourceModel) {
	m.ID = types.StringValue(bucket.ID)
	m.OrganizationID = types.StringValue(bucket.OrganizationID)
	m.Name = types.StringValue(bucket.Name)
	m.Region = types.StringValue(bucket.Region)
	m.Mode = types.StringValue(bucket.Mode)
	if bucket.CreatedAt != nil {
		m.CreatedAt = types.StringValue(*bucket.CreatedAt)
	} else {
		m.CreatedAt = types.StringNull()
	}
}

var (
	_ resource.Resource                = (*BucketResource)(nil)
	_ resource.ResourceWithImportState = (*BucketResource)(nil)
	_ resource.ResourceWithConfigure   = (*BucketResource)(nil)
)
