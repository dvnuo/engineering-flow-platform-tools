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
		Short: "Inspect and automate browser page content",
		Long:  "Attach briefly to the current browser tab or a selected page target to snapshot, extract, and run bounded page actions without exporting browser secrets.",
	}
	c.AddCommand(
		pageSnapshotCmd(o),
		pageExtractCmd(o),
		pageExtractSchemaCmd(o),
		pageFindCmd(o),
		pageAXCmd(o),
		pageClickCmd(o),
		pageTypeCmd(o),
		pageSelectCmd(o),
		pageCheckCmd(o),
		pageUncheckCmd(o),
		pagePressCmd(o),
		pageUploadCmd(o),
		pageWaitCmd(o),
		pageScreenshotCmd(o),
		pageEvalCmd(o),
		pageFetchCmd(o),
		pageConsoleCmd(o),
		pageErrorsCmd(o),
		pageConsoleClearCmd(o),
		pageNetworkCmd(o),
		pageMetricsCmd(o),
		pageOutlineCmd(o),
		pageTableCmd(o),
		pageTableExportCmd(o),
		pageListCmd(o),
		pageListExportCmd(o),
		pageScrollCollectCmd(o),
		pageDiffCmd(o),
	)
	return c
}

func pageFindCmd(o *Opts) *cobra.Command {
	opts := automation.PageFindOptions{PageOptions: defaultPageOptions(), Limit: 20}
	c := &cobra.Command{
		Use:   "find",
		Short: "Find page elements by semantic locators",
		Long:  "Find page elements by selector, role, accessible name, visible text, label, placeholder, or nearby text, returning stable refs and fallback locator candidates for agents.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.Find(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().StringVar(&opts.Locator.Selector, "selector", "", "Optional CSS selector to seed semantic finding.")
	c.Flags().StringVar(&opts.Locator.Role, "role", "", "ARIA-style role to match, such as button, link, textbox, checkbox, combobox, or heading.")
	c.Flags().StringVar(&opts.Locator.Name, "name", "", "Accessible name substring to match.")
	c.Flags().StringVar(&opts.Locator.Text, "text", "", "Visible text substring to match.")
	c.Flags().StringVar(&opts.Locator.Label, "label", "", "Associated form label substring to match.")
	c.Flags().StringVar(&opts.Locator.Placeholder, "placeholder", "", "Input placeholder substring to match.")
	c.Flags().StringVar(&opts.Locator.NearText, "near-text", "", "Text near the candidate element to match.")
	c.Flags().IntVar(&opts.Locator.Nth, "nth", 0, "One-based match index within this locator; 0 returns all matches up to --limit.")
	c.Flags().IntVar(&opts.Limit, "limit", 20, "Maximum number of matching elements to return.")
	c.Flags().BoolVar(&opts.IncludeHidden, "include-hidden", false, "Include hidden elements in semantic matching.")
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
	c.Flags().BoolVar(&opts.Pierce, "pierce", false, "Traverse open shadow roots when matching elements; closed shadow roots are not accessible.")
	c.Flags().IntVar(&opts.MaxHTMLBytes, "max-html-bytes", 20000, "Maximum bytes of redacted outer HTML per element when --include-html is set.")
	return c
}

func pageExtractSchemaCmd(o *Opts) *cobra.Command {
	opts := automation.ExtractSchemaOptions{PageOptions: defaultPageOptions(), Limit: 50}
	c := &cobra.Command{
		Use:   "extract-schema",
		Short: "Extract structured page data using a YAML schema",
		Long:  "Extract selector-declared text or attribute values from the current page into stable JSON fields. Results are redacted and truncated.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.ExtractSchema(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().StringVar(&opts.File, "file", "", "YAML extraction schema file.")
	c.Flags().IntVar(&opts.Limit, "limit", 50, "Default maximum number of matching values for fields with many: true.")
	return c
}

func pageAXCmd(o *Opts) *cobra.Command {
	opts := automation.AXOptions{PageOptions: defaultPageOptions(), Limit: 100}
	c := &cobra.Command{
		Use:   "ax",
		Short: "Snapshot page accessibility-style refs",
		Long:  "Return a bounded DOM/ARIA accessibility-style tree with stable short-session refs, redacted names, state flags, bounds, and selector hints for agent interaction.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.AX(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().IntVar(&opts.Limit, "limit", 100, "Maximum number of accessibility-style nodes to return.")
	c.Flags().BoolVar(&opts.IncludeHidden, "include-hidden", false, "Include hidden nodes in the accessibility-style snapshot.")
	c.Flags().BoolVar(&opts.Pierce, "pierce", false, "Traverse open shadow roots in the accessibility-style snapshot; closed shadow roots are not accessible.")
	return c
}

func pageClickCmd(o *Opts) *cobra.Command {
	opts := automation.ClickOptions{PageOptions: defaultPageOptions()}
	c := &cobra.Command{
		Use:   "click",
		Short: "Click a visible page element",
		Long:  "Wait for a CSS selector to become visible, click it in the selected browser tab, and return redacted page metadata.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.Click(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().StringVar(&opts.Selector, "selector", "", "CSS selector for the visible element to click.")
	c.Flags().StringVar(&opts.Ref, "ref", "", "Accessibility ref from browser page ax for the visible element to click.")
	c.Flags().BoolVar(&opts.AllowRisky, "yes", false, "Confirm a click that appears to be a risky action such as submit, delete, pay, save, approve, or publish.")
	return c
}

func pageTypeCmd(o *Opts) *cobra.Command {
	opts := automation.TypeOptions{PageOptions: defaultPageOptions()}
	c := &cobra.Command{
		Use:   "type",
		Short: "Type text into a page element",
		Long:  "Wait for a CSS selector to become visible, optionally clear it, type the provided text, and return only non-secret metadata.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.Type(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().StringVar(&opts.Selector, "selector", "", "CSS selector for the visible input or editable element.")
	c.Flags().StringVar(&opts.Ref, "ref", "", "Accessibility ref from browser page ax for the visible input or editable element.")
	c.Flags().StringVar(&opts.Text, "text", "", "Text to type; the value is not included in command output.")
	c.Flags().BoolVar(&opts.Clear, "clear", false, "Clear the selected element before typing.")
	return c
}

func pageSelectCmd(o *Opts) *cobra.Command {
	opts := automation.SelectOptions{PageOptions: defaultPageOptions(), Index: -1}
	c := &cobra.Command{
		Use:   "select",
		Short: "Select an option in a page select element",
		Long:  "Select an option by value, label, or index on a select element addressed by CSS selector or accessibility ref, returning only non-secret action metadata.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.Select(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().StringVar(&opts.Selector, "selector", "", "CSS selector for the visible select element.")
	c.Flags().StringVar(&opts.Ref, "ref", "", "Accessibility ref from browser page ax for the visible select element.")
	c.Flags().StringVar(&opts.Value, "value", "", "Option value to select; the value is not included in command output.")
	c.Flags().StringVar(&opts.Label, "label", "", "Option label to select; the label is not included in command output.")
	c.Flags().IntVar(&opts.Index, "index", -1, "Zero-based option index to select.")
	return c
}

func pageCheckCmd(o *Opts) *cobra.Command {
	return pageCheckStateCmd(o, true)
}

func pageUncheckCmd(o *Opts) *cobra.Command {
	return pageCheckStateCmd(o, false)
}

func pageCheckStateCmd(o *Opts, checked bool) *cobra.Command {
	opts := automation.CheckOptions{PageOptions: defaultPageOptions(), Checked: checked}
	use := "check"
	short := "Check a checkbox-like page element"
	long := "Set a checkbox, radio, switch, or ARIA checkbox addressed by CSS selector or accessibility ref to checked and return non-secret action metadata."
	if !checked {
		use = "uncheck"
		short = "Uncheck a checkbox-like page element"
		long = "Set a checkbox, switch, or ARIA checkbox addressed by CSS selector or accessibility ref to unchecked and return non-secret action metadata."
	}
	c := &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.Check(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().StringVar(&opts.Selector, "selector", "", "CSS selector for the visible checkable element.")
	c.Flags().StringVar(&opts.Ref, "ref", "", "Accessibility ref from browser page ax for the visible checkable element.")
	return c
}

func pagePressCmd(o *Opts) *cobra.Command {
	opts := automation.PressOptions{PageOptions: defaultPageOptions()}
	c := &cobra.Command{
		Use:   "press",
		Short: "Press a keyboard key in the page",
		Long:  "Press a key in the selected page, optionally focusing an element by CSS selector or accessibility ref first, and return non-secret action metadata.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.Press(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().StringVar(&opts.Selector, "selector", "", "Optional CSS selector to focus before pressing the key.")
	c.Flags().StringVar(&opts.Ref, "ref", "", "Optional accessibility ref from browser page ax to focus before pressing the key.")
	c.Flags().StringVar(&opts.Key, "key", "", "Key to press, such as Enter, Tab, Escape, ArrowDown, or a printable character.")
	return c
}

func pageUploadCmd(o *Opts) *cobra.Command {
	opts := automation.UploadOptions{PageOptions: defaultPageOptions()}
	c := &cobra.Command{
		Use:   "upload",
		Short: "Attach files to a page file input",
		Long:  "Set files on an input[type=file] element in the selected page and return only file metadata, selector, URL, and title.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.Upload(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().StringVar(&opts.Selector, "selector", "", "CSS selector for the input[type=file] element.")
	c.Flags().StringArrayVar(&opts.Files, "file", nil, "Local regular file path to attach; repeat for multiple files.")
	c.Flags().BoolVar(&opts.Clear, "clear", false, "Clear the file input before attaching files, or clear it without files.")
	return c
}

func pageWaitCmd(o *Opts) *cobra.Command {
	opts := automation.WaitOptions{PageOptions: defaultPageOptions()}
	c := &cobra.Command{
		Use:   "wait",
		Short: "Wait for a page condition",
		Long:  "Wait for one or more bounded page conditions, then return redacted page metadata and condition summaries.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.Wait(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().StringVar(&opts.Selector, "selector", "", "Optional CSS selector to wait for until it is visible.")
	c.Flags().IntVar(&opts.DurationMilliseconds, "duration-ms", 0, "Optional bounded sleep duration in milliseconds.")
	c.Flags().StringVar(&opts.URLContains, "url-contains", "", "Wait until the current page URL contains this text.")
	c.Flags().StringVar(&opts.Text, "text", "", "Wait until visible body text contains this text.")
	c.Flags().IntVar(&opts.NetworkIdleMilliseconds, "network-idle-ms", 0, "Wait until resource timing entries stop changing for this many milliseconds.")
	c.Flags().IntVar(&opts.DOMStableMilliseconds, "dom-stable-ms", 0, "Wait until body text and DOM shape remain stable for this many milliseconds.")
	return c
}

func pageScreenshotCmd(o *Opts) *cobra.Command {
	opts := automation.ScreenshotOptions{PageOptions: defaultPageOptions(), FullPage: true}
	c := &cobra.Command{
		Use:   "screenshot",
		Short: "Write a page screenshot artifact",
		Long:  "Capture the selected browser tab to a PNG file and return file metadata instead of binary image data.",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.FullPageSet = cmd.Flags().Changed("full-page")
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.Screenshot(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().StringVar(&opts.OutPath, "out", "", "Screenshot output PNG path; defaults to ~/.efp/browser/artifacts/page-screenshot-<timestamp>.png.")
	c.Flags().BoolVar(&opts.FullPage, "full-page", true, "Capture the full page instead of only the current viewport.")
	c.Flags().StringVar(&opts.Selector, "selector", "", "Optional CSS selector for a visible element screenshot.")
	c.Flags().StringVar(&opts.Ref, "ref", "", "Optional accessibility ref from browser page ax for a visible element screenshot.")
	return c
}

func pageEvalCmd(o *Opts) *cobra.Command {
	opts := automation.EvalOptions{PageOptions: defaultPageOptions(), MaxStringBytes: 20000}
	c := &cobra.Command{
		Use:   "eval",
		Short: "Evaluate a safe page expression",
		Long:  "Evaluate a bounded JavaScript expression in the selected page and return recursively redacted serializable values.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.Eval(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().StringVar(&opts.Expression, "expr", "", "JavaScript expression to evaluate; storage, cookie, header, credential, and network APIs are rejected.")
	c.Flags().IntVar(&opts.MaxStringBytes, "max-string-bytes", 20000, "Maximum bytes per redacted string value in the returned result.")
	return c
}

func pageFetchCmd(o *Opts) *cobra.Command {
	opts := automation.FetchOptions{PageOptions: defaultPageOptions(), MaxBodyBytes: 20000}
	c := &cobra.Command{
		Use:   "fetch",
		Short: "Fetch a URL from the page context",
		Long:  "Run a sanitized GET fetch from the selected page context with credentials omitted, returning status and a redacted body preview without headers.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.Fetch(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().StringVar(&opts.URL, "url", "", "HTTP, HTTPS, or relative URL to fetch with unsafe schemes rejected.")
	c.Flags().IntVar(&opts.MaxBodyBytes, "max-body-bytes", 20000, "Maximum bytes of redacted response body preview to return.")
	return c
}

func pageConsoleCmd(o *Opts) *cobra.Command {
	opts := automation.ConsoleOptions{PageOptions: defaultPageOptions(), Limit: 50}
	c := &cobra.Command{
		Use:   "console",
		Short: "List recorded page console messages",
		Long:  "Install or read a bounded page-side console recorder and return redacted, truncated console messages without object previews.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.Console(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().StringVar(&opts.Level, "level", "", "Optional console level filter: error, warning, info, log, or debug.")
	c.Flags().IntVar(&opts.Limit, "limit", 50, "Maximum number of recorded console messages to return.")
	return c
}

func pageErrorsCmd(o *Opts) *cobra.Command {
	opts := automation.ConsoleOptions{PageOptions: defaultPageOptions(), Limit: 50}
	c := &cobra.Command{
		Use:   "errors",
		Short: "List recorded page runtime errors",
		Long:  "Install or read a bounded page-side console recorder and return redacted console errors, runtime exceptions, and unhandled rejections.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.RuntimeErrors(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().IntVar(&opts.Limit, "limit", 50, "Maximum number of recorded runtime errors to return.")
	return c
}

func pageConsoleClearCmd(o *Opts) *cobra.Command {
	opts := automation.ConsoleOptions{PageOptions: defaultPageOptions(), Limit: 50}
	c := &cobra.Command{
		Use:   "console-clear",
		Short: "Clear recorded page console messages",
		Long:  "Install the page-side console recorder if needed, clear its bounded entries, and return page metadata.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.ConsoleClear(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	return c
}

func pageNetworkCmd(o *Opts) *cobra.Command {
	opts := automation.NetworkOptions{PageOptions: defaultPageOptions(), Limit: 50}
	c := &cobra.Command{
		Use:   "network",
		Short: "Snapshot page network resource timings",
		Long:  "Return sanitized resource timing entries from the selected page, favoring API-like fetch/XHR entries by default and never returning headers or bodies.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.Network(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().StringVar(&opts.Filter, "filter", "", "Case-insensitive filter matched against resource URL, initiator type, resource type, or API-like marker.")
	c.Flags().IntVar(&opts.Limit, "limit", 50, "Maximum number of matching resource timing entries to return.")
	c.Flags().BoolVar(&opts.All, "all", false, "Include all resource timing entries instead of only API-like fetch/XHR entries.")
	return c
}

func pageMetricsCmd(o *Opts) *cobra.Command {
	opts := automation.MetricsOptions{PageOptions: defaultPageOptions(), LimitResources: 10}
	c := &cobra.Command{
		Use:   "metrics",
		Short: "Summarize browser performance timing metadata",
		Long:  "Return safe browser Performance API metadata including navigation timings, paint timings, resource aggregates, DOM node count, long-task counts, and bounded largest resources with redacted URLs.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.Metrics(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().IntVar(&opts.LimitResources, "limit-resources", 10, "Maximum number of largest matching resource timing entries to return.")
	c.Flags().StringVar(&opts.Filter, "filter", "", "Case-insensitive filter matched against resource URL, resource type, or initiator type.")
	return c
}

func pageOutlineCmd(o *Opts) *cobra.Command {
	opts := automation.OutlineOptions{PageOptions: defaultPageOptions(), Limit: 100}
	c := &cobra.Command{
		Use:   "outline",
		Short: "Snapshot page structure for agents",
		Long:  "Return a redacted DOM-derived outline of headings, links, buttons, fields, forms, tables, and lists with roles, names, labels, and selector hints.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.Outline(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().IntVar(&opts.Limit, "limit", 100, "Maximum number of outline elements to return.")
	c.Flags().BoolVar(&opts.IncludeHidden, "include-hidden", false, "Include hidden elements when deriving the page outline.")
	c.Flags().BoolVar(&opts.Pierce, "pierce", false, "Traverse open shadow roots when deriving the page outline; closed shadow roots are not accessible.")
	return c
}

func pageTableCmd(o *Opts) *cobra.Command {
	opts := automation.TableOptions{PageOptions: defaultPageOptions(), LimitRows: 50, LimitCells: 20}
	c := &cobra.Command{
		Use:   "table",
		Short: "Extract structured page tables",
		Long:  "Extract captions, headers, rows, cells, spans, and counts from tables in the selected page, with all returned text redacted and truncated.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.Table(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().StringVar(&opts.Selector, "selector", "", "Optional CSS selector for a table or container whose tables should be extracted.")
	c.Flags().IntVar(&opts.LimitRows, "limit-rows", 50, "Maximum number of rows to return per table.")
	c.Flags().IntVar(&opts.LimitCells, "limit-cells", 20, "Maximum number of cells to return per row.")
	c.Flags().BoolVar(&opts.IncludeHTML, "include-html", false, "Include a redacted and truncated table outer HTML preview.")
	return c
}

func pageListCmd(o *Opts) *cobra.Command {
	opts := automation.PageListOptions{PageOptions: defaultPageOptions(), LimitItems: 100}
	c := &cobra.Command{
		Use:   "list",
		Short: "Extract structured page lists",
		Long:  "Extract ordered, unordered, and role=list data from the selected page with redacted item text, hrefs, nesting level, and selector hints.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.PageList(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().StringVar(&opts.Selector, "selector", "", "Optional CSS selector for a list or container whose lists should be extracted.")
	c.Flags().IntVar(&opts.LimitItems, "limit-items", 100, "Maximum number of items to return per list.")
	return c
}

func pageTableExportCmd(o *Opts) *cobra.Command {
	opts := automation.DataExportOptions{PageOptions: defaultPageOptions(), Format: "json", LimitRows: 500, LimitCells: 50}
	c := &cobra.Command{
		Use:   "table-export",
		Short: "Export page tables to JSON or CSV",
		Long:  "Extract page tables and write redacted structured table data to a JSON or CSV artifact.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.TableExport(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().StringVar(&opts.Selector, "selector", "", "Optional CSS selector for a table or container whose tables should be exported.")
	c.Flags().StringVar(&opts.OutPath, "out", "", "Output file path for the table export.")
	c.Flags().StringVar(&opts.Format, "format", "json", "Export format: json or csv.")
	c.Flags().IntVar(&opts.LimitRows, "limit-rows", 500, "Maximum number of rows to export per table.")
	c.Flags().IntVar(&opts.LimitCells, "limit-cells", 50, "Maximum number of cells to export per row.")
	return c
}

func pageListExportCmd(o *Opts) *cobra.Command {
	opts := automation.DataExportOptions{PageOptions: defaultPageOptions(), Format: "json", LimitItems: 1000}
	c := &cobra.Command{
		Use:   "list-export",
		Short: "Export page lists to JSON or CSV",
		Long:  "Extract page lists and write redacted structured list data to a JSON or CSV artifact.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.ListExport(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().StringVar(&opts.Selector, "selector", "", "Optional CSS selector for a list or container whose lists should be exported.")
	c.Flags().StringVar(&opts.OutPath, "out", "", "Output file path for the list export.")
	c.Flags().StringVar(&opts.Format, "format", "json", "Export format: json or csv.")
	c.Flags().IntVar(&opts.LimitItems, "limit-items", 1000, "Maximum number of list items to export per list.")
	return c
}

func pageScrollCollectCmd(o *Opts) *cobra.Command {
	opts := automation.ScrollCollectOptions{PageOptions: defaultPageOptions(), Format: "json", Limit: 500, MaxScrolls: 20, ScrollStep: 900, IntervalMilliseconds: 250}
	c := &cobra.Command{
		Use:   "scroll-collect",
		Short: "Collect repeated page items while scrolling",
		Long:  "Scroll a page or scrollable container, collect repeated item text and links, deduplicate them, and optionally write a JSON or CSV artifact.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := automation.DefaultManager()
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(automation.PageTimeoutSeconds(opts.TimeoutSeconds))*time.Second)
			defer cancel()
			result, err := mgr.ScrollCollect(ctx, opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	addPageCommonFlags(c, &opts.PageOptions)
	c.Flags().StringVar(&opts.Selector, "selector", "", "Optional scrollable container selector; defaults to the page scroller.")
	c.Flags().StringVar(&opts.ItemSelector, "item-selector", "", "Optional selector for repeated items to collect during scrolling.")
	c.Flags().StringVar(&opts.OutPath, "out", "", "Optional output file path for collected items.")
	c.Flags().StringVar(&opts.Format, "format", "json", "Export format when --out is set: json or csv.")
	c.Flags().IntVar(&opts.Limit, "limit", 500, "Maximum number of unique items to collect.")
	c.Flags().IntVar(&opts.MaxScrolls, "max-scrolls", 20, "Maximum number of scroll steps.")
	c.Flags().IntVar(&opts.ScrollStep, "scroll-step", 900, "Pixels to scroll per step.")
	c.Flags().IntVar(&opts.IntervalMilliseconds, "interval-ms", 250, "Milliseconds to wait after each scroll step.")
	return c
}

func pageDiffCmd(o *Opts) *cobra.Command {
	opts := automation.PageDiffOptions{Limit: 100}
	c := &cobra.Command{
		Use:   "diff",
		Short: "Diff two browser page state JSON files",
		Long:  "Compare two JSON files, including browser --json envelopes, and return redacted changed JSON paths with before/after previews.",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := automation.PageDiff(opts)
			if err != nil {
				return printAutomationError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	c.Flags().StringVar(&opts.BeforeFile, "before", "", "Before JSON file, such as a browser page snapshot envelope.")
	c.Flags().StringVar(&opts.AfterFile, "after", "", "After JSON file, such as a browser page snapshot envelope.")
	c.Flags().StringVar(&opts.OutPath, "out", "", "Optional JSON file path to write the diff result.")
	c.Flags().IntVar(&opts.Limit, "limit", 100, "Maximum number of changed JSON paths to return.")
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
