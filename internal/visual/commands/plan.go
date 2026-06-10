package commands

import (
	"engineering-flow-platform-tools/internal/output"
	visualconfig "engineering-flow-platform-tools/internal/visual/config"
	"engineering-flow-platform-tools/internal/visual/plan"
	"github.com/spf13/cobra"
)

func inspectPlanCmd(o *Opts) *cobra.Command {
	var templateID, inputPath, outDir string
	c := &cobra.Command{
		Use:   "inspect-plan",
		Short: "Compile a visual input into an agent-readable render plan before rendering",
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputPath == "" {
				return invalidArgs(cmd, o, "--input is required", "Pass --input <file> or --input - to read Mermaid from stdin.")
			}
			templateDir, err := visualconfig.ResolveTemplateDir(o.TemplateDir, o.Config)
			if err != nil {
				return print(cmd, o, failureFromError(err, "template_dir_missing"))
			}
			result, err := plan.Inspect(plan.Options{
				TemplateDir: templateDir,
				TemplateID:  templateID,
				InputPath:   inputPath,
				OutDir:      outDir,
				Stdin:       cmd.InOrStdin(),
			})
			if err != nil {
				return print(cmd, o, failureFromError(err, "visual_plan_failed"))
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	c.Flags().StringVar(&templateID, "template", "", "Template id from visual template list; optional for Mermaid input")
	c.Flags().StringVar(&inputPath, "input", "", "Input Mermaid file path, or - for stdin")
	c.Flags().StringVar(&outDir, "out", "", "Optional planned output directory for render command hints")
	return c
}
