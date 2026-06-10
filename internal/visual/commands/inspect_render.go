package commands

import (
	"engineering-flow-platform-tools/internal/output"
	visualconfig "engineering-flow-platform-tools/internal/visual/config"
	"engineering-flow-platform-tools/internal/visual/renderinspect"

	"github.com/spf13/cobra"
)

func inspectRenderCmd(o *Opts) *cobra.Command {
	var outDir, screenshotPath string
	c := &cobra.Command{
		Use:   "inspect-render",
		Short: "Inspect a rendered visual artifact for offline safety and first-view readability",
		RunE: func(cmd *cobra.Command, args []string) error {
			if outDir == "" {
				return invalidArgs(cmd, o, "--out is required", "Pass the visual artifact output directory produced by visual render.")
			}
			templateDir, err := visualconfig.ResolveTemplateDir(o.TemplateDir, o.Config)
			if err != nil {
				return print(cmd, o, failureFromError(err, "template_dir_missing"))
			}
			result, err := renderinspect.Inspect(renderinspect.Options{TemplateDir: templateDir, OutDir: outDir, Screenshot: screenshotPath, OfflineStrict: o.OfflineStrict})
			if err != nil {
				return print(cmd, o, failureFromError(err, "visual_render_inspect_failed"))
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	c.Flags().StringVar(&outDir, "out", "", "Rendered visual artifact output directory to inspect")
	c.Flags().StringVar(&screenshotPath, "screenshot", "", "Optional PNG, JPEG, or GIF screenshot for pixel-level readability checks")
	return c
}
