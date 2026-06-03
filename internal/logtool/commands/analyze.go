package commands

import (
	"engineering-flow-platform-tools/internal/logtool"
	"engineering-flow-platform-tools/internal/output"
	"engineering-flow-platform-tools/internal/version"
	"github.com/spf13/cobra"
)

func analyzeCmd(o *Opts) *cobra.Command {
	opts := logtool.AnalyzeOptions{FormatHint: "auto", MaxLineBytes: 65536}
	c := &cobra.Command{
		Use:   "analyze",
		Short: "Stream local log files into a bounded analysis run directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ToolVersion = version.Version
			result, err := logtool.Analyze(cmd.Context(), opts)
			if err != nil {
				return printErr(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	c.Flags().StringVar(&opts.Source, "source", "", "Local log file, directory, or glob to analyze.")
	c.Flags().StringVar(&opts.RunDir, "run", "", "Run directory where manifest.json, entries.jsonl, and templates.json are written.")
	c.Flags().StringVar(&opts.FormatHint, "format-hint", "auto", "Input format hint: auto, json, or plain.")
	c.Flags().Int64Var(&opts.MaxBytes, "max-bytes", 0, "Maximum bytes to ingest across sources; zero means no explicit cap.")
	c.Flags().Int64Var(&opts.MaxLineBytes, "max-line-bytes", 65536, "Maximum bytes kept per line preview before truncation.")
	return c
}
