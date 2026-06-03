package commands

import (
	"engineering-flow-platform-tools/internal/logtool"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func templatesCmd(o *Opts) *cobra.Command {
	var runDir, only, sortBy string
	var limit int
	c := &cobra.Command{
		Use:   "templates",
		Short: "List recurring redacted log templates from a run",
		RunE: func(cmd *cobra.Command, args []string) error {
			if missingRunDir(runDir) {
				return requireRunDir(cmd, o, runDir)
			}
			result, err := logtool.Templates(runDir, only, sortBy, limit)
			if err != nil {
				return printErr(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	c.Flags().StringVar(&runDir, "run", "", "Run directory produced by log analyze.")
	c.Flags().StringVar(&only, "only", "all", "Template filter: all or non-info.")
	c.Flags().StringVar(&sortBy, "sort", "count", "Template sort: count, first_seen, or last_seen.")
	c.Flags().IntVar(&limit, "limit", 50, "Maximum templates to return, up to 200.")
	return c
}
