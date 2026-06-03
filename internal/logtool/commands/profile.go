package commands

import (
	"engineering-flow-platform-tools/internal/logtool"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func profileCmd(o *Opts) *cobra.Command {
	var runDir string
	c := &cobra.Command{
		Use:   "profile",
		Short: "Summarize a log analysis run",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := logtool.Profile(runDir)
			if err != nil {
				return printErr(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	c.Flags().StringVar(&runDir, "run", "", "Run directory produced by log analyze.")
	return c
}
