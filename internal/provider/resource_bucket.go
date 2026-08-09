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
//
// v0.4.0 attribute expansion mirrors Cloud's v2 API:
//   - `visibility` — new canonical name for the private/public flag.
//     `mode` remains supported for backward compatibility.
//   - `jurisdiction` — geographic zone slug (eu/us/ap).
//   - `key_name` + `key_permission` — Cloud-mints an access key
//     alongside each bucket; these attributes surface the key's name
//     + permission set to the consumer.
type BucketResource struct{ client *api.Client }

// BucketResourceModel is the Terraform-facing state shape.
type BucketResourceModel struct {
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	Name           types.String `tfsdk:"name"`
	Region         types.String `tfsdk:"region"`

	// Mode is the pre-v0.4.0 access flag. Consumers should prefer
	// Visibility; Mode is kept for backward compatibility.
	Mode types.String `tfsdk:"mode"`

	// Visibility is the v0.4.0 canonical access flag — "private" | "public".
	Visibility types.String `tfsdk:"visibility"`

	// Jurisdiction is the geographic zone slug.
	Jurisdiction types.String `tfsdk:"jurisdiction"`

	// KeyName is the identifier of the auto-generated access key.
	KeyName types.String `tfsdk:"key_name"`

	// KeyPermission is the permission level — "read_write"|"read"|"write".
	KeyPermission types.String `tfsdk:"key_permission"`

	Status    types.String `tfsdk:"status"`
	CreatedAt types.String `tfsdk:"created_at"`
}

// NewBucketResource is the plugin-framework factory.
func NewBucketResource() resource.Resource { return &BucketResource{} }

func (r *BucketResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bucket"
}

func (r *BucketResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "S3-compatible object storage bucket. As of v0.4.0, " +
			"`visibility` supersedes `mode` (kept for backward compatibility) and " +
			"three new attributes surface the Cloud-generated access key + " +
			"jurisdiction. Either the legacy `mode` OR the new `visibility` may " +
			"be set — the provider aliases both onto Cloud's v2 API shape.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Cloud-assigned bucket ID (ULID).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "Owning Cloud organisation. Immutable — forces replace.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Globally unique bucket name.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "Deploy region. Optional — Cloud derives from the organisation when omitted.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"mode": schema.StringAttribute{
				MarkdownDescription: "**Deprecated in v0.4.0** — use `visibility`. `private` | `public`.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				DeprecationMessage:  "Use `visibility` instead — same semantics, matches Cloud's v2 API shape.",
			},
			"visibility": schema.StringAttribute{
				MarkdownDescription: "Access model — `private` (default) or `public`. Introduced in v0.4.0.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"jurisdiction": schema.StringAttribute{
				MarkdownDescription: "Geographic zone slug — `eu`, `us`, `ap`, or a Cloud-defined value. Distinct from `region`. Immutable — forces replace.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"key_name": schema.StringAttribute{
				MarkdownDescription: "Identifier of the auto-generated access key Cloud mints alongside the bucket. Immutable — forces replace.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"key_permission": schema.StringAttribute{
				MarkdownDescription: "Permission level of the generated key — `read_write` (default), `read`, or `write`.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Cloud lifecycle status. Read-only.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp of bucket creation.",
				Computed:            true,
			},
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

	apiReq := api.CreateBucketRequest{
		Name: plan.Name.ValueString(),
	}
	if !plan.OrganizationID.IsNull() && !plan.OrganizationID.IsUnknown() {
		apiReq.OrganizationID = plan.OrganizationID.ValueString()
	}
	if !plan.Region.IsNull() && !plan.Region.IsUnknown() {
		apiReq.Region = plan.Region.ValueString()
	}
	// Prefer Visibility when set; fall back to Mode. Cloud accepts either.
	if !plan.Visibility.IsNull() && !plan.Visibility.IsUnknown() {
		apiReq.Visibility = plan.Visibility.ValueString()
	} else if !plan.Mode.IsNull() && !plan.Mode.IsUnknown() {
		apiReq.Mode = plan.Mode.ValueString()
	}
	if !plan.Jurisdiction.IsNull() && !plan.Jurisdiction.IsUnknown() {
		v := plan.Jurisdiction.ValueString()
		apiReq.Jurisdiction = &v
	}
	if !plan.KeyName.IsNull() && !plan.KeyName.IsUnknown() {
		v := plan.KeyName.ValueString()
		apiReq.KeyName = &v
	}
	if !plan.KeyPermission.IsNull() && !plan.KeyPermission.IsUnknown() {
		v := plan.KeyPermission.ValueString()
		apiReq.KeyPermission = &v
	}

	bucket, err := r.client.CreateBucket(ctx, apiReq)
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
	if !plan.Visibility.IsNull() && !plan.Visibility.IsUnknown() {
		v := plan.Visibility.ValueString()
		apiReq.Visibility = &v
	} else if !plan.Mode.IsNull() && !plan.Mode.IsUnknown() {
		v := plan.Mode.ValueString()
		apiReq.Mode = &v
	}
	if !plan.KeyPermission.IsNull() && !plan.KeyPermission.IsUnknown() {
		v := plan.KeyPermission.ValueString()
		apiReq.KeyPermission = &v
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

// applyBucketToModel copies Cloud's API response into the Terraform state.
// Handles the mode↔visibility aliasing so callers using either shape see
// consistent state.
func applyBucketToModel(bucket *api.Bucket, m *BucketResourceModel) {
	m.ID = types.StringValue(bucket.ID)
	m.OrganizationID = types.StringValue(bucket.OrganizationID)
	m.Name = types.StringValue(bucket.Name)
	m.Region = types.StringValue(bucket.Region)

	// Alias mode↔visibility. Cloud may respond with either, depending on
	// API version. Always populate BOTH fields on the state model so
	// consumers reading either see a value.
	visibility := bucket.Visibility
	if visibility == "" {
		visibility = bucket.Mode
	}
	mode := bucket.Mode
	if mode == "" {
		mode = bucket.Visibility
	}
	if visibility != "" {
		m.Visibility = types.StringValue(visibility)
	} else {
		m.Visibility = types.StringNull()
	}
	if mode != "" {
		m.Mode = types.StringValue(mode)
	} else {
		m.Mode = types.StringNull()
	}

	if bucket.Jurisdiction != nil {
		m.Jurisdiction = types.StringValue(*bucket.Jurisdiction)
	} else {
		m.Jurisdiction = types.StringNull()
	}
	if bucket.KeyName != nil {
		m.KeyName = types.StringValue(*bucket.KeyName)
	} else {
		m.KeyName = types.StringNull()
	}
	if bucket.KeyPermission != nil {
		m.KeyPermission = types.StringValue(*bucket.KeyPermission)
	} else {
		m.KeyPermission = types.StringNull()
	}
	if bucket.Status != nil {
		m.Status = types.StringValue(*bucket.Status)
	} else {
		m.Status = types.StringNull()
	}
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
