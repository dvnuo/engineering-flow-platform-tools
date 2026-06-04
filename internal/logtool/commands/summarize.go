package commands

import (
	"engineering-flow-platform-tools/internal/logtool"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func summarizeCmd(o *Opts) *cobra.Command {
	opts := logtool.SummaryOptions{}
	var runDir string
	c := &cobra.Command{
		Use:   "summarize [run]",
		Short: "Create a deterministic evidence summary for a run",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, ok, err := resolveRunDirArg(cmd, o, runDir, args)
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			result, err := logtool.Summarize(logtool.ResolveRunDir(resolved), opts)
			if err != nil {
				return printErr(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	c.Flags().StringVar(&runDir, "run", "", "Run directory produced by log analyze.")
	c.Flags().StringVar(&opts.Focus, "focus", "", "Optional investigation focus; no LLM is called.")
	c.Flags().StringVar(&opts.Since, "since", "", "Optional start timestamp for agent planning metadata.")
	c.Flags().StringVar(&opts.Until, "until", "", "Optional end timestamp for agent planning metadata.")
	return c
}
