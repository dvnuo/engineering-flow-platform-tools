package commands

import (
	"context"
	"time"

	"engineering-flow-platform-tools/internal/browser/automation"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func networkCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{
		Use:   "network",
		Short: "Record sanitized browser network metadata",
		Long:  "Start, stop, list, wait for, or clear a bounded page-side network recorder that stores sanitized HAR-lite metadata plus redacted fetch/XHR response body previews by default, without headers, cookies, storage, or request bodies.",
	}
	c.AddCommand(
		networkStartCmd(o),
		networkStopCmd(o),
		networkListCmd(o),
		networkWaitCmd(o),
		networkExportCmd(o),
		networkClearCmd(o),
	)
	return c
}

func networkStartCmd(o *Opts) *cobra.Command {
	opts := automation.NetworkRecorderOptions{PageOptions: defaultPageOptions(), Limit: 500, Status: -1, Body: true, MaxBodyBytes: 20000}
	c := &cobra.Command{
		Use:   "start",
		Short: "Start network event recording",
		Long:  "Install a bounded page-side recorder for fetch, XHR, and resource timing metadata on the selected page target and persist a sanitized artifact.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.NetworkStart(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().IntVar(&opts.Limit, "limit", 500, "Maximum number of network metadata events to keep for this page target.")
	c.Flags().StringVar(&opts.Filter, "filter", "", "Optional case-insensitive filter applied when returning and storing events.")
	c.Flags().BoolVar(&opts.Body, "body", true, "Capture redacted fetch/XHR response body previews by default.")
	c.Flags().IntVar(&opts.MaxBodyBytes, "max-body-bytes", 20000, "Maximum bytes of redacted response body preview per fetch/XHR event.")
	return c
}

func networkStopCmd(o *Opts) *cobra.Command {
	opts := automation.NetworkRecorderOptions{PageOptions: defaultPageOptions(), Limit: 500, Status: -1, Body: true, MaxBodyBytes: 20000}
	c := &cobra.Command{
		Use:   "stop",
		Short: "Stop network event recording",
		Long:  "Stop the page-side network recorder for the selected page target and persist the final sanitized HAR-lite metadata artifact.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.NetworkStop(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	return c
}

func networkListCmd(o *Opts) *cobra.Command {
	opts := automation.NetworkRecorderOptions{PageOptions: defaultPageOptions(), Limit: 500, Status: -1, Body: true, MaxBodyBytes: 20000}
	c := &cobra.Command{
		Use:   "list",
		Short: "List recorded network events",
		Long:  "Return sanitized HAR-lite network metadata and redacted fetch/XHR response body previews from the selected page recorder artifact and page state, without headers, cookies, storage, or request bodies.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.NetworkList(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().StringVar(&opts.Filter, "filter", "", "Case-insensitive filter matched against redacted URL, method, resource type, initiator type, or source.")
	c.Flags().IntVar(&opts.Limit, "limit", 500, "Maximum number of matching network metadata events to return.")
	c.Flags().StringVar(&opts.Method, "method", "", "Optional HTTP method filter such as GET or POST when method is available.")
	c.Flags().IntVar(&opts.Status, "status", -1, "Optional HTTP status filter when status is available.")
	c.Flags().BoolVar(&opts.Body, "body", true, "Return redacted fetch/XHR response body previews by default.")
	c.Flags().IntVar(&opts.MaxBodyBytes, "max-body-bytes", 20000, "Maximum bytes of redacted response body preview per fetch/XHR event.")
	return c
}

func networkWaitCmd(o *Opts) *cobra.Command {
	opts := automation.NetworkWaitOptions{NetworkRecorderOptions: automation.NetworkRecorderOptions{PageOptions: defaultPageOptions(), Limit: 500, Status: -1, Body: true, MaxBodyBytes: 20000}}
	c := &cobra.Command{
		Use:   "wait",
		Short: "Wait for a recorded network event",
		Long:  "Poll the selected page recorder until sanitized network metadata matches a URL substring and optional method/status filter, or until timeout.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.NetworkWait(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().StringVar(&opts.URLContains, "url-contains", "", "URL substring to wait for; returned URLs are redacted.")
	c.Flags().StringVar(&opts.Method, "method", "", "Optional HTTP method filter such as GET or POST when method is available.")
	c.Flags().IntVar(&opts.Status, "status", -1, "Optional HTTP status filter when status is available.")
	c.Flags().IntVar(&opts.Limit, "limit", 500, "Maximum number of recorded events to scan per poll.")
	c.Flags().BoolVar(&opts.Body, "body", true, "Return redacted fetch/XHR response body previews by default.")
	c.Flags().IntVar(&opts.MaxBodyBytes, "max-body-bytes", 20000, "Maximum bytes of redacted response body preview per fetch/XHR event.")
	return c
}

func networkExportCmd(o *Opts) *cobra.Command {
	opts := automation.NetworkExportOptions{PageOptions: defaultPageOptions(), Format: "har-lite", Limit: 500}
	c := &cobra.Command{
		Use:   "export",
		Short: "Export sanitized recorded network metadata",
		Long:  "Write sanitized HAR-lite or JSON network recorder metadata plus redacted fetch/XHR response body previews when captured. Exports never include headers, cookies, storage, or request bodies.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.NetworkExport(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().StringVar(&opts.OutPath, "out", "", "Output file path for the sanitized network export.")
	c.Flags().StringVar(&opts.Format, "format", "har-lite", "Network export artifact format: json or har-lite. Use --json for the command envelope.")
	c.Flags().StringVar(&opts.Filter, "filter", "", "Case-insensitive filter matched against sanitized URL, method, resource type, initiator type, or source.")
	c.Flags().IntVar(&opts.Limit, "limit", 500, "Maximum number of matching sanitized network entries to write.")
	return c
}

func networkClearCmd(o *Opts) *cobra.Command {
	opts := automation.NetworkRecorderOptions{PageOptions: defaultPageOptions(), Limit: 500, Status: -1, Body: true, MaxBodyBytes: 20000}
	c := &cobra.Command{
		Use:   "clear",
		Short: "Clear recorded network events",
		Long:  "Clear the selected page-side network recorder entries and persist an empty sanitized artifact while keeping the recorder installed.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.NetworkClear(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	return c
}
