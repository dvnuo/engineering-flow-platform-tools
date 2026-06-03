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
