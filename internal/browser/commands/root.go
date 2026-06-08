package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/browser/probe"
	"engineering-flow-platform-tools/internal/catalog"
	"engineering-flow-platform-tools/internal/clihelp"
	"engineering-flow-platform-tools/internal/output"
	"engineering-flow-platform-tools/internal/version"
	"github.com/spf13/cobra"
)

type Opts struct {
	Format        string
	JSON, Verbose bool
}

func NewRoot() *cobra.Command {
	return NewRootWithRunner(probe.NewChromeDPRunner())
}

func NewRootWithRunner(r probe.Runner) *cobra.Command {
	cobra.EnableCommandSorting = false
	o := &Opts{Format: "table"}
	c := &cobra.Command{Use: "browser", SilenceErrors: true, SilenceUsage: true}
	c.PersistentFlags().BoolVar(&o.JSON, "json", false, "")
	c.PersistentFlags().StringVar(&o.Format, "format", "table", "")
	c.PersistentFlags().BoolVar(&o.Verbose, "verbose", false, "")
	c.AddCommand(probeCmd(o, r), sessionCmd(o), tabCmd(o), pageCmd(o), assertCmd(o), workflowCmd(o), frameCmd(o), networkCmd(o), downloadCmd(o), commandsCmd(o), schemaCmd(o), helpLLMCmd(o), versionCmd(o))
	clihelp.ApplyCatalogHelp(c, clihelp.ProductHelp{
		Product: "browser",
		Binary:  "browser",
		Short:   "Probe browser SSO and page state through Edge/Chrome/Chromium",
		Long: strings.TrimSpace(`browser is a terminal-invoked CLI for agents that need to open an internal URL, capture page artifacts, and inspect browser SSO indicators through Edge, Chrome, or Chromium DevTools.

It writes non-secret diagnostics such as summary.json, network.json, page.html, and screenshot.png. It does not export cookies or tokens. For agent workflows, default every command and subcommand to --json so callers can read ok, data.files, error.code, and error.hint.`),
		Examples: []string{
			`browser probe --url https://intranet.example.test --selector .user-avatar --wait 10 --out result --json`,
			`browser session start --name default --url https://intranet.example.test --json`,
			`browser session status default --json`,
			`browser tab current --session default --json`,
			`browser page snapshot --session default --json`,
			`browser page ax --json`,
			`browser page click --selector button.sign-in --json`,
			`browser page click --ref axref-0-abcdef123456 --json`,
			`browser page wait --selector .ready --network-idle-ms 500 --json`,
			`browser assert visible --selector .ready --json`,
			`browser assert text --contains "Signed in" --json`,
			`browser assert url --contains /dashboard --json`,
			`browser assert count --selector .result --min 1 --json`,
			`browser workflow run --file flow.yaml --dry-run --json`,
			`browser workflow run --file flow.yaml --session default --json`,
			`browser page network --filter /api/ --json`,
			`browser page metrics --limit-resources 10 --json`,
			`browser network start --session default --limit 500 --json`,
			`browser network export --out result/network.har-lite.json --format har-lite --json`,
			`browser page console --level error --json`,
			`browser frame list --json`,
			`browser page outline --json`,
			`browser page table --selector table.results --json`,
			`browser page upload --selector input[type=file] --file ./report.pdf --json`,
			`browser download wait --session default --filename-contains report --json`,
			`browser page screenshot --out result/page-screenshot.png --json`,
			`browser schema probe --json`,
			`browser help llm --json`,
		},
		Instructions: "copy cmd/browser/browser-cli.instructions.md to ~/.copilot/instructions/browser-cli.instructions.md.",
	})
	return c
}

