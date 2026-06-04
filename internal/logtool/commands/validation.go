package commands

import (
	"strings"

	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func requireRunDir(cmd *cobra.Command, o *Opts, runDir string) error {
	if missingRunDir(runDir) {
		return print(cmd, o, output.Failure(
			"invalid_args",
			"--run is required.",
			"Pass --run <run-dir> produced by log analyze.",
			400,
		))
	}
	return nil
}

func missingRunDir(runDir string) bool {
	return strings.TrimSpace(runDir) == ""
}

func resolveRunDirArg(cmd *cobra.Command, o *Opts, runDir string, args []string) (string, bool, error) {
	if len(args) > 1 {
		return "", false, print(cmd, o, output.Failure(
			"invalid_args",
			"Only one run argument is allowed.",
			"Pass a run id/path once, or use --run <run-dir>.",
			400,
		))
	}
	if missingRunDir(runDir) && len(args) == 1 {
		runDir = args[0]
	}
	if missingRunDir(runDir) {
		return "", false, requireRunDir(cmd, o, runDir)
	}
	return runDir, true, nil
}
