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

// CommandResource runs a one-shot artisan / shell command inside an
// environment. Common uses: `migrate --force` after a schema change,
// `db:seed --class=NewSeeder` after landing new seed data,
// `cache:clear` after a config bump.
//
// Bump `rerun_trigger` to fire a fresh command. Delete is a no-op —
// Cloud retains the command record for its history retention window.
//
// Added in v0.5.0.
type CommandResource struct {
	client *api.Client
}

type CommandResourceModel struct {
	ID                types.String `tfsdk:"id"`
	EnvironmentID     types.String `tfsdk:"environment_id"`
	Command           types.String `tfsdk:"command"`
	RerunTrigger      types.String `tfsdk:"rerun_trigger"`
	WaitForCompletion types.Bool   `tfsdk:"wait_for_completion"`
	TimeoutSeconds    types.Int64  `tfsdk:"timeout_seconds"`

	Status        types.String `tfsdk:"status"`
	Output        types.String `tfsdk:"output"`
	ExitCode      types.Int64  `tfsdk:"exit_code"`
	FailureReason types.String `tfsdk:"failure_reason"`
	StartedAt     types.String `tfsdk:"started_at"`
	FinishedAt    types.String `tfsdk:"finished_at"`
	CreatedAt     types.String `tfsdk:"created_at"`
}

func NewCommandResource() resource.Resource { return &CommandResource{} }

func (r *CommandResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_command"
}

func (r *CommandResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Runs a one-shot command inside a Laravel Cloud " +
			"environment. Cloud spins a fresh short-lived container attached " +
			"to the env's DB + cache + secrets, executes the command, streams " +
			"output back, and terminates. Bump `rerun_trigger` to re-run. " +
			"Delete is a no-op. Added in v0.5.0.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"environment_id": schema.StringAttribute{
				MarkdownDescription: "Target environment. Immutable.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"command": schema.StringAttribute{
				MarkdownDescription: "Command to run — typically an artisan invocation like " +
					"`migrate --force` or a shell one-liner like `php artisan cache:clear`.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"rerun_trigger": schema.StringAttribute{
				MarkdownDescription: "Arbitrary string that forces a re-run when it changes.",
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"wait_for_completion": schema.BoolAttribute{
				MarkdownDescription: "When true (default), apply blocks until the command " +
					"reaches a terminal status.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"timeout_seconds": schema.Int64Attribute{
				MarkdownDescription: "Max seconds to wait. Defaults to 600.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(600),
			},

			"status": schema.StringAttribute{
				MarkdownDescription: "Terminal status — `completed`, `failed`, `cancelled`, `timeout`.",
				Computed:            true,
			},
			"output": schema.StringAttribute{
				MarkdownDescription: "Combined stdout+stderr captured from the command.",
				Computed:            true,
			},
			"exit_code": schema.Int64Attribute{
				MarkdownDescription: "Process exit code. Null while still running.",
				Computed:            true,
			},
			"failure_reason": schema.StringAttribute{
				MarkdownDescription: "Human-readable failure reason on non-happy terminal.",
				Computed:            true,
			},
			"started_at": schema.StringAttribute{Computed: true},
			"finished_at": schema.StringAttribute{
				MarkdownDescription: "RFC3339 timestamp when the command finished. Null while running.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{Computed: true},
		},
	}
}