func probeCmd(o *Opts, r probe.Runner) *cobra.Command {
	opts := probe.ProbeOptions{WaitSeconds: 8, TimeoutSeconds: 90, OutDir: "result", Browser: "auto", MaxNetworkEvents: 1000, SaveHTML: true, SaveScreenshot: true}
	c := &cobra.Command{Use: "probe", RunE: func(cmd *cobra.Command, args []string) error {
		opts.Verbose = o.Verbose
		if strings.TrimSpace(opts.URL) == "" {
			return print(cmd, o, output.Failure("invalid_args", "--url is required", "Run browser schema probe --json.", 400))
		}
		if opts.RequireSelector && strings.TrimSpace(opts.Selector) == "" {
			return print(cmd, o, output.Failure(
				"invalid_args",
				"--require-selector requires --selector",
				"Pass --selector <css> or remove --require-selector.",
				400,
			))
		}
		if opts.ProfileDir == "" {
			opts.ProfileDir = probe.DefaultProfileDir()
		}
		timeout := time.Duration(opts.TimeoutSeconds) * time.Second
		if timeout <= 0 {
			timeout = 90 * time.Second
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		result, err := r.Probe(ctx, opts)
		if err != nil {
			var probeErr *probe.ProbeError
			if errors.As(err, &probeErr) {
				return print(cmd, o, output.Failure(probeErr.Code, probeErr.Message, probeErr.Hint, probeErr.Status))
			}
			return print(cmd, o, output.Failure("server_error", probe.RedactErrorMessage(err.Error()), "", 500))
		}
		if opts.RequireSelector && opts.Selector != "" && !result.SelectorFound {
			return print(cmd, o, output.Failure(
				"selector_not_found",
				"Selector was not found.",
				"Selector was not found. Inspect the generated summary.json, screenshot.png, and page.html.",
				404,
			))
		}
		return print(cmd, o, output.Success("", result))
	}}
	c.Flags().StringVar(&opts.URL, "url", "", "")
	c.Flags().StringVar(&opts.Selector, "selector", "", "")
	c.Flags().BoolVar(&opts.RequireSelector, "require-selector", false, "")
	c.Flags().IntVar(&opts.WaitSeconds, "wait", 8, "")
	c.Flags().IntVar(&opts.TimeoutSeconds, "timeout", 90, "")
	c.Flags().StringVar(&opts.OutDir, "out", "result", "")
	c.Flags().StringVar(&opts.ProfileDir, "profile", "", "")
	c.Flags().BoolVar(&opts.CleanProfile, "clean-profile", false, "")
	c.Flags().StringVar(&opts.BrowserExe, "browser-exe", "", "")
	c.Flags().StringVar(&opts.Browser, "browser", "auto", "")
	c.Flags().BoolVar(&opts.Headless, "headless", false, "")
	c.Flags().BoolVar(&opts.IgnoreCertErrors, "ignore-cert-errors", false, "")
	c.Flags().StringVar(&opts.FetchAPI, "fetch-api", "", "")
	c.Flags().StringVar(&opts.NetworkFilter, "network-filter", "", "")
	c.Flags().IntVar(&opts.MaxNetworkEvents, "max-network-events", 1000, "")
	c.Flags().BoolVar(&opts.SaveHTML, "save-html", true, "")
	c.Flags().BoolVar(&opts.SaveScreenshot, "save-screenshot", true, "")
	return c
}

func commandsCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "commands", RunE: func(cmd *cobra.Command, args []string) error {
		return print(cmd, o, output.Success("", map[string]any{"commands": catalog.CommandsFromCobra("browser", cmd.Root())}))
	}}
}

func schemaCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "schema <command>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		schema, ok := catalog.SchemaFromCobra("browser", args[0], cmd.Root())
		if !ok {
			return print(cmd, o, output.Failure("not_found", "command not found", "Run browser commands --json to list command names.", 404))
		}
		return print(cmd, o, output.Success("", schema))
	}}
}

func versionCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "version", RunE: func(cmd *cobra.Command, args []string) error {
		return print(cmd, o, output.Success("", map[string]any{"version": version.Version, "commit": version.Commit, "date": version.Date}))
	}}
}

func helpLLMCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "help llm", RunE: func(cmd *cobra.Command, args []string) error {
		tips := browserLLMTips()
		if fmtOut(o) == "json" {
			return print(cmd, o, output.Success("", map[string]any{"tips": tips, "commands": catalog.Commands("browser")}))
		}
		_, err := fmt.Fprintln(cmd.OutOrStdout(), browserLLMMarkdown(tips))
		return err
	}}
}

