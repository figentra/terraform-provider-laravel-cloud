package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/figentra/terraform-provider-laravel-cloud/internal/api"
)

// ApplicationDataSource reads an existing Cloud application by ID.
//
// Use case: an env-root module composes a service (via
// `laravel-cloud-service` module) but needs to reference an application
// created by another Terraform state OR by the Cloud dashboard. The
// data source hydrates read-only state without importing.
//
// Note: look-up by (org_slug, name) is deferred to Phase 2 — the Cloud
// API doesn't yet expose a name-scoped GET; we'd need to page through
// `GET /organizations/:slug/applications` + filter client-side. Not
// worth the complexity in v0.1.0.
type ApplicationDataSource struct {
	client *api.Client
}

// ApplicationDataSourceModel matches the same shape as the resource
// model — every field is Computed so operators only supply `id`.
type ApplicationDataSourceModel struct {
	ID                        types.String `tfsdk:"id"`
	OrganizationID            types.String `tfsdk:"organization_id"`
	Name                      types.String `tfsdk:"name"`
	Slug                      types.String `tfsdk:"slug"`
	Region                    types.String `tfsdk:"region"`
	SourceControlProviderType types.String `tfsdk:"source_control_provider_type"`
	Repository                types.String `tfsdk:"repository"`
	RootDirectory             types.String `tfsdk:"root_directory"`
	ClusterID                 types.String `tfsdk:"cluster_id"`
	SlackChannel              types.String `tfsdk:"slack_channel"`
	AvatarURL                 types.String `tfsdk:"avatar_url"`
	CreatedAt                 types.String `tfsdk:"created_at"`
}

// NewApplicationDataSource is the plugin-framework factory registered
// from `provider.DataSources()`.
func NewApplicationDataSource() datasource.DataSource {
	return &ApplicationDataSource{}
}

// Metadata sets the data-source type name — `laravelcloud_application`.
func (d *ApplicationDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application"
}

// Schema declares every attribute as Computed except `id` which is Required.
// Data sources are read-only — no CRUD, no plan modifiers.
func (d *ApplicationDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads an existing Laravel Cloud application by ID. " +
			"Useful when composing modules that reference apps created in a " +
			"separate Terraform state OR via the Cloud dashboard.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Cloud-assigned application ID (ULID).",
				Required:            true,
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "Organisation this application belongs to.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable application name.",
				Computed:            true,
			},
			"slug": schema.StringAttribute{
				MarkdownDescription: "URL-safe slug.",
				Computed:            true,
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "Deploy region.",
				Computed:            true,
			},
			"source_control_provider_type": schema.StringAttribute{
				MarkdownDescription: "Source control provider — `github`, `gitlab`, " +
					"or `bitbucket`.",
				Computed: true,
			},
			"repository": schema.StringAttribute{
				MarkdownDescription: "Repository identifier in `owner/repo` shape.",
				Computed:            true,
			},
			"root_directory": schema.StringAttribute{
				MarkdownDescription: "Sub-directory inside the repo Cloud treats as the build root. Added in v0.4.0.",
				Computed:            true,
			},
			"cluster_id": schema.StringAttribute{
				MarkdownDescription: "Deploy cluster ID.",
				Computed:            true,
			},
			"slack_channel": schema.StringAttribute{
				MarkdownDescription: "Slack channel for deploy notifications.",
				Computed:            true,
			},
			"avatar_url": schema.StringAttribute{
				MarkdownDescription: "URL of the Cloud-generated avatar.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp of application creation.",
				Computed:            true,
			},
		},
	}
}

// Configure pulls the shared client from ProviderData.
func (d *ApplicationDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

// Read hydrates the data source from Cloud.
func (d *ApplicationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ApplicationDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	app, err := d.client.GetApplication(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to read application",
			"Cloud API returned an error. Original error: "+err.Error(),
		)
		return
	}

	// Hydrate every field — data sources always populate the full model.
	data.Name = types.StringValue(app.Name)
	data.Slug = types.StringValue(app.Slug)
	data.Region = types.StringValue(app.Region)
	data.SourceControlProviderType = types.StringValue(app.SourceControlProviderType)

	if app.Organization != nil {
		data.OrganizationID = types.StringValue(app.Organization.ID)
	}
	if app.Repository != nil {
		data.Repository = types.StringValue(*app.Repository)
	} else {
		data.Repository = types.StringNull()
	}
	if app.RootDirectory != nil {
		data.RootDirectory = types.StringValue(*app.RootDirectory)
	} else {
		data.RootDirectory = types.StringNull()
	}
	if app.ClusterID != nil {
		data.ClusterID = types.StringValue(*app.ClusterID)
	} else {
		data.ClusterID = types.StringNull()
	}
	if app.SlackChannel != nil {
		data.SlackChannel = types.StringValue(*app.SlackChannel)
	} else {
		data.SlackChannel = types.StringNull()
	}
	if app.AvatarURL != nil {
		data.AvatarURL = types.StringValue(*app.AvatarURL)
	} else {
		data.AvatarURL = types.StringNull()
	}
	if app.CreatedAt != nil {
		data.CreatedAt = types.StringValue(app.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	} else {
		data.CreatedAt = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Compile-time interface assertions.
var (
	_ datasource.DataSource              = (*ApplicationDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*ApplicationDataSource)(nil)
)
