package commands

import (
	"context"
	"time"

	"engineering-flow-platform-tools/internal/browser/automation"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func frameCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{
		Use:   "frame",
		Short: "Inspect frames in a browser page target",
		Long:  "List frames and snapshot a selected frame through DevTools with redacted URLs, titles, text, and optional HTML previews.",
	}
	c.AddCommand(frameListCmd(o), frameSnapshotCmd(o))
	return c
}

func frameListCmd(o *Opts) *cobra.Command {
	opts := automation.FrameOptions{PageOptions: defaultPageOptions()}
	c := &cobra.Command{
		Use:   "list",
		Short: "List frames in the selected page target",
		Long:  "Return the DevTools frame tree for the selected page target with frame URLs and names redacted.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.FrameList(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	return c
}

func frameSnapshotCmd(o *Opts) *cobra.Command {
	opts := automation.FrameSnapshotOptions{PageOptions: defaultPageOptions(), MaxTextBytes: 4000, MaxHTMLBytes: 20000}
	c := &cobra.Command{
		Use:   "snapshot",
		Short: "Snapshot one frame",
		Long:  "Evaluate a bounded read in a selected frame and return redacted frame URL, title, body text preview, and optional HTML preview.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.FrameSnapshot(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().StringVar(&opts.FrameID, "frame-id", "", "Frame id from browser frame list.")
	c.Flags().BoolVar(&opts.IncludeHTML, "include-html", false, "Include a redacted and truncated frame HTML preview.")
	c.Flags().IntVar(&opts.MaxTextBytes, "max-text-bytes", 4000, "Maximum bytes of redacted frame body text preview to return.")
	c.Flags().IntVar(&opts.MaxHTMLBytes, "max-html-bytes", 20000, "Maximum bytes of redacted frame HTML preview to return when --include-html is set.")
	return c
}
