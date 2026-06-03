package commands

import (
	"engineering-flow-platform-tools/internal/logtool"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func extractCmd(o *Opts) *cobra.Command {
	var runDir, kind string
	var limit int
	limit = 20
	c := &cobra.Command{
		Use:   "extract",
		Short: "Extract stacktraces or error signatures from a run",
		RunE: func(cmd *cobra.Command, args []string) error {
			if missingRunDir(runDir) {
				return requireRunDir(cmd, o, runDir)
			}
			result, err := logtool.Extract(runDir, kind, limit)
			if err != nil {
				return printErr(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	c.Flags().StringVar(&runDir, "run", "", "Run directory produced by log analyze.")
	c.Flags().StringVar(&kind, "kind", "", "Extraction kind: stacktrace or error-signature.")
	c.Flags().IntVar(&limit, "limit", 20, "Maximum extracted items to return, up to 200.")
	return c
}
