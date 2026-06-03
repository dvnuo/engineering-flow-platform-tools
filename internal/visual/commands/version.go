package commands

import (
	"engineering-flow-platform-tools/internal/output"
	"engineering-flow-platform-tools/internal/version"
	"github.com/spf13/cobra"
)

func versionCmd(o *Opts) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print visual CLI version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			return print(cmd, o, output.Success("", map[string]any{
				"version": version.Version,
				"commit":  version.Commit,
				"date":    version.Date,
			}))
		},
	}
}
