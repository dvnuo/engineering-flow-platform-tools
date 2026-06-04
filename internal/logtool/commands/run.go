package commands

import (
	"engineering-flow-platform-tools/internal/logtool"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func runCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{
		Use:   "run",
		Short: "Manage local log analysis run directories",
	}
	c.AddCommand(runListCmd(o), runGetCmd(o), runDeleteCmd(o), runVerifyCmd(o))
	return c
}

func runListCmd(o *Opts) *cobra.Command {
	var workspace string
	c := &cobra.Command{
		Use:   "list",
		Short: "List runs in the default or selected workspace",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := logtool.RunList(workspace)
			if err != nil {
				return printErr(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	c.Flags().StringVar(&workspace, "workspace", "", "Workspace directory to scan; defaults to ~/.efp/log-runs.")
	return c
}

func runGetCmd(o *Opts) *cobra.Command {
	return &cobra.Command{
		Use:   "get <run>",
		Short: "Show one run manifest and index summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return print(cmd, o, output.Failure("invalid_args", "run is required.", "Pass a run id or run directory.", 400))
			}
			result, err := logtool.RunGet(args[0])
			if err != nil {
				return printErr(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
}

func runDeleteCmd(o *Opts) *cobra.Command {
	var yes, dryRun bool
	c := &cobra.Command{
		Use:   "delete <run>",
		Short: "Delete a run directory after explicit confirmation",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return print(cmd, o, output.Failure("invalid_args", "run is required.", "Pass a run id or run directory.", 400))
			}
			result, err := logtool.RunDelete(args[0], yes, dryRun)
			if err != nil {
				return printErr(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	c.Flags().BoolVar(&yes, "yes", false, "Confirm deletion of the run directory.")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "Validate the delete request without removing files.")
	return c
}

func runVerifyCmd(o *Opts) *cobra.Command {
	return &cobra.Command{
		Use:   "verify <run>",
		Short: "Verify run files and manifest counts",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return print(cmd, o, output.Failure("invalid_args", "run is required.", "Pass a run id or run directory.", 400))
			}
			result, err := logtool.RunVerify(args[0])
			if err != nil {
				return printErr(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
}
