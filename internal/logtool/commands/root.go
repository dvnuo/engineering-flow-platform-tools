package commands

import (
	"errors"
	"fmt"
	"strings"

	"engineering-flow-platform-tools/internal/catalog"
	"engineering-flow-platform-tools/internal/clihelp"
	"engineering-flow-platform-tools/internal/logtool"
	"engineering-flow-platform-tools/internal/output"
	"engineering-flow-platform-tools/internal/version"
	"github.com/spf13/cobra"
)

type Opts struct {
	Format        string
	JSON, Verbose bool
}

func NewRoot() *cobra.Command {
	cobra.EnableCommandSorting = false
	o := &Opts{Format: "table"}
	c := &cobra.Command{
		Use:           "log",
		Short:         "Analyze large local logs with bounded JSON outputs for agents",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	c.PersistentFlags().BoolVar(&o.JSON, "json", false, "Print the stable JSON envelope.")
	c.PersistentFlags().StringVar(&o.Format, "format", "table", "Output format: table, json, or yaml.")
	c.PersistentFlags().BoolVar(&o.Verbose, "verbose", false, "Print non-secret diagnostics to stderr when available.")
	c.AddCommand(
		versionCmd(o),
		commandsCmd(o),
		schemaCmd(o),
		helpLLMCmd(o),
		analyzeCmd(o),
		profileCmd(o),
		templatesCmd(o),
		entriesCmd(o),
		searchCmd(o),
		windowCmd(o),
		extractCmd(o),
	)
	clihelp.ApplyCatalogHelp(c, clihelp.ProductHelp{
		Product: "log",
		Binary:  "log",
		Short:   "Analyze large local logs with bounded JSON outputs for agents",
		Long: strings.TrimSpace(`log is a local-only CLI for agents that need to analyze large files, directories, or globs without dumping raw logs into the conversation.

Run log analyze first to create a small run directory containing a manifest, redacted entry index, and templates. Then use profile, templates, search, window, and extract for bounded evidence retrieval.`),
		Examples: []string{
			`log analyze --source ./logs/app.log --run ./.log-runs/run_001 --json`,
			`log search --run ./.log-runs/run_001 --query "ERROR OR timeout" --json`,
			`log window --run ./.log-runs/run_001 --entry-id entry_000001 --before 50 --after 50 --json`,
		},
	})
	return c
}

func versionCmd(o *Opts) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print log CLI version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			return print(cmd, o, output.Success("", map[string]any{"version": version.Version, "commit": version.Commit, "date": version.Date}))
		},
	}
}

func commandsCmd(o *Opts) *cobra.Command {
	return &cobra.Command{
		Use:   "commands",
		Short: "List agent-facing log commands",
		RunE: func(cmd *cobra.Command, args []string) error {
			return print(cmd, o, output.Success("", map[string]any{"product": "log", "commands": catalog.CommandsFromCobra("log", cmd.Root())}))
		},
	}
}

func schemaCmd(o *Opts) *cobra.Command {
	return &cobra.Command{
		Use:   "schema <command>",
		Short: "Show argument and flag schema for a log command",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			schema, ok := catalog.SchemaFromCobra("log", args[0], cmd.Root())
			if !ok {
				return print(cmd, o, output.Failure("not_found", "command not found", "Run log commands --json to list command names.", 404))
			}
			return print(cmd, o, output.Success("", schema))
		},
	}
}

func helpLLMCmd(o *Opts) *cobra.Command {
	return &cobra.Command{
		Use:   "help llm",
		Short: "Show log CLI usage guidance for LLM agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			tips := logLLMTips()
			if fmtOut(o) == "json" {
				return print(cmd, o, output.Success("", map[string]any{"tips": tips, "commands": catalog.Commands("log")}))
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), logLLMMarkdown(tips))
			return err
		},
	}
}

func logLLMTips() []string {
	return []string{
		"For agents, default every log command to --json so results and failures use the stable ok/data/error envelope.",
		"Do not cat huge logs into the conversation; run log analyze --source <file|dir|glob> --run <run-dir> --json first.",
		"Use log profile and log templates to understand volume, levels, and repeated patterns before drilling into individual evidence.",
		"Use log search for bounded matches and pass next_cursor to continue; never request all entries at once.",
		"Use log window to retrieve redacted before/after context from the original source file when you need evidence.",
		"Prefer log window --entry-id; log window --file --line is limited to files already recorded in the run manifest.",
		"Use log extract --kind stacktrace or --kind error-signature to find repeated failures.",
		"Run directories contain only manifest.json, entries.jsonl, and templates.json with redacted previews; original logs must remain available for window.",
		"Secrets such as Authorization bearer tokens, passwords, API keys, AWS keys, private keys, and emails are redacted from outputs.",
		"P0 supports only local files, directories, and globs; it does not call LLMs and does not connect to Loki, ClickHouse, Kubernetes, Docker, journalctl, or remote backends.",
		"For durable agent instructions, use cmd/log/log-cli.instructions.md.",
	}
}

func logLLMMarkdown(tips []string) string {
	var b strings.Builder
	b.WriteString("# log CLI usage for agents\n\n")
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

func printErr(cmd *cobra.Command, o *Opts, err error) error {
	var toolErr *logtool.ToolError
	if errors.As(err, &toolErr) {
		return print(cmd, o, output.Failure(toolErr.Code, toolErr.Message, toolErr.Hint, toolErr.Status))
	}
	return print(cmd, o, output.Failure("server_error", logtool.RedactError(err.Error()), "Retry with --json and inspect error.code/error.hint.", 500))
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
