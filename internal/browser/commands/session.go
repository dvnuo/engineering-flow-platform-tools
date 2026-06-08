package commands

import (
	"context"
	"errors"
	"time"

	"engineering-flow-platform-tools/internal/browser/automation"
	"engineering-flow-platform-tools/internal/browser/probe"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func sessionCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{
		Use:   "session",
		Short: "Manage persistent browser automation sessions",
		Long:  "Manage visible Edge/Chrome/Chromium browser sessions launched with a dedicated profile and local DevTools endpoint.",
	}
	c.AddCommand(sessionStartCmd(o), sessionListCmd(o), sessionStatusCmd(o), sessionStopCmd(o))
	return c
}

func sessionStartCmd(o *Opts) *cobra.Command {
	opts := automation.StartOptions{Name: "default", Browser: "auto"}
	c := &cobra.Command{
		Use:   "start",
		Short: "Start a persistent browser automation session",
		Long:  "Start a visible Edge/Chrome/Chromium process with a dedicated profile and a DevTools endpoint bound to 127.0.0.1.",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Verbose = o.Verbose
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 20*time.Second)
			defer cancel()
			session, err := mgr.Start(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", session))
		},
	}
	c.Flags().StringVar(&opts.Name, "name", "default", "Session name used for metadata and the default profile directory.")
	c.Flags().StringVar(&opts.Browser, "browser", "auto", "Browser family to launch: edge, chrome, chromium, or auto.")
	c.Flags().StringVar(&opts.BrowserExe, "browser-exe", "", "Explicit Edge/Chrome/Chromium executable path to launch.")
	c.Flags().BoolVar(&opts.Headless, "headless", false, "Run the persistent browser without a visible UI.")
	c.Flags().StringVar(&opts.ProfileDir, "profile", "", "Dedicated browser profile directory; defaults to ~/.efp/browser/profiles/<session-name>.")
	c.Flags().BoolVar(&opts.CleanProfile, "clean-profile", false, "Delete the dedicated profile directory before launching the session.")
	c.Flags().IntVar(&opts.Port, "port", 0, "Local DevTools port on 127.0.0.1; 0 picks a free port.")
	c.Flags().StringVar(&opts.URL, "url", "", "Optional initial HTTP or HTTPS URL to open in the session.")
	return c
}

func sessionListCmd(o *Opts) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List stored browser automation sessions",
		Long:  "List browser automation sessions from metadata and refresh whether each local DevTools endpoint is alive.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 8*time.Second)
			defer cancel()
			sessions, err := mgr.List(ctx)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", map[string]any{"sessions": sessions}))
		},
	}
}

func sessionStatusCmd(o *Opts) *cobra.Command {
	return &cobra.Command{
		Use:   "status [name]",
		Short: "Show browser automation session status",
		Long:  "Show one stored browser automation session and refresh whether its local DevTools endpoint is alive.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := "default"
			if len(args) > 0 {
				name = args[0]
			}
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 8*time.Second)
			defer cancel()
			session, err := mgr.Status(ctx, name)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", session))
		},
	}
}

func sessionStopCmd(o *Opts) *cobra.Command {
	opts := automation.StopOptions{Name: "default"}
	c := &cobra.Command{
		Use:   "stop [name]",
		Short: "Stop a browser automation session",
		Long:  "Stop a browser automation session started by this CLI using stored process metadata when possible.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Name = args[0]
			} else {
				opts.Name = "default"
			}
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()
			session, err := mgr.Stop(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", session))
		},
	}
	c.Flags().BoolVar(&opts.KeepMetadata, "keep-metadata", false, "Keep the session metadata file after stopping or finding a stale browser.")
	return c
}

func printAutomationError(cmd *cobra.Command, o *Opts, err error) error {
	var autoErr *automation.Error
	if errors.As(err, &autoErr) {
		return print(cmd, o, output.Failure(autoErr.Code, probe.RedactErrorMessage(autoErr.Message), autoErr.Hint, autoErr.Status))
	}
	return print(cmd, o, output.Failure("automation_failed", probe.RedactErrorMessage(err.Error()), "", 500))
}
