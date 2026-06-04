package commands

import (
	"engineering-flow-platform-tools/internal/logtool"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func timelineCmd(o *Opts) *cobra.Command {
	opts := logtool.TimelineOptions{Bucket: "1m", Limit: 200}
	var runDir string
	c := &cobra.Command{
		Use:   "timeline [run]",
		Short: "Build a bounded time series from timestamped entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, ok, err := resolveRunDirArg(cmd, o, runDir, args)
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			result, err := logtool.Timeline(logtool.ResolveRunDir(resolved), opts)
			if err != nil {
				return printErr(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	c.Flags().StringVar(&runDir, "run", "", "Run directory produced by log analyze.")
	c.Flags().StringVar(&opts.Bucket, "bucket", "1m", "Bucket duration such as 30s, 1m, or 5m.")
	c.Flags().StringVar(&opts.Level, "level", "", "Filter by normalized level.")
	c.Flags().StringVar(&opts.TemplateID, "template-id", "", "Filter to one template id.")
	c.Flags().IntVar(&opts.Limit, "limit", 200, "Maximum buckets to return, up to 200.")
	return c
}
