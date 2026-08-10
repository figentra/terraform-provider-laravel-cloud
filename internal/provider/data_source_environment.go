// Package provider — data-source implementation for
// `laravelcloud_environment`. Reads an existing Cloud environment by
// ID + hydrates every attribute the resource-side surfaces (application
// binding, branch, env-vars, database + cache + inherits linkages,
// timestamps).
//
// Consumers use this data source when composing Terraform modules that
// need to REFERENCE an environment created out-of-band (Cloud dashboard,
// a separate Terraform state, a legacy import). Read-only — no CRUD.
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/figentra/terraform-provider-laravel-cloud/internal/api"
)

// EnvironmentDataSource implements the Terraform Plugin Framework
// `datasource.DataSource` contract. The `*api.Client` field is
// populated once via `Configure` (shared client stashed on
// ProviderData by the provider's Configure step).
type EnvironmentDataSource struct {
	// client is the shared authenticated Cloud API wrapper.
	client *api.Client
}

// EnvironmentDataSourceModel mirrors the resource-side Environment
// model. Every field is Computed at the data-source layer; only `id`
// is Required so operators lookup by primary key.
//
// The `tfsdk` tags match the corresponding resource attributes 1:1 so
// generated docs read identically for both.
type EnvironmentDataSourceModel struct {
	// ID is the Cloud-assigned environment ULID. REQUIRED input.
	ID types.String `tfsdk:"id"`

	// ApplicationID is the owning application. Set by Cloud at create.
	ApplicationID types.String `tfsdk:"application_id"`

	// Name is the environment slug (development, staging, production,
	// preview-pr-<n>).
	Name types.String `tfsdk:"name"`

	// Branch is the source-control branch this env auto-deploys from.
	// Nullable when the env isn't source-controlled (preview envs).
	Branch types.String `tfsdk:"branch"`

	// Variables is the per-env env-var map. Every value round-trips
	// through Cloud's encryption-at-rest.
	Variables types.Map `tfsdk:"variables"`

	// DatabaseSchemaID is the DB schema bound to this env. Nil when
	// the env doesn't provision a database (marketing sites, etc.).
	DatabaseSchemaID types.String `tfsdk:"database_schema_id"`

	// CacheID is the cache bound to this env. Nil when unbound.
	CacheID types.String `tfsdk:"cache_id"`

	// InheritsID is the parent-env this env inherits variables from
	// (preview envs inherit from dev; stg inherits from dev, etc.).
	InheritsID types.String `tfsdk:"inherits_id"`

	// CreatedAt / UpdatedAt are RFC3339 timestamps stamped by Cloud.
	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`

	// VanityDomain is the Cloud-generated `<slug>-<env>-<random>.laravel.cloud`
	// hostname every env gets automatically. Downstream Terraform configs
	// point Cloudflare (or other DNS provider) CNAMEs at this value so
	// custom hostnames route to Cloud.
	VanityDomain types.String `tfsdk:"vanity_domain"`
}

// NewEnvironmentDataSource is the factory the provider registers in its
// DataSources() slice.
func NewEnvironmentDataSource() datasource.DataSource {
	return &EnvironmentDataSource{}
}

// Metadata sets the data-source type name. Callers reference it via
// `data.laravelcloud_environment.<name>`.
func (d *EnvironmentDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment"
}

// Schema declares the read-only surface — matches the resource shape
// with every field Computed except `id` which is Required.
func (d *EnvironmentDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads an existing Laravel Cloud environment by ID. " +
			"Useful when a Terraform module needs to reference an env created " +
			"in a separate state OR via the Cloud dashboard.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Cloud-assigned environment ID (ULID).",
				Required:            true,
			},
			"application_id": schema.StringAttribute{
				MarkdownDescription: "ID of the owning application.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Environment slug — one of `development`, " +
					"`staging`, `production`, or a `preview-pr-<n>` per-PR env.",
				Computed: true,
			},
			"branch": schema.StringAttribute{
				MarkdownDescription: "Source-control branch this environment " +
					"auto-deploys from. Null for non-source-controlled envs.",
				Computed: true,
			},
			"variables": schema.MapAttribute{
				MarkdownDescription: "Per-environment env-var map. Values are " +
					"encrypted at rest on the Cloud side. Sensitive values " +
					"should live in Doppler + be referenced via env-var " +
					"substitution rather than stored here in cleartext.",
				Computed:    true,
				ElementType: types.StringType,
				Sensitive:   true,
			},
			"database_schema_id": schema.StringAttribute{
				MarkdownDescription: "ID of the database schema bound to this env.",
				Computed:            true,
			},
			"cache_id": schema.StringAttribute{
				MarkdownDescription: "ID of the cache bound to this env.",
				Computed:            true,
			},
			"inherits_id": schema.StringAttribute{
				MarkdownDescription: "ID of a parent env this env inherits " +
					"variables from. Common patterns: staging inherits from " +
					"development, preview envs inherit from development.",
				Computed: true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp of environment creation.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp of last mutation.",
				Computed:            true,
			},
			"vanity_domain": schema.StringAttribute{
				MarkdownDescription: "Cloud-generated `<slug>-<env>-<random>." +
					"laravel.cloud` hostname. Point CNAMEs at this value to " +
					"route custom hostnames to the env.",
				Computed: true,
			},
		},
	}
}

