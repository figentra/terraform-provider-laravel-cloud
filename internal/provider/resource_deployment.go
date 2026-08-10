package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/figentra/terraform-provider-laravel-cloud/internal/api"
)

// DeploymentResource manages a `laravelcloud_deployment` resource — one
// deploy against one environment. Terraform authors this to fire the
// FIRST deploy for a fresh env (Cloud does not auto-deploy on env
// creation) OR to trigger a re-deploy without pushing a git commit.
//
// Immutable semantics: every attribute forces replace on change. The
// `redeploy_trigger` attribute is the operator's escape hatch — bump it
// (e.g. via `terraform apply -var="redeploy_trigger=$(date +%s)"`) to
// fire a fresh deploy without changing anything else.
//
// Delete is a no-op: Cloud doesn't undeploy — the environment is what's
// deployed, and destroying the deployment record just drops it from
// state. Actual rollback happens by re-deploying an earlier commit.
type DeploymentResource struct {
	client *api.Client
}

// DeploymentResourceModel is the typed state struct.
type DeploymentResourceModel struct {
	ID              types.String `tfsdk:"id"`
	EnvironmentID   types.String `tfsdk:"environment_id"`
	RedeployTrigger types.String `tfsdk:"redeploy_trigger"`
	WaitForCompletion types.Bool `tfsdk:"wait_for_completion"`
	TimeoutSeconds  types.Int64  `tfsdk:"timeout_seconds"`

	// Computed — populated on Create + refreshed on Read.
	Status          types.String `tfsdk:"status"`
	BranchName      types.String `tfsdk:"branch_name"`
	CommitHash      types.String `tfsdk:"commit_hash"`
	CommitMessage   types.String `tfsdk:"commit_message"`
	CommitAuthor    types.String `tfsdk:"commit_author"`
	FailureReason   types.String `tfsdk:"failure_reason"`
	PhpMajorVersion types.String `tfsdk:"php_major_version"`
	NodeVersion     types.String `tfsdk:"node_version"`
	BuildCommand    types.String `tfsdk:"build_command"`
	UsesOctane      types.Bool   `tfsdk:"uses_octane"`
	UsesHibernation types.Bool   `tfsdk:"uses_hibernation"`
	StartedAt       types.String `tfsdk:"started_at"`
	FinishedAt      types.String `tfsdk:"finished_at"`
	CreatedAt       types.String `tfsdk:"created_at"`
}

func NewDeploymentResource() resource.Resource {
	return &DeploymentResource{}
}

func (r *DeploymentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_deployment"
}

func (r *DeploymentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Triggers a Laravel Cloud deployment on an environment. " +
			"Cloud does NOT auto-deploy on environment creation — this resource " +
			"is what fires the first deploy + every re-deploy. Bump " +
			"`redeploy_trigger` to force a new deploy without changing anything " +
			"else. Delete is a no-op — rollback by re-deploying an earlier " +
			"commit via a fresh git push OR by changing the environment's " +
			"branch. Added in v0.5.0.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Cloud-assigned deployment ID (ULID).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"environment_id": schema.StringAttribute{
				MarkdownDescription: "Target environment ID. Immutable — changing " +
					"forces a fresh deployment resource.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"redeploy_trigger": schema.StringAttribute{
				MarkdownDescription: "Arbitrary string that forces a fresh " +
					"deployment when it changes. Common patterns: `timestamp()` " +
					"in HCL, `$(date +%s)` via `-var`, or a manual bump each " +
					"time you want to re-deploy. Leaving it unset means the " +
					"deployment only fires once (on Create).",
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"wait_for_completion": schema.BoolAttribute{
				MarkdownDescription: "When true (default), the apply blocks " +
					"until the deployment reaches a terminal status " +
					"(succeeded / failed / cancelled). When false, the apply " +
					"returns immediately after Cloud accepts the deploy " +
					"request — useful for CI where a downstream job polls.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"timeout_seconds": schema.Int64Attribute{
				MarkdownDescription: "Max seconds to wait for a terminal " +
					"status when `wait_for_completion` is true. Defaults to " +
					"1200 (20 minutes) — long enough for the largest Laravel " +
					"apps to build + roll out. Exceeded → Terraform surfaces " +
					"the timeout as an error; the deployment continues " +
					"server-side.",
				Optional: true,
				Computed: true,
				Default:  int64default.StaticInt64(1200),
			},

			// Computed status + metadata.
			"status": schema.StringAttribute{
				MarkdownDescription: "Terminal deployment status. Common terminal " +
					"values: `deployment.succeeded` (happy path), " +
					"`deployment.failed`, `build.failed`, `failed`, `cancelled`.",
				Computed: true,
			},
			"branch_name": schema.StringAttribute{
				MarkdownDescription: "Git branch this deployment built from.",
				Computed:            true,
			},
			"commit_hash": schema.StringAttribute{
				MarkdownDescription: "Git commit SHA this deployment built from.",
				Computed:            true,
			},
			"commit_message": schema.StringAttribute{
				MarkdownDescription: "Git commit message.",
				Computed:            true,
			},
			"commit_author": schema.StringAttribute{
				MarkdownDescription: "Git commit author name.",
				Computed:            true,
			},
			"failure_reason": schema.StringAttribute{
				MarkdownDescription: "Human-readable failure reason when " +
					"status is a failing terminal — else null.",
				Computed: true,
			},
			"php_major_version": schema.StringAttribute{
				MarkdownDescription: "PHP major version Cloud used to build.",
				Computed:            true,
			},
			"node_version": schema.StringAttribute{
				MarkdownDescription: "Node.js version Cloud used to build.",
				Computed:            true,
			},
			"build_command": schema.StringAttribute{
				MarkdownDescription: "Build command Cloud ran.",
				Computed:            true,
			},
			"uses_octane": schema.BoolAttribute{
				MarkdownDescription: "Whether the deploy uses Laravel Octane.",
				Computed:            true,
			},
			"uses_hibernation": schema.BoolAttribute{
				MarkdownDescription: "Whether the deploy uses hibernation (Cloud " +
					"scale-to-zero for low-traffic envs).",
				Computed: true,
			},
			"started_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp when build/deploy started.",
				Computed:            true,
			},
			"finished_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp when build/deploy finished. " +
					"Null while still in flight.",
				Computed: true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp when the deployment was " +
					"created.",
				Computed: true,
			},
		},
	}
}

