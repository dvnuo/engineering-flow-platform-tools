package commands

import (
	"strings"

	"engineering-flow-platform-tools/internal/catalog"
	"engineering-flow-platform-tools/internal/clihelp"
	"engineering-flow-platform-tools/internal/output"
	"engineering-flow-platform-tools/internal/version"
	"github.com/spf13/cobra"
)

const product = "appd"

type Opts struct {
	Instance, Config, Format   string
	JSON, Verbose, DryRun, Yes bool
}

func NewRoot() *cobra.Command {
	cobra.EnableCommandSorting = false
	o := &Opts{Format: "table"}
	c := &cobra.Command{Use: product, SilenceErrors: true, SilenceUsage: true}
	c.PersistentFlags().StringVar(&o.Instance, "instance", "", "Configured appd instance name.")
	c.PersistentFlags().StringVar(&o.Config, "config", "", "Path to EFP config file.")
	c.PersistentFlags().BoolVar(&o.JSON, "json", false, "Print JSON envelope.")
	c.PersistentFlags().StringVar(&o.Format, "format", "table", "Output format: table|json|yaml.")
	c.PersistentFlags().BoolVar(&o.Verbose, "verbose", false, "Print non-secret diagnostics when available.")
	c.PersistentFlags().BoolVar(&o.DryRun, "dry-run", false, "Preview a request without sending it.")
	c.PersistentFlags().BoolVar(&o.Yes, "yes", false, "Confirm destructive or service-affecting operations.")
	c.AddCommand(commandsCmd(o), schemaCmd(o), helpLLMCmd(o), versionCmd(o))
	clihelp.ApplyCatalogHelp(c, clihelp.ProductHelp{
		Product: product,
		Binary:  product,
		Short:   "Inspect AppDynamics applications, tiers, nodes, metrics, snapshots, and events",
		Long: strings.TrimSpace(`appd is a terminal-invoked CLI for agents that need read-only, JSON-first access to the AppDynamics Controller REST API: applications, tiers, nodes, business transactions, metrics, transaction snapshots, health-rule violations, and events.

Configuration uses the shared EFP config from environment variables injected by managed runtimes (for example EFP_APPD_DEFAULT_INSTANCE, EFP_APPD_INSTANCES_0_BASE_URL) or the config file, normally ~/.efp/config.yaml (local), under the appd node.`),
		Examples: []string{
			`appd commands --json`,
			`appd help llm --json`,
			`appd version --json`,
		},
		Instructions: "copy cmd/appd/appd-cli.instructions.md to ~/.copilot/instructions/appd-cli.instructions.md.",
		Groups:       map[string]string{},
	})
	return c
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

func print(cmd *cobra.Command, o *Opts, e output.Envelope) error {
	return output.Print(cmd.OutOrStdout(), fmtOut(o), e)
}

func commandsCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "commands", RunE: func(cmd *cobra.Command, args []string) error {
		return print(cmd, o, output.Success("", map[string]any{"commands": catalog.CommandsFromCobra(product, cmd.Root())}))
	}}
}

func schemaCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "schema <command>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		schema, ok := catalog.SchemaFromCobra(product, args[0], cmd.Root())
		if !ok {
			return print(cmd, o, output.Failure("not_found", "command not found", "Run "+product+" commands --json to list command names.", 404))
		}
		return print(cmd, o, output.Success("", schema))
	}}
}

func helpLLMCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "help llm", RunE: func(cmd *cobra.Command, args []string) error {
		tips := []string{
			"For agents, --json is the default way to use every " + product + " command and subcommand.",
			"Run " + product + " commands --json, then " + product + " schema <command> --json before constructing a complex call.",
			"Use --instance when several instances are configured; without it the default instance is used.",
			"Inspect error.code and error.hint before retrying.",
		}
		return print(cmd, o, output.Success("", map[string]any{"tips": tips, "commands": catalog.CommandsFromCobra(product, cmd.Root())}))
	}}
}

func versionCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "version", RunE: func(cmd *cobra.Command, args []string) error {
		return print(cmd, o, output.Success("", map[string]any{"version": version.Version, "commit": version.Commit, "date": version.Date}))
	}}
}
