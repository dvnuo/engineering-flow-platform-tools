package commands

import (
	"context"
	"time"

	"engineering-flow-platform-tools/internal/browser/automation"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func formCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{
		Use:   "form",
		Short: "Inspect and fill browser forms",
		Long:  "Inspect form field metadata or fill fields from a YAML file without echoing field values.",
	}
	c.AddCommand(formInspectCmd(o), formFillCmd(o))
	return c
}

func formInspectCmd(o *Opts) *cobra.Command {
	opts := automation.FormInspectOptions{PageOptions: defaultPageOptions(), Limit: 100}
	c := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect form field metadata",
		Long:  "Return bounded form field metadata such as labels, names, types, selectors, and options without returning current field values.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.FormInspect(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().StringVar(&opts.Selector, "selector", "", "Optional form/container selector to inspect.")
	c.Flags().IntVar(&opts.Limit, "limit", 100, "Maximum number of form fields to return.")
	return c
}

func formFillCmd(o *Opts) *cobra.Command {
	opts := automation.FormFillOptions{PageOptions: defaultPageOptions()}
	c := &cobra.Command{
		Use:   "fill",
		Short: "Fill form fields from YAML",
		Long:  "Fill form fields from fields: {name_or_selector: value} YAML. Output includes match metadata and value byte counts only.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.FormFill(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().StringVar(&opts.File, "file", "", "YAML file with fields: {name_or_selector: value}.")
	return c
}