func (r *DeploymentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*api.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("expected *api.Client, got %T", req.ProviderData))
		return
	}
	r.client = client
}

// Create fires a deployment, then optionally polls until terminal.
func (r *DeploymentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DeploymentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	envID := plan.EnvironmentID.ValueString()
	waitForCompletion := plan.WaitForCompletion.ValueBool()
	timeoutSeconds := plan.TimeoutSeconds.ValueInt64()

	tflog.Info(ctx, "Firing Laravel Cloud deployment", map[string]any{
		"environment_id":       envID,
		"wait_for_completion":  waitForCompletion,
		"timeout_seconds":      timeoutSeconds,
	})

	deploy, err := r.client.CreateDeployment(ctx, envID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to create deployment",
			"Cloud API returned an error while triggering the deployment. "+
				"Common causes: environment lacks a repository binding, no "+
				"build cluster available, or the branch does not exist on "+
				"the remote. Original error: "+err.Error(),
		)
		return
	}

	// Poll to terminal status when requested.
	if waitForCompletion && !deploy.IsTerminal() {
		pollCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
		defer cancel()

		tflog.Info(ctx, "Polling deployment for terminal status", map[string]any{
			"deployment_id":   deploy.ID,
			"initial_status":  deploy.Status,
			"timeout_seconds": timeoutSeconds,
		})

		terminal, pollErr := r.client.PollDeployment(pollCtx, deploy.ID)
		if terminal != nil {
			deploy = terminal
		}
		if pollErr != nil {
			// Deployment continues server-side; surface as an error but
			// still write partial state so operator sees the deployment_id.
			applyDeploymentToModel(deploy, &plan)
			_ = resp.State.Set(ctx, &plan)
			resp.Diagnostics.AddError(
				"Deployment did not reach terminal status within timeout",
				fmt.Sprintf(
					"deployment_id=%s current_status=%s\n"+
						"The deployment is still running on Cloud — Terraform's "+
						"wait budget expired first. Bump `timeout_seconds` on "+
						"the resource or set `wait_for_completion = false` to "+
						"return immediately. Original error: %s",
					deploy.ID, api.DeploymentStatusHuman(deploy.Status), pollErr.Error(),
				),
			)
			return
		}
	}

	// Surface a diagnostic when the deploy exited non-happy.
	if deploy.IsTerminal() && deploy.IsFailure() {
		reason := "(no failure reason reported)"
		if deploy.FailureReason != nil && *deploy.FailureReason != "" {
			reason = *deploy.FailureReason
		}
		applyDeploymentToModel(deploy, &plan)
		_ = resp.State.Set(ctx, &plan)
		resp.Diagnostics.AddError(
			"Deployment reached a failure state",
			fmt.Sprintf(
				"deployment_id=%s status=%s\nfailure_reason: %s\n\n"+
					"Inspect the deployment logs in the Cloud dashboard, fix "+
					"the underlying issue, and re-run `terraform apply` with a "+
					"bumped `redeploy_trigger` to fire a fresh deploy.",
				deploy.ID, api.DeploymentStatusHuman(deploy.Status), reason,
			),
		)
		return
	}

	applyDeploymentToModel(deploy, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes state from Cloud. Deployment records are terminal-ish —
// once they succeed or fail, their status doesn't change. A 404 drops
// from state (deployment was purged past its retention window).
func (r *DeploymentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DeploymentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deploy, err := r.client.GetDeployment(ctx, state.ID.ValueString())
	if err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Failed to read deployment",
			"Cloud API returned an error while reading the deployment. "+
				"Original error: "+err.Error(),
		)
		return
	}

	applyDeploymentToModel(deploy, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update handles no-op changes to `wait_for_completion` / `timeout_seconds`
// (they don't affect existing deploys). Every other attribute carries
// RequiresReplace so Update never routes there.
func (r *DeploymentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan DeploymentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Preserve computed state; only refresh from remote when necessary.
	var state DeploymentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Carry non-mutated computed values.
	plan.Status = state.Status
	plan.BranchName = state.BranchName
	plan.CommitHash = state.CommitHash
	plan.CommitMessage = state.CommitMessage
	plan.CommitAuthor = state.CommitAuthor
	plan.FailureReason = state.FailureReason
	plan.PhpMajorVersion = state.PhpMajorVersion
	plan.NodeVersion = state.NodeVersion
	plan.BuildCommand = state.BuildCommand
	plan.UsesOctane = state.UsesOctane
	plan.UsesHibernation = state.UsesHibernation
	plan.StartedAt = state.StartedAt
	plan.FinishedAt = state.FinishedAt
	plan.CreatedAt = state.CreatedAt
	plan.ID = state.ID

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete is a no-op — Cloud doesn't have an "undo deploy" endpoint. The
// deployment record stays on Cloud's side (subject to retention); we
// simply drop it from Terraform state.
func (r *DeploymentResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
	// nothing to do
}

// ImportState — `terraform import laravelcloud_deployment.foo <id>` +
// subsequent Read() populates the rest.
func (r *DeploymentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// applyDeploymentToModel copies API DTO into the state model.
func applyDeploymentToModel(d *api.Deployment, model *DeploymentResourceModel) {
	model.ID = types.StringValue(d.ID)
	model.Status = types.StringValue(d.Status)

	// environment_id — prefer the API-provided FK; fall back to plan.
	if d.EnvironmentID != "" {
		model.EnvironmentID = types.StringValue(d.EnvironmentID)
	}

	if d.BranchName != nil {
		model.BranchName = types.StringValue(*d.BranchName)
	} else {
		model.BranchName = types.StringNull()
	}
	if d.CommitHash != nil {
		model.CommitHash = types.StringValue(*d.CommitHash)
	} else {
		model.CommitHash = types.StringNull()
	}
	if d.CommitMessage != nil {
		model.CommitMessage = types.StringValue(*d.CommitMessage)
	} else {
		model.CommitMessage = types.StringNull()
	}
	if d.CommitAuthor != nil {
		model.CommitAuthor = types.StringValue(*d.CommitAuthor)
	} else {
		model.CommitAuthor = types.StringNull()
	}
	if d.FailureReason != nil {
		model.FailureReason = types.StringValue(*d.FailureReason)
	} else {
		model.FailureReason = types.StringNull()
	}
	if d.PhpMajorVersion != nil {
		model.PhpMajorVersion = types.StringValue(*d.PhpMajorVersion)
	} else {
		model.PhpMajorVersion = types.StringNull()
	}
	if d.NodeVersion != nil {
		model.NodeVersion = types.StringValue(*d.NodeVersion)
	} else {
		model.NodeVersion = types.StringNull()
	}
	if d.BuildCommand != nil {
		model.BuildCommand = types.StringValue(*d.BuildCommand)
	} else {
		model.BuildCommand = types.StringNull()
	}
	model.UsesOctane = types.BoolValue(d.UsesOctane)
	model.UsesHibernation = types.BoolValue(d.UsesHibernation)

	if d.StartedAt != nil {
		model.StartedAt = types.StringValue(d.StartedAt.Format(time.RFC3339))
	} else {
		model.StartedAt = types.StringNull()
	}
	if d.FinishedAt != nil {
		model.FinishedAt = types.StringValue(d.FinishedAt.Format(time.RFC3339))
	} else {
		model.FinishedAt = types.StringNull()
	}
	if d.CreatedAt != nil {
		model.CreatedAt = types.StringValue(d.CreatedAt.Format(time.RFC3339))
	} else {
		model.CreatedAt = types.StringNull()
	}
}

// Compile-time interface assertions.
var (
	_ resource.Resource                = (*DeploymentResource)(nil)
	_ resource.ResourceWithImportState = (*DeploymentResource)(nil)
	_ resource.ResourceWithConfigure   = (*DeploymentResource)(nil)
)
