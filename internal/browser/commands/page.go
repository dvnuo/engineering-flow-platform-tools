package commands

import (
	"context"
	"time"

	"engineering-flow-platform-tools/internal/browser/automation"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func pageCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{
		Use:   "page",
		Short: "Inspect browser page content",
		Long:  "Attach briefly to the current browser tab or a selected page target to snapshot or extract redacted page content.",
	}
	c.AddCommand(pageSnapshotCmd(o), pageExtractCmd(o))
	return c
}

func pageSnapshotCmd(o *Opts) *cobra.Command {
	opts := automation.SnapshotOptions{PageOptions: defaultPageOptions()}
	c := &cobra.Command{
		Use:   "snapshot",
		Short: "Snapshot browser page text",
		Long:  "Return the current page URL, title, redacted body text preview, and optional redacted HTML preview without closing the browser tab.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.Snapshot(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().BoolVar(&opts.IncludeHTML, "include-html", false, "Include a redacted and truncated document HTML preview.")
	c.Flags().IntVar(&opts.MaxTextBytes, "max-text-bytes", 4000, "Maximum bytes of redacted body text preview to return.")
	c.Flags().IntVar(&opts.MaxHTMLBytes, "max-html-bytes", 20000, "Maximum bytes of redacted HTML preview to return when --include-html is set.")
	return c
}

func pageExtractCmd(o *Opts) *cobra.Command {
	opts := automation.ExtractOptions{PageOptions: defaultPageOptions(), Limit: 20}
	c := &cobra.Command{
		Use:   "extract",
		Short: "Extract browser page elements",
		Long:  "Return redacted text, values, links, labels, titles, tag names, and optional outer HTML for elements matching a CSS selector.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.Extract(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().StringVar(&opts.Selector, "selector", "", "CSS selector for elements to extract from the current page.")
	c.Flags().IntVar(&opts.Limit, "limit", 20, "Maximum number of matching elements to return.")
	c.Flags().BoolVar(&opts.IncludeHTML, "include-html", false, "Include redacted and truncated outer HTML for each matching element.")
	c.Flags().IntVar(&opts.MaxHTMLBytes, "max-html-bytes", 20000, "Maximum bytes of redacted outer HTML per element when --include-html is set.")
	return c
}

func defaultPageOptions() automation.PageOptions {
	return automation.PageOptions{SessionName: automation.DefaultSessionName, TimeoutSeconds: 30}
}

func addPageCommonFlags(c *cobra.Command, opts *automation.PageOptions) {
	c.Flags().StringVar(&opts.SessionName, "session", automation.DefaultSessionName, "Browser session name to connect to.")
	c.Flags().StringVar(&opts.TargetID, "target-id", "", "Optional DevTools page target id; defaults to the session's active tab.")
	c.Flags().IntVar(&opts.TimeoutSeconds, "timeout", 30, "Maximum seconds to wait for this page command.")
}
