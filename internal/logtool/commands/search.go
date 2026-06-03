package commands

import (
	"engineering-flow-platform-tools/internal/logtool"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func searchCmd(o *Opts) *cobra.Command {
	opts := logtool.SearchOptions{Limit: 20}
	var runDir string
	c := &cobra.Command{
		Use:   "search",
		Short: "Search redacted log entries with bounded results",
		RunE: func(cmd *cobra.Command, args []string) error {
			if missingRunDir(runDir) {
				return requireRunDir(cmd, o, runDir)
			}
			if opts.Query == "" {
				return print(cmd, o, output.Failure("invalid_args", "--query is required.", "Pass --query <text> or run log schema search --json.", 400))
			}
			result, err := logtool.Search(runDir, opts)
			if err != nil {
				return printErr(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	c.Flags().StringVar(&runDir, "run", "", "Run directory produced by log analyze.")
	c.Flags().StringVar(&opts.Query, "query", "", "Search query text; separate alternatives with OR.")
	c.Flags().BoolVar(&opts.Regex, "regex", false, "Treat --query as a regular expression.")
	c.Flags().StringVar(&opts.Level, "level", "", "Filter by normalized level: TRACE, DEBUG, INFO, WARN, ERROR, FATAL, or PANIC.")
	c.Flags().StringVar(&opts.TemplateID, "template-id", "", "Filter matches to one template id.")
	c.Flags().StringVar(&opts.Since, "since", "", "Only include entries at or after this timestamp.")
	c.Flags().StringVar(&opts.Until, "until", "", "Only include entries at or before this timestamp.")
	c.Flags().IntVar(&opts.Limit, "limit", 20, "Maximum matches to return, up to 200.")
	c.Flags().StringVar(&opts.Cursor, "cursor", "", "Cursor returned by a previous search response.")
	return c
}