func (r *CommandResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *CommandResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan CommandResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	envID := plan.EnvironmentID.ValueString()
	waitFor := plan.WaitForCompletion.ValueBool()
	timeoutSeconds := plan.TimeoutSeconds.ValueInt64()

	tflog.Info(ctx, "Running Cloud command", map[string]any{
		"environment_id":      envID,
		"command":             plan.Command.ValueString(),
		"wait_for_completion": waitFor,
	})

	cmd, err := r.client.RunCommand(ctx, envID, api.RunCommandRequest{
		Command: plan.Command.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to run command", err.Error())
		return
	}

	if waitFor && !cmd.IsTerminal() {
		pollCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
		defer cancel()

		terminal, pollErr := r.client.PollCommand(pollCtx, cmd.ID)
		if terminal != nil {
			cmd = terminal
		}
		if pollErr != nil {
			applyCommandToModel(cmd, &plan)
			_ = resp.State.Set(ctx, &plan)
			resp.Diagnostics.AddError(
				"Command did not reach terminal status within timeout",
				fmt.Sprintf("command_id=%s current_status=%s: %s", cmd.ID, cmd.Status, pollErr.Error()),
			)
			return
		}
	}

	if cmd.IsTerminal() && cmd.IsFailure() {
		reason := "(no failure reason)"
		if cmd.FailureReason != nil && *cmd.FailureReason != "" {
			reason = *cmd.FailureReason
		}
		applyCommandToModel(cmd, &plan)
		_ = resp.State.Set(ctx, &plan)
		resp.Diagnostics.AddError(
			"Command exited with failure",
			fmt.Sprintf(
				"command_id=%s status=%s exit_code=%s\nfailure_reason: %s\n\nOutput:\n%s",
				cmd.ID, cmd.Status,
				formatExitCode(cmd.ExitCode), reason,
				formatCommandOutput(cmd.Output),
			),
		)
		return
	}

	applyCommandToModel(cmd, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *CommandResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state CommandResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cmd, err := r.client.GetCommand(ctx, state.ID.ValueString())
	if err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read command", err.Error())
		return
	}
	applyCommandToModel(cmd, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *CommandResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Only wait_for_completion / timeout_seconds can update; carry state forward.
	var plan, state CommandResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID
	plan.Status = state.Status
	plan.Output = state.Output
	plan.ExitCode = state.ExitCode
	plan.FailureReason = state.FailureReason
	plan.StartedAt = state.StartedAt
	plan.FinishedAt = state.FinishedAt
	plan.CreatedAt = state.CreatedAt
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *CommandResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
	// No-op — Cloud has no "delete command" endpoint.
}

func (r *CommandResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func formatExitCode(e *int) string {
	if e == nil {
		return "(unset)"
	}
	return fmt.Sprintf("%d", *e)
}

func formatCommandOutput(o *string) string {
	if o == nil || *o == "" {
		return "(no output)"
	}
	// Cap at ~4KB in the diagnostic — the full output stays in state.
	const maxLen = 4096
	if len(*o) > maxLen {
		return (*o)[:maxLen] + "\n[... truncated in diagnostic; full output in state ...]"
	}
	return *o
}

func applyCommandToModel(c *api.Command, model *CommandResourceModel) {
	model.ID = types.StringValue(c.ID)
	if c.EnvironmentID != "" {
		model.EnvironmentID = types.StringValue(c.EnvironmentID)
	}
	if c.Command != "" {
		model.Command = types.StringValue(c.Command)
	}
	model.Status = types.StringValue(c.Status)
	if c.Output != nil {
		model.Output = types.StringValue(*c.Output)
	} else {
		model.Output = types.StringNull()
	}
	if c.ExitCode != nil {
		model.ExitCode = types.Int64Value(int64(*c.ExitCode))
	} else {
		model.ExitCode = types.Int64Null()
	}
	if c.FailureReason != nil {
		model.FailureReason = types.StringValue(*c.FailureReason)
	} else {
		model.FailureReason = types.StringNull()
	}
	if c.StartedAt != nil {
		model.StartedAt = types.StringValue(c.StartedAt.Format(time.RFC3339))
	} else {
		model.StartedAt = types.StringNull()
	}
	if c.FinishedAt != nil {
		model.FinishedAt = types.StringValue(c.FinishedAt.Format(time.RFC3339))
	} else {
		model.FinishedAt = types.StringNull()
	}
	if c.CreatedAt != nil {
		model.CreatedAt = types.StringValue(c.CreatedAt.Format(time.RFC3339))
	} else {
		model.CreatedAt = types.StringNull()
	}
}

var (
	_ resource.Resource                = (*CommandResource)(nil)
	_ resource.ResourceWithImportState = (*CommandResource)(nil)
	_ resource.ResourceWithConfigure   = (*CommandResource)(nil)
)
