package commands

import (
	"context"
	"errors"
	"time"

	"engineering-flow-platform-tools/internal/browser/automation"
	"engineering-flow-platform-tools/internal/browser/probe"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func workflowCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{
		Use:   "workflow",
		Short: "Run whitelisted browser automation workflows",
		Long:  "Parse, validate, dry-run, or execute a YAML workflow made only of whitelisted browser page, assert, network, console, and tab actions.",
	}
	c.AddCommand(workflowRunCmd(o))
	return c
}

func workflowRunCmd(o *Opts) *cobra.Command {
	opts := automation.WorkflowRunOptions{}
	session := automation.DefaultSessionName
	targetID := ""
	timeout := 30
	c := &cobra.Command{
		Use:   "run",
		Short: "Run a browser workflow YAML file",
		Long:  "Run a YAML browser workflow using only whitelisted browser CLI actions/assertions. It never executes shell commands, arbitrary browser CLI strings, arbitrary JavaScript, page eval, or page fetch.",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.SessionName = session
			opts.TargetID = targetID
			opts.TimeoutSeconds = timeout
			opts.SessionOverride = cmd.Flags().Changed("session")
			opts.TargetOverride = cmd.Flags().Changed("target-id")
			opts.TimeoutOverride = cmd.Flags().Changed("timeout")
			opts.ContinueOverride = cmd.Flags().Changed("continue-on-error")

			def, err := automation.LoadWorkflowFile(opts.File)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			opts.Definition = def
			if opts.DryRun {
				result, err := automation.RunWorkflow(cmd.Context(), nil, opts)
				return printWorkflow(cmd, o, result, err)
			}
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(timeout))*time.Second*time.Duration(maxWorkflowSteps(def.Steps)))
			defer cancel()
			result, err := automation.RunWorkflow(ctx, mgr, opts)
			return printWorkflow(cmd, o, result, err)
		},
	}
	c.Flags().StringVar(&opts.File, "file", "", "Workflow YAML file to parse and run.")
	c.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Parse and validate the workflow, returning a sanitized plan without attaching to a browser or executing steps.")
	c.Flags().StringVar(&session, "session", automation.DefaultSessionName, "Browser session name to use unless the workflow file sets one.")
	c.Flags().StringVar(&targetID, "target-id", "", "Optional DevTools page target id to use for page, assert, network, and console steps.")
	c.Flags().IntVar(&timeout, "timeout", 30, "Maximum seconds per browser page action/assertion step.")
	c.Flags().BoolVar(&opts.ContinueOnError, "continue-on-error", false, "Continue running later steps after a step fails; the final workflow result still fails.")
	return c
}

func printWorkflow(cmd *cobra.Command, o *Opts, result automation.WorkflowRunResult, err error) error {
	if err == nil {
		return print(cmd, o, output.Success("", result))
	}
	var workflowErr *automation.WorkflowError
	if errors.As(err, &workflowErr) {
		base := workflowErr.Base
		if base == nil {
			base = automation.NewError("workflow_failed", workflowErr.Error(), "", 412)
		}
		return output.Print(cmd.OutOrStdout(), fmtOut(o), output.Envelope{
			OK:   false,
			Data: workflowErr.Result,
			Error: &output.ErrorDetail{
				Code:    base.Code,
				Message: probe.RedactErrorMessage(base.Message),
				Hint:    base.Hint,
				Status:  base.Status,
			},
		})
	}
	return printAutomationError(cmd, o, err)
}

func maxWorkflowSteps(steps []automation.WorkflowStep) int {
	if len(steps) <= 0 {
		return 1
	}
	return len(steps)
}
