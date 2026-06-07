package commands

import (
	"engineering-flow-platform-tools/internal/output"
	"engineering-flow-platform-tools/internal/visual/browserinspect"
	visualconfig "engineering-flow-platform-tools/internal/visual/config"

	"github.com/spf13/cobra"
)

func inspectBrowserCmd(o *Opts) *cobra.Command {
	var outDir, screenshotPath, browserPath string
	var timeoutSeconds int
	c := &cobra.Command{
		Use:   "inspect-browser",
		Short: "Open a rendered visual artifact through a local HTTP server and inspect browser screenshot readiness",
		RunE: func(cmd *cobra.Command, args []string) error {
			if outDir == "" {
				return invalidArgs(cmd, o, "--out is required", "Pass the visual artifact output directory produced by visual render.")
			}
			templateDir, err := visualconfig.ResolveTemplateDir(o.TemplateDir, o.Config)
			if err != nil {
				return print(cmd, o, failureFromError(err, "template_dir_missing"))
			}
			result, err := browserinspect.Inspect(browserinspect.Options{
				TemplateDir:    templateDir,
				OutDir:         outDir,
				Screenshot:     screenshotPath,
				OfflineStrict:  o.OfflineStrict,
				TimeoutSeconds: timeoutSeconds,
				BrowserPath:    browserPath,
			})
			if err != nil {
				return print(cmd, o, failureFromError(err, "visual_browser_inspect_failed"))
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	c.Flags().StringVar(&outDir, "out", "", "Rendered visual artifact output directory to open through local HTTP")
	c.Flags().StringVar(&screenshotPath, "screenshot", "", "Optional screenshot path; defaults to <out>/visual-screenshot.png")
	c.Flags().StringVar(&browserPath, "browser", "", "Optional Chrome/Chromium executable path; defaults to EFP_BROWSER or common system locations")
	c.Flags().IntVar(&timeoutSeconds, "timeout", 90, "Maximum seconds to wait for browser render and screenshot")
	return c
}
