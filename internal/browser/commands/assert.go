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

func assertCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{
		Use:   "assert",
		Short: "Assert browser page state",
		Long:  "Run safe browser page assertions that return JSON-first pass/fail metadata without exposing cookies, storage, headers, bodies, or raw page content.",
	}
	c.AddCommand(assertVisibleCmd(o), assertTextCmd(o), assertURLCmd(o), assertCountCmd(o))
	return c
}

func assertVisibleCmd(o *Opts) *cobra.Command {
	opts := automation.AssertionOptions{PageOptions: defaultPageOptions(), Equals: -1, Min: -1, Max: -1}
	c := &cobra.Command{
		Use:   "visible",
		Short: "Assert that an element is visible",
		Long:  "Assert that a CSS selector or accessibility ref resolves to at least one visible page element, returning sanitized page and assertion metadata.",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := runAssertion(cmd, opts.PageOptions, func(ctx context.Context, mgr *automation.Manager) (automation.AssertionResult, error) {
				return mgr.AssertVisible(ctx, opts)
			})
			return printAssertion(cmd, o, result, err)
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().StringVar(&opts.Selector, "selector", "", "CSS selector for the visible element assertion.")
	c.Flags().StringVar(&opts.Ref, "ref", "", "Accessibility ref from browser page ax for the visible element assertion.")
	c.Flags().BoolVar(&opts.Not, "not", false, "Invert the assertion so it passes when the element is not visible.")
	return c
}

func assertTextCmd(o *Opts) *cobra.Command {
	opts := automation.AssertionOptions{PageOptions: defaultPageOptions(), Equals: -1, Min: -1, Max: -1}
	c := &cobra.Command{
		Use:   "text",
		Short: "Assert that page or element text contains a substring",
		Long:  "Assert that the selected element text, or page body text when no target is given, contains a substring. Output reports redacted/truncated expectation metadata and text lengths, not page text snippets.",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := runAssertion(cmd, opts.PageOptions, func(ctx context.Context, mgr *automation.Manager) (automation.AssertionResult, error) {
				return mgr.AssertText(ctx, opts)
			})
			return printAssertion(cmd, o, result, err)
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().StringVar(&opts.Contains, "contains", "", "Text substring to assert; output returns only redacted/truncated expectation metadata and byte counts.")
	c.Flags().StringVar(&opts.Selector, "selector", "", "Optional CSS selector whose text should be checked.")
	c.Flags().StringVar(&opts.Ref, "ref", "", "Optional accessibility ref from browser page ax whose text should be checked.")
	c.Flags().BoolVar(&opts.Not, "not", false, "Invert the assertion so it passes when the substring is absent.")
	return c
}

func assertURLCmd(o *Opts) *cobra.Command {
	opts := automation.AssertionOptions{PageOptions: defaultPageOptions(), Equals: -1, Min: -1, Max: -1}
	c := &cobra.Command{
		Use:   "url",
		Short: "Assert that the current URL contains a substring",
		Long:  "Assert that the current page URL contains a substring, returning the redacted current URL and sanitized expectation metadata.",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := runAssertion(cmd, opts.PageOptions, func(ctx context.Context, mgr *automation.Manager) (automation.AssertionResult, error) {
				return mgr.AssertURL(ctx, opts)
			})
			return printAssertion(cmd, o, result, err)
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().StringVar(&opts.Contains, "contains", "", "URL substring to assert; returned URLs and expectation metadata are redacted.")
	c.Flags().BoolVar(&opts.Not, "not", false, "Invert the assertion so it passes when the substring is absent.")
	return c
}

func assertCountCmd(o *Opts) *cobra.Command {
	opts := automation.AssertionOptions{PageOptions: defaultPageOptions(), Equals: -1, Min: -1, Max: -1}
	c := &cobra.Command{
		Use:   "count",
		Short: "Assert a selector match count",
		Long:  "Assert a CSS selector's element count against an exact count or inclusive min/max bounds.",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := runAssertion(cmd, opts.PageOptions, func(ctx context.Context, mgr *automation.Manager) (automation.AssertionResult, error) {
				return mgr.AssertCount(ctx, opts)
			})
			return printAssertion(cmd, o, result, err)
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().StringVar(&opts.Selector, "selector", "", "CSS selector whose matching element count should be asserted.")
	c.Flags().IntVar(&opts.Equals, "equals", -1, "Exact expected selector count.")
	c.Flags().IntVar(&opts.Min, "min", -1, "Minimum expected selector count.")
	c.Flags().IntVar(&opts.Max, "max", -1, "Maximum expected selector count.")
	return c
}

func runAssertion(cmd *cobra.Command, opts automation.PageOptions, fn func(context.Context, *automation.Manager) (automation.AssertionResult, error)) (automation.AssertionResult, error) {
	mgr, err := automation.DefaultManager()
	if err != nil {
		return automation.AssertionResult{}, err
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
	defer cancel()
	return fn(ctx, mgr)
}

func printAssertion(cmd *cobra.Command, o *Opts, result automation.AssertionResult, err error) error {
	if err == nil {
		return print(cmd, o, output.Success("", result))
	}
	var assertErr *automation.AssertionError
	if errors.As(err, &assertErr) {
		base := assertErr.Base
		if base == nil {
			base = automation.NewError("assertion_failed", assertErr.Error(), "", 412)
		}
		detail := &output.ErrorDetail{
			Code:    base.Code,
			Message: probe.RedactErrorMessage(base.Message),
			Hint:    base.Hint,
			Status:  base.Status,
		}
		return output.Print(cmd.OutOrStdout(), fmtOut(o), output.Envelope{
			OK:    false,
			Data:  assertErr.Result,
			Error: detail,
		})
	}
	return printAutomationError(cmd, o, err)
}
