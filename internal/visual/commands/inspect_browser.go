package commands

import (
	"engineering-flow-platform-tools/internal/output"
	"engineering-flow-platform-tools/internal/visual/browserinspect"
	visualconfig "engineering-flow-platform-tools/internal/visual/config"

	"github.com/spf13/cobra"
)

func inspectBrowserCmd(o *Opts) *cobra.Command {
	var outDir, screenshotPath, browserPath, scenario, entityID string
	var timeoutSeconds int
	var dragX, dragZ, cameraTheta, cameraPhi, cameraZoom float64
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
				Scenario:       scenario,
				EntityID:       entityID,
				DragX:          dragX,
				DragZ:          dragZ,
				CameraTheta:    cameraTheta,
				CameraPhi:      cameraPhi,
				CameraZoom:     cameraZoom,
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
	c.Flags().StringVar(&scenario, "scenario", "", "Optional browser scenario: overview, angle-left, angle-right, top, or drag")
	c.Flags().StringVar(&entityID, "entity", "", "Entity id for drag scenario")
	c.Flags().Float64Var(&dragX, "drag-x", 0, "Grid-space x delta for drag scenario")
	c.Flags().Float64Var(&dragZ, "drag-z", 0, "Grid-space z/y delta for drag scenario")
	c.Flags().Float64Var(&cameraTheta, "camera-theta", 0, "Override isometric camera theta for scenario screenshots")
	c.Flags().Float64Var(&cameraPhi, "camera-phi", 0, "Override isometric camera phi for scenario screenshots")
	c.Flags().Float64Var(&cameraZoom, "camera-zoom", 0, "Override isometric camera zoom for scenario screenshots")
	return c
}