func browserLLMTips() []string {
	return []string{
		"For agents, --json is the default way to use every browser command and subcommand.",
		"Always add --json so results and failures use the stable ok/data/error envelope; omit it only when intentionally reading human-oriented --help text.",
		"browser is a terminal-invoked CLI binary.",
		"It opens Edge/Chrome/Chromium through DevTools.",
		"It uses a dedicated browser profile by default.",
		"It does not export cookies or tokens.",
		"Use browser session start to keep a dedicated browser open for multi-step agent workflows.",
		"Use browser tab list/current/activate/open to inspect and select page targets in a persistent session.",
		"Use browser page snapshot and browser page extract for redacted page reads.",
		"Use browser page ax to get accessibility-style refs for short-session ref-based interactions; rerun it after navigation or DOM changes.",
		"Use browser page outline, table, and list for structured page reads that are easier for agents to navigate than raw text extraction.",
		"Use --pierce on page extract, outline, or ax only for open shadow roots; closed shadow roots are not accessible.",
		"Use browser page network for sanitized resource timing summaries; it returns no headers, cookies, or bodies.",
		"Use browser page metrics for navigation, paint/resource aggregate, DOM node count, long-task count, and bounded largest-resource timing metadata; it returns no headers, cookies, storage, or bodies.",
		"Use browser assert visible/text/url/count for JSON-first page state checks; assertion failures return ok=false with error.code assertion_failed and sanitized details in data.",
		"Dedicated console/network assertions are not separate assert commands in this pass; use browser network wait/list and browser page console/errors for those checks.",
		"Use browser workflow run --file flow.yaml --dry-run --json before executing YAML workflows; workflows only call whitelisted browser actions/assertions and never run shell commands, arbitrary browser CLI strings, arbitrary JavaScript, page eval, or page fetch.",
		"Use browser network start/list/wait/export/stop/clear for sanitized HAR-lite metadata after recording starts; it returns no headers, cookies, storage, or bodies.",
		"Use browser page console and browser page errors for redacted console/runtime diagnostics captured after recorder injection.",
		"Use browser frame list and browser frame snapshot for redacted frame reads.",
		"Use browser page click/type/select/check/uncheck/press/upload/wait/screenshot/eval/fetch for bounded page actions against the active or selected tab.",
		"Use either --selector or --ref for ref-capable actions; action outputs do not echo typed text or selected option values.",
		"browser page wait can wait for selectors, URL substrings, visible text, network-idle timing, DOM stability, or a bounded duration.",
		"browser page screenshot writes a PNG artifact and returns file metadata, not image bytes; element screenshots require a visible --selector or fresh --ref.",
		"browser network export writes JSON or HAR-lite metadata artifacts and returns path/count/size metadata only.",
		"browser page eval rejects cookie, storage, header, credential, and network APIs, then recursively redacts returned values.",
		"browser page fetch performs a sanitized GET with credentials omitted, rejects unsafe URL schemes, and returns no headers.",
		"browser page upload validates local regular files and returns file metadata only; it never prints file contents.",
		"browser download list/wait read only file metadata from the session download directory.",
		"Use --selector to check login success.",
		"Use --clean-profile to distinguish true OS/enterprise SSO from cached browser session.",
		"Inspect network.json and summary.json.",
		"Do not treat negotiate_401_seen as proof; it is only an indicator.",
		"Command parsing failures return an invalid_args JSON envelope when --json is present.",
		"On Windows cmd, use double quotes and cmd-native commands such as where/dir/cd/type; do not use Bash-only commands such as pwd, command -v, cat, ls, cd \"$PWD\", or single quotes.",
		"If terminal output capture is unreliable, rerun the exact .exe path from where browser, redirect the JSON envelope to a workspace file, read it with the file-read tool, and inspect artifact files under --out.",
		"In OpenCode runtime, this command requires a browser executable in the runtime image.",
	}
}

func browserLLMMarkdown(tips []string) string {
	var b strings.Builder
	b.WriteString("# browser CLI usage for agents\n\n")
	for _, tip := range tips {
		b.WriteString("- ")
		b.WriteString(tip)
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func print(cmd *cobra.Command, o *Opts, env output.Envelope) error {
	return output.Print(cmd.OutOrStdout(), fmtOut(o), env)
}

func fmtOut(o *Opts) string {
	if o.JSON {
		return "json"
	}
	if o.Format != "" {
		return strings.ToLower(o.Format)
	}
	return "table"
}
