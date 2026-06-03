package commands

import (
	"engineering-flow-platform-tools/internal/logtool"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func entriesCmd(o *Opts) *cobra.Command {
	opts := logtool.EntryListOptions{Limit: 50}
	var runDir string
	c := &cobra.Command{
		Use:   "entries",
		Short: "List bounded redacted entries from a run",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := logtool.Entries(runDir, opts)
			if err != nil {
				return printErr(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	c.Flags().StringVar(&runDir, "run", "", "Run directory produced by log analyze.")
	c.Flags().StringVar(&opts.TemplateID, "template-id", "", "Filter entries to one template id.")
	c.Flags().StringVar(&opts.Level, "level", "", "Filter by normalized level: TRACE, DEBUG, INFO, WARN, ERROR, FATAL, or PANIC.")
	c.Flags().IntVar(&opts.Limit, "limit", 50, "Maximum entries to return, up to 200.")
	c.Flags().StringVar(&opts.Cursor, "cursor", "", "Cursor returned by a previous entries response.")
	return c
}
