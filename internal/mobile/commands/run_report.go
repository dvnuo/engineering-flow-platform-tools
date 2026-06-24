package commands

import (
	"encoding/json"
	"os"
	"path/filepath"

	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func runReportCmd(o *Opts) *cobra.Command {
	var runID, outPath string
	c := &cobra.Command{Use: "report", RunE: func(cmd *cobra.Command, args []string) error {
		if runID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id is required", "Pass the run id to summarize.", 400))
		}
		svc, err := newServices(o, false)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		st, err := svc.Store.LoadRun(runID)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		events, err := svc.Store.LoadTimeline(runID)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		report := map[string]any{
			"run":            st,
			"timeline":       events,
			"timeline_count": len(events),
			"state_path":     svc.Store.StatePath(runID),
			"timeline_path":  svc.Store.TimelinePath(runID),
		}
		if outPath != "" {
			path := outPath
			if !filepath.IsAbs(path) {
				path = filepath.Join(svc.Store.RunDir(runID), path)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return renderErr(cmd, o, err)
			}
			b, err := json.MarshalIndent(output.RedactValue(report), "", "  ")
			if err != nil {
				return renderErr(cmd, o, err)
			}
			if err := os.WriteFile(path, b, 0o600); err != nil {
				return renderErr(cmd, o, err)
			}
			report["report_path"] = path
			appendTimelineBestEffort(svc, runID, "report", "run_report", "", st.Status, map[string]any{"path": path})
		}
		return print(cmd, o, output.Success("", report))
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	c.Flags().StringVar(&outPath, "out", "", "")
	return c
}
