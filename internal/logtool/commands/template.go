package commands

import (
	"engineering-flow-platform-tools/internal/logtool"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func templateCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{
		Use:   "template",
		Short: "Inspect recurring redacted log templates",
	}
	c.AddCommand(templateListCmd(o), templateGetCmd(o), templateEntriesCmd(o), templateVariablesCmd(o))
	return c
}

func templateListCmd(o *Opts) *cobra.Command {
	var only, sortBy string
	var limit int
	c := &cobra.Command{
		Use:   "list <run>",
		Short: "List recurring redacted log templates",
		RunE: func(cmd *cobra.Command, args []string) error {
			runDir, ok, err := resolveRunDirArg(cmd, o, "", args)
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			result, err := logtool.Templates(logtool.ResolveRunDir(runDir), only, sortBy, limit)
			if err != nil {
				return printErr(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	c.Flags().StringVar(&only, "only", "all", "Template filter: all or non-info.")
	c.Flags().StringVar(&sortBy, "sort", "count", "Template sort: count, first_seen, or last_seen.")
	c.Flags().IntVar(&limit, "limit", 50, "Maximum templates to return, up to 200.")
	return c
}

func templateGetCmd(o *Opts) *cobra.Command {
	var templateID string
	c := &cobra.Command{
		Use:   "get <run>",
		Short: "Get one redacted template by id",
		RunE: func(cmd *cobra.Command, args []string) error {
			runDir, ok, err := resolveRunDirArg(cmd, o, "", args)
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			result, err := logtool.TemplateGet(logtool.ResolveRunDir(runDir), templateID)
			if err != nil {
				return printErr(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	c.Flags().StringVar(&templateID, "template", "", "Template id returned by template list.")
	return c
}

func templateEntriesCmd(o *Opts) *cobra.Command {
	var templateID string
	opts := logtool.EntryListOptions{Limit: 20}
	c := &cobra.Command{
		Use:   "entries <run>",
		Short: "List bounded entries for one template",
		RunE: func(cmd *cobra.Command, args []string) error {
			runDir, ok, err := resolveRunDirArg(cmd, o, "", args)
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			result, err := logtool.TemplateEntries(logtool.ResolveRunDir(runDir), templateID, opts)
			if err != nil {
				return printErr(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	c.Flags().StringVar(&templateID, "template", "", "Template id returned by template list.")
	c.Flags().StringVar(&opts.Level, "level", "", "Filter by normalized level.")
	c.Flags().IntVar(&opts.Limit, "limit", 20, "Maximum entries to return, up to 200.")
	c.Flags().StringVar(&opts.Cursor, "cursor", "", "Cursor returned by a previous response.")
	return c
}

func templateVariablesCmd(o *Opts) *cobra.Command {
	var templateID string
	var limit int
	c := &cobra.Command{
		Use:   "variables <run>",
		Short: "Summarize redacted variable samples for one template",
		RunE: func(cmd *cobra.Command, args []string) error {
			runDir, ok, err := resolveRunDirArg(cmd, o, "", args)
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			result, err := logtool.TemplateVariables(logtool.ResolveRunDir(runDir), templateID, limit)
			if err != nil {
				return printErr(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	c.Flags().StringVar(&templateID, "template", "", "Template id returned by template list.")
	c.Flags().IntVar(&limit, "limit", 20, "Maximum variable values per position, up to 200.")
	return c
}
