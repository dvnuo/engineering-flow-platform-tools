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
		Use:   "search [run]",
		Short: "Search redacted log entries with bounded results",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, ok, err := resolveRunDirArg(cmd, o, runDir, args)
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			if opts.Query == "" && opts.Cursor == "" {
				return print(cmd, o, output.Failure("invalid_args", "--query or --cursor is required.", "Pass --query <text>, or pass --cursor from a previous search response.", 400))
			}
			result, err := logtool.Search(logtool.ResolveRunDir(resolved), opts)
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
	c.Flags().StringVar(&opts.Service, "service", "", "Filter matches to one parsed service or component.")
	c.Flags().StringVar(&opts.TemplateID, "template-id", "", "Filter matches to one template id.")
	c.Flags().StringVar(&opts.TemplateID, "template", "", "Alias for --template-id.")
	c.Flags().StringVar(&opts.Since, "since", "", "Only include entries at or after this timestamp.")
	c.Flags().StringVar(&opts.Until, "until", "", "Only include entries at or before this timestamp.")
	c.Flags().IntVar(&opts.Limit, "limit", 20, "Maximum matches to return, up to 200.")
	c.Flags().StringVar(&opts.Cursor, "cursor", "", "Cursor returned by a previous search response.")
	return c
}
