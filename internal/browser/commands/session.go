package commands

import (
	"context"
	"errors"
	"strconv"
	"strings"
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
		Long:  "Manage visible Edge/Chrome/Chromium browser sessions launched with a dedicated profile, local DevTools endpoint, and detached process lifetime for multi-step agent workflows.",
	}
	c.AddCommand(sessionStartCmd(o), sessionListCmd(o), sessionStatusCmd(o), sessionAttachCmd(o), sessionDiscoverCmd(o), sessionStopCmd(o))
	return c
}

func sessionStartCmd(o *Opts) *cobra.Command {
	opts := automation.StartOptions{Name: "default", Browser: "chrome"}
	c := &cobra.Command{
		Use:   "start",
		Short: "Start a persistent browser automation session",
		Long:  "Start a visible Edge/Chrome/Chromium process with a dedicated profile and a DevTools endpoint bound to 127.0.0.1. The browser process is detached from the short-lived CLI caller when the platform allows it.",
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
	c.Flags().StringVar(&opts.Browser, "browser", "chrome", "Browser family to launch: chrome, edge, chromium, or auto.")
	c.Flags().StringVar(&opts.BrowserExe, "browser-exe", "", "Explicit Edge/Chrome/Chromium executable path to launch.")
	c.Flags().BoolVar(&opts.Headless, "headless", false, "Run the persistent browser without a visible UI.")
	c.Flags().StringVar(&opts.ProfileDir, "profile", "", "Dedicated browser profile directory; defaults to ~/.efp/browser/profiles/<session-name>.")
	c.Flags().StringVar(&opts.DownloadDir, "download-dir", "", "Dedicated download directory; defaults to ~/.efp/browser/downloads/<session-name>.")
	c.Flags().BoolVar(&opts.CleanProfile, "clean-profile", false, "Delete the dedicated profile directory before launching the session.")
	c.Flags().IntVar(&opts.Port, "port", 0, "Local DevTools port on 127.0.0.1; 0 picks a free port.")
	c.Flags().StringVar(&opts.URL, "url", "", "Optional initial HTTP or HTTPS URL to open in the session.")
	return c
}

func sessionAttachCmd(o *Opts) *cobra.Command {
	opts := automation.AttachOptions{Name: "default", DebugAddr: automation.LocalDebugAddr}
	c := &cobra.Command{
		Use:   "attach",
		Short: "Attach session metadata to an explicit local DevTools endpoint",
		Long:  "Attach browser session metadata to an explicitly supplied 127.0.0.1 DevTools port. This does not inspect default browser cookies or launch/stop the external browser process.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 8*time.Second)
			defer cancel()
			session, err := mgr.Attach(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", session))
		},
	}
	c.Flags().StringVar(&opts.Name, "name", "default", "Session name used for metadata.")
	c.Flags().StringVar(&opts.DebugAddr, "debug-addr", automation.LocalDebugAddr, "Explicit local DevTools address; only 127.0.0.1 is allowed.")
	c.Flags().IntVar(&opts.DebugPort, "debug-port", 0, "Explicit local DevTools port exposed by a browser launched by the user.")
	return c
}

func sessionDiscoverCmd(o *Opts) *cobra.Command {
	opts := automation.DiscoverOptions{DebugAddr: automation.LocalDebugAddr, Ports: []int{9222, 9223, 9224}}
	portsRaw := "9222,9223,9224"
	c := &cobra.Command{
		Use:   "discover",
		Short: "Discover explicit local DevTools endpoints",
		Long:  "Probe explicitly listed 127.0.0.1 DevTools ports and return redacted target metadata. It does not scan arbitrary hosts or browser profiles.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ports, err := parsePortList(portsRaw)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			opts.Ports = ports
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 8*time.Second)
			defer cancel()
			discovered, err := mgr.Discover(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", map[string]any{"sessions": discovered}))
		},
	}
	c.Flags().StringVar(&opts.DebugAddr, "debug-addr", automation.LocalDebugAddr, "Explicit local DevTools address; only 127.0.0.1 is allowed.")
	c.Flags().StringVar(&portsRaw, "ports", "9222,9223,9224", "Comma-separated explicit local DevTools ports to probe.")
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

func parsePortList(raw string) ([]int, error) {
	parts := strings.Split(raw, ",")
	ports := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		port, err := strconv.Atoi(part)
		if err != nil || port <= 0 || port > 65535 {
			return nil, automation.NewError("invalid_args", "--ports must contain integers between 1 and 65535", "Pass a comma-separated list such as --ports 9222,9223.", 400)
		}
		ports = append(ports, port)
	}
	if len(ports) == 0 {
		return nil, automation.NewError("invalid_args", "--ports must include at least one port", "Pass a comma-separated list such as --ports 9222,9223.", 400)
	}
	return ports, nil
}

func printAutomationError(cmd *cobra.Command, o *Opts, err error) error {
	var autoErr *automation.Error
	if errors.As(err, &autoErr) {
		return print(cmd, o, output.Failure(autoErr.Code, probe.RedactErrorMessage(autoErr.Message), autoErr.Hint, autoErr.Status))
	}
	return print(cmd, o, output.Failure("automation_failed", probe.RedactErrorMessage(err.Error()), "", 500))
}
