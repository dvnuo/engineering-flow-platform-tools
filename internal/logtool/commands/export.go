package commands

import (
	"engineering-flow-platform-tools/internal/logtool"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func exportCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{
		Use:   "export",
		Short: "Export redacted evidence artifacts",
	}
	c.AddCommand(exportEvidenceCmd(o))
	return c
}

func exportEvidenceCmd(o *Opts) *cobra.Command {
	opts := logtool.ExportEvidenceOptions{Format: "json"}
	var runDir string
	c := &cobra.Command{
		Use:   "evidence [run]",
		Short: "Export one redacted entry or template evidence item",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, ok, err := resolveRunDirArg(cmd, o, runDir, args)
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			result, err := logtool.ExportEvidence(logtool.ResolveRunDir(resolved), opts)
			if err != nil {
				return printErr(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	c.Flags().StringVar(&runDir, "run", "", "Run directory produced by log analyze.")
	c.Flags().StringVar(&opts.Evidence, "evidence", "", "Entry id or template id to export.")
	c.Flags().StringVar(&opts.Format, "format", "json", "Evidence file format: json or markdown.")
	c.Flags().StringVar(&opts.Output, "output", "", "Output file for the redacted evidence.")
	c.Flags().BoolVar(&opts.Overwrite, "overwrite", false, "Replace an existing output file.")
	c.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Validate export without writing a file.")
	return c
}
