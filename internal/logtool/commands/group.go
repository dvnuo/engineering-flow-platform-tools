package commands

import (
	"engineering-flow-platform-tools/internal/logtool"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func groupCmd(o *Opts) *cobra.Command {
	opts := logtool.GroupOptions{By: "template", Bucket: "1m", Limit: 50}
	var runDir string
	c := &cobra.Command{
		Use:   "group [run]",
		Short: "Group entries by template, error signature, level, service, or time",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, ok, err := resolveRunDirArg(cmd, o, runDir, args)
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			result, err := logtool.Group(logtool.ResolveRunDir(resolved), opts)
			if err != nil {
				return printErr(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	c.Flags().StringVar(&runDir, "run", "", "Run directory produced by log analyze.")
	c.Flags().StringVar(&opts.By, "by", "template", "Grouping key: template, error_signature, level, service, or time.")
	c.Flags().StringVar(&opts.Level, "level", "", "Filter by normalized level.")
	c.Flags().StringVar(&opts.Query, "query", "", "Filter entries by text before grouping.")
	c.Flags().StringVar(&opts.TemplateID, "template-id", "", "Filter to one template id.")
	c.Flags().StringVar(&opts.Bucket, "bucket", "1m", "Time bucket duration when --by time.")
	c.Flags().IntVar(&opts.Limit, "limit", 50, "Maximum groups to return, up to 200.")
	return c
}
