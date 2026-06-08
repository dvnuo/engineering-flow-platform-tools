package commands

import (
	"engineering-flow-platform-tools/internal/output"
	visualconfig "engineering-flow-platform-tools/internal/visual/config"
	"engineering-flow-platform-tools/internal/visual/render"
	"github.com/spf13/cobra"
)

func renderCmd(o *Opts) *cobra.Command {
	var templateID, inputPath, outDir, title, dataMode string
	var overwrite bool
	c := &cobra.Command{
		Use:   "render",
		Short: "Render an offline visual artifact from a local template and JSON or Mermaid input",
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputPath == "" {
				return invalidArgs(cmd, o, "--input is required", "Pass --input <file> or --input - to read JSON or Mermaid from stdin.")
			}
			if outDir == "" {
				return invalidArgs(cmd, o, "--out is required", "Pass a new output directory path.")
			}
			templateDir, err := visualconfig.ResolveTemplateDir(o.TemplateDir, o.Config)
			if err != nil {
				return print(cmd, o, failureFromError(err, "template_dir_missing"))
			}
			result, err := render.Render(render.Options{
				TemplateDir:   templateDir,
				TemplateID:    templateID,
				InputPath:     inputPath,
				OutDir:        outDir,
				Title:         title,
				Overwrite:     overwrite,
				DryRun:        o.DryRun,
				DataMode:      dataMode,
				OfflineStrict: o.OfflineStrict,
				Stdin:         cmd.InOrStdin(),
			})
			if err != nil {
				return print(cmd, o, failureFromError(err, "output_write_failed"))
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	c.Flags().StringVar(&templateID, "template", "", "Template id from visual template list; optional for Mermaid input")
	c.Flags().StringVar(&inputPath, "input", "", "Input JSON or Mermaid file path, or - for stdin")
	c.Flags().StringVar(&outDir, "out", "", "Output directory for the static artifact")
	c.Flags().StringVar(&title, "title", "", "Artifact title override")
	c.Flags().BoolVar(&overwrite, "overwrite", false, "Replace an existing non-empty output directory")
	c.Flags().StringVar(&dataMode, "data-mode", "js-file", "Data output mode; only js-file is supported")
	return c
}

func inspectOutputCmd(o *Opts) *cobra.Command {
	var outDir string
	c := &cobra.Command{
		Use:   "inspect-output",
		Short: "Inspect a generated visual output directory for offline safety",
		RunE: func(cmd *cobra.Command, args []string) error {
			if outDir == "" {
				return invalidArgs(cmd, o, "--out is required", "Pass the visual artifact output directory.")
			}
			inspection, err := render.InspectOutput(outDir, o.OfflineStrict)
			if err != nil {
				return print(cmd, o, failureFromError(err, "offline_violation"))
			}
			return print(cmd, o, output.Success("", inspection))
		},
	}
	c.Flags().StringVar(&outDir, "out", "", "Output directory to inspect")
	return c
}
