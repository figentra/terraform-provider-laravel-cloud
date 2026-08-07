// Package provider — data-source implementation for
// `laravelcloud_organization`. Reads a Cloud organisation by slug (or by
// falling back to the token-scoped default when neither `id` nor `slug`
// is set).
//
// Two lookup modes:
//
//  1. `data "laravelcloud_organization" "figentra" { slug = "figentra" }`
//     → resolves the org by slug via `GET /organizations/figentra`.
//
//  2. `data "laravelcloud_organization" "current" {}`
//     → falls back to `GET /meta/organization` for the token-scoped org.
//     Useful when the token was generated for a specific org and the
//     caller doesn't want to hard-code the slug in HCL.
//
// Consumers use this to hydrate the `organization_id` field on
// `laravelcloud_application` and every other resource that takes an
// organisation binding, without having to know the ULID up front.
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/figentra/terraform-provider-laravel-cloud/internal/api"
)

// OrganizationDataSource implements the plugin-framework `DataSource`
// contract.
type OrganizationDataSource struct {
	// client is the shared authenticated Cloud API wrapper.
	client *api.Client
}

// OrganizationDataSourceModel mirrors the resource-side model. Both `slug`
// and `id` are Optional; the ID field is also Computed so the framework
// hydrates it after a slug-only lookup.
type OrganizationDataSourceModel struct {
	// ID is the Cloud-assigned ULID. Optional input; when set the lookup
	// goes through `GET /organizations/:id`. Otherwise the framework
	// hydrates this field from the response.
	ID types.String `tfsdk:"id"`

	// Slug is the URL-safe organisation identifier. Optional input; when
	// set the lookup goes through `GET /organizations/:slug`. When neither
	// `id` nor `slug` is set, the data source falls back to the
	// token-scoped default (`GET /meta/organization`).
	Slug types.String `tfsdk:"slug"`

	// Name is the human-readable organisation name — hydrated from the API.
	Name types.String `tfsdk:"name"`
}

// NewOrganizationDataSource is the factory the provider registers.
func NewOrganizationDataSource() datasource.DataSource {
	return &OrganizationDataSource{}
}

// Metadata sets the data-source type name.
func (d *OrganizationDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization"
}

// Schema declares the read-only surface with both id + slug Optional.
func (d *OrganizationDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a Laravel Cloud organisation by slug or ID. " +
			"When both are omitted, falls back to the token-scoped default " +
			"organisation via `GET /meta/organization` — useful when the API " +
			"token was generated for a specific org and the caller doesn't " +
			"want to hard-code the slug in HCL.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Cloud-assigned organisation ID (ULID). " +
					"Optional — when set, performs a `GET /organizations/:id` " +
					"lookup. When both `id` and `slug` are omitted, the data " +
					"source resolves the token-scoped default organisation.",
				Optional: true,
				Computed: true,
			},
			"slug": schema.StringAttribute{
				MarkdownDescription: "URL-safe organisation slug (e.g. `figentra`, " +
					"`academorix`). Optional — when set, performs a slug-based " +
					"lookup. When both `id` and `slug` are omitted, resolves the " +
					"token-scoped default organisation.",
				Optional: true,
				Computed: true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable organisation name.",
				Computed:            true,
			},
		},
	}
}

// Configure pulls the shared client from ProviderData.
func (d *OrganizationDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*api.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected DataSource Configure Type",
			fmt.Sprintf("Expected *api.Client, got: %T. Please report this issue.",
				req.ProviderData),
		)
		return
	}

	d.client = client
}

// Read resolves the organisation via one of three paths:
//  1. Both id + slug set → same as slug-only (id takes precedence for the API call).
//  2. id-only → `GET /organizations/:id`.
//  3. slug-only → `GET /organizations/:slug`.
//  4. Neither set → `GET /meta/organization` (token-scoped default).
func (d *OrganizationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OrganizationDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Resolve which lookup path to take. ID takes precedence over slug
	// when both are set — the API accepts either at
	// `/organizations/:slugOrId` so this is a natural tiebreaker.
	var (
		org *api.Organization
		err error
	)
	switch {
	case !data.ID.IsNull() && !data.ID.IsUnknown() && data.ID.ValueString() != "":
		org, err = d.client.GetOrganization(ctx, data.ID.ValueString())
	case !data.Slug.IsNull() && !data.Slug.IsUnknown() && data.Slug.ValueString() != "":
		org, err = d.client.GetOrganization(ctx, data.Slug.ValueString())
	default:
		// Neither id nor slug — fall back to the token-scoped default.
		org, err = d.client.GetDefaultOrganization(ctx)
	}

	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to read organization",
			"Cloud API returned an error. Original error: "+err.Error(),
		)
		return
	}

	// Hydrate every field from the response. ID + Slug + Name are all
	// non-nullable per the API contract.
	data.ID = types.StringValue(org.ID)
	data.Slug = types.StringValue(org.Slug)
	data.Name = types.StringValue(org.Name)

	// Persist to state.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Compile-time interface assertions.
var (
	_ datasource.DataSource              = (*OrganizationDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*OrganizationDataSource)(nil)
)
