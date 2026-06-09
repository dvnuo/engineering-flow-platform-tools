package commands

import (
	"context"
	"time"

	"engineering-flow-platform-tools/internal/browser/automation"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func downloadCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{
		Use:   "download",
		Short: "Inspect browser session downloads",
		Long:  "List or wait for completed files in a browser session's dedicated download directory without reading file contents.",
	}
	c.AddCommand(downloadListCmd(o), downloadWaitCmd(o))
	return c
}

func downloadListCmd(o *Opts) *cobra.Command {
	opts := automation.DownloadListOptions{SessionName: automation.DefaultSessionName}
	c := &cobra.Command{
		Use:   "list",
		Short: "List completed session downloads",
		Long:  "List completed non-temporary files in a browser session's dedicated download directory, returning metadata only.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 8*time.Second)
			defer cancel()
			result, err := mgr.DownloadList(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	c.Flags().StringVar(&opts.SessionName, "session", automation.DefaultSessionName, "Browser session name whose download directory should be listed.")
	return c
}

func downloadWaitCmd(o *Opts) *cobra.Command {
	opts := automation.DownloadWaitOptions{SessionName: automation.DefaultSessionName, TimeoutSeconds: 30}
	c := &cobra.Command{
		Use:   "wait",
		Short: "Wait for a completed session download",
		Long:  "Poll a browser session's dedicated download directory until a matching non-temporary file appears and file metadata settles.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.DownloadWait(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	c.Flags().StringVar(&opts.SessionName, "session", automation.DefaultSessionName, "Browser session name whose download directory should be watched.")
	c.Flags().StringVar(&opts.FilenameContains, "filename-contains", "", "Optional case-insensitive substring that the completed filename must contain.")
	c.Flags().IntVar(&opts.TimeoutSeconds, "timeout", 30, "Maximum seconds to wait for a completed matching download.")
	return c
}