// Configure pulls the shared client from ProviderData. Every data source
// gets the SAME client instance — thread-safe by construction.
func (d *EnvironmentDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// req.ProviderData is nil during framework-side validation phases.
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

// Read hydrates state from Cloud. Called once during `terraform plan`
// and once during `terraform apply` (data sources are re-read every plan
// to catch drift).
func (d *EnvironmentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Pull the config (the ID the operator supplied) into a typed model.
	var data EnvironmentDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Fire the read against Cloud. Any 404 or transient error surfaces
	// as a diagnostic — data sources don't have the "drift-tolerant"
	// escape hatch resources use.
	env, err := d.client.GetEnvironment(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to read environment",
			"Cloud API returned an error. Original error: "+err.Error(),
		)
		return
	}

	// Hydrate every field. Nullable API fields → typed null when unset.
	data.ApplicationID = types.StringValue(env.ApplicationID)
	data.Name = types.StringValue(env.Name)

	if env.Branch != nil {
		data.Branch = types.StringValue(*env.Branch)
	} else {
		data.Branch = types.StringNull()
	}

	// Variables map — convert the plain Go map into a Terraform types.Map.
	if len(env.Variables) > 0 {
		// The map element type is StringType per the Schema declaration.
		mapValue, mapDiags := types.MapValueFrom(ctx, types.StringType, env.Variables)
		resp.Diagnostics.Append(mapDiags...)
		if resp.Diagnostics.HasError() {
			return
		}
		data.Variables = mapValue
	} else {
		// Empty map is distinct from null — Cloud always returns an object,
		// even when empty. `nil` is a valid element map here (framework
		// treats it as an empty map with the declared element type).
		data.Variables = types.MapValueMust(types.StringType, nil)
	}

	if env.DatabaseSchemaID != nil {
		data.DatabaseSchemaID = types.StringValue(*env.DatabaseSchemaID)
	} else {
		data.DatabaseSchemaID = types.StringNull()
	}
	if env.CacheID != nil {
		data.CacheID = types.StringValue(*env.CacheID)
	} else {
		data.CacheID = types.StringNull()
	}
	if env.InheritsID != nil {
		data.InheritsID = types.StringValue(*env.InheritsID)
	} else {
		data.InheritsID = types.StringNull()
	}
	if env.CreatedAt != nil {
		data.CreatedAt = types.StringValue(*env.CreatedAt)
	} else {
		data.CreatedAt = types.StringNull()
	}
	if env.UpdatedAt != nil {
		data.UpdatedAt = types.StringValue(*env.UpdatedAt)
	} else {
		data.UpdatedAt = types.StringNull()
	}
	if env.VanityDomain != nil {
		data.VanityDomain = types.StringValue(*env.VanityDomain)
	} else {
		data.VanityDomain = types.StringNull()
	}

	// Persist the hydrated model into state.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Compile-time interface assertions — fail the build at authoring time
// if the data source ever loses a required method.
var (
	_ datasource.DataSource              = (*EnvironmentDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*EnvironmentDataSource)(nil)
)
