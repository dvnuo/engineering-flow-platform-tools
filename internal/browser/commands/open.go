package commands

import (
	"context"
	"time"

	"engineering-flow-platform-tools/internal/browser/automation"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func openCmd(o *Opts) *cobra.Command {
	opts := automation.StartOptions{Name: "default", Browser: "chrome"}
	c := &cobra.Command{
		Use:   "open",
		Short: "Open a URL in a persistent browser session",
		Long:  "Open an HTTP or HTTPS URL in a visible persistent browser for manual login and page operations in later turns. Start the named managed session when none is running; otherwise reuse it and always open the URL in a new tab.",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Verbose = o.Verbose
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 75*time.Second)
			defer cancel()
			result, err := mgr.OpenPersistent(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	c.Flags().StringVar(&opts.URL, "url", "", "HTTP or HTTPS URL to open and leave available for later actions.")
	c.Flags().StringVar(&opts.Name, "session", "default", "Persistent browser session name to start or reuse.")
	c.Flags().StringVar(&opts.Browser, "browser", "chrome", "Browser family for a new session (chrome, edge, chromium, or auto).")
	c.Flags().StringVar(&opts.BrowserExe, "browser-exe", "", "Explicit Edge/Chrome/Chromium executable path for a new session.")
	c.Flags().BoolVar(&opts.Headless, "headless", false, "Run a newly started persistent browser without a visible UI.")
	c.Flags().StringVar(&opts.ProfileDir, "profile", "", "Dedicated profile for a new session; defaults to ~/.efp/browser/profiles/<session-name>.")
	c.Flags().StringVar(&opts.DownloadDir, "download-dir", "", "Dedicated download directory for a new session; defaults to ~/.efp/browser/downloads/<session-name>.")
	c.Flags().BoolVar(&opts.CleanProfile, "clean-profile", false, "Delete the dedicated profile before starting a new session.")
	c.Flags().IntVar(&opts.Port, "port", 0, "Local DevTools port for a new session; 0 picks a free port on 127.0.0.1.")
	return c
}
