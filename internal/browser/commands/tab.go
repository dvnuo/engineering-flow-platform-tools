package commands

import (
	"context"
	"time"

	"engineering-flow-platform-tools/internal/browser/automation"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func tabCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{
		Use:   "tab",
		Short: "Inspect and control tabs in a browser automation session",
		Long:  "Inspect, select, activate, and open page tabs in an existing browser automation session.",
	}
	c.AddCommand(tabListCmd(o), tabCurrentCmd(o), tabActivateCmd(o), tabOpenCmd(o))
	return c
}

func tabListCmd(o *Opts) *cobra.Command {
	session := "default"
	c := &cobra.Command{
		Use:   "list",
		Short: "List page tabs in a browser automation session",
		Long:  "List DevTools page targets for an existing browser automation session and mark the stored active target.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 8*time.Second)
			defer cancel()
			result, err := mgr.TabList(ctx, session)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	c.Flags().StringVar(&session, "session", "default", "Browser automation session name to connect to.")
	return c
}

func tabCurrentCmd(o *Opts) *cobra.Command {
	session := "default"
	c := &cobra.Command{
		Use:   "current",
		Short: "Show the current page tab for a session",
		Long:  "Return the stored active page target, or choose and persist the first available page target.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 8*time.Second)
			defer cancel()
			result, err := mgr.CurrentTab(ctx, session)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	c.Flags().StringVar(&session, "session", "default", "Browser automation session name to connect to.")
	return c
}

func tabActivateCmd(o *Opts) *cobra.Command {
	session := "default"
	targetID := ""
	c := &cobra.Command{
		Use:   "activate",
		Short: "Activate a page tab in a session",
		Long:  "Activate a DevTools page target and persist it as the session's active target.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 8*time.Second)
			defer cancel()
			result, err := mgr.ActivateTab(ctx, session, targetID)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	c.Flags().StringVar(&session, "session", "default", "Browser automation session name to connect to.")
	c.Flags().StringVar(&targetID, "target-id", "", "DevTools page target id from browser tab list.")
	return c
}

func tabOpenCmd(o *Opts) *cobra.Command {
	session := "default"
	rawURL := ""
	c := &cobra.Command{
		Use:   "open",
		Short: "Open a new page tab in a session",
		Long:  "Open an HTTP or HTTPS URL in a new tab through the browser session's DevTools endpoint.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 8*time.Second)
			defer cancel()
			result, err := mgr.OpenTab(ctx, session, rawURL)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	c.Flags().StringVar(&session, "session", "default", "Browser automation session name to connect to.")
	c.Flags().StringVar(&rawURL, "url", "", "HTTP or HTTPS URL to open in a new tab.")
	return c
}
