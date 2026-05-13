package cli

import (
	"fmt"
	"os"
	"strings"

	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

type RootOptions struct {
	Instance string
	Config   string
	JSON     bool
	Format   string
	Verbose  bool
}

type BuilderInput struct {
	Use       string
	Short     string
	Long      string
	Commands  []string
	SchemaFor func(string) SchemaDoc
}

func NewRootCommand(in BuilderInput) *cobra.Command {
	opts := &RootOptions{Format: "table"}
	cmd := &cobra.Command{Use: in.Use, Short: in.Short, Long: in.Long}
	cmd.PersistentFlags().StringVar(&opts.Instance, "instance", "", "Instance name")
	cmd.PersistentFlags().StringVar(&opts.Config, "config", "", "Config path")
	cmd.PersistentFlags().BoolVar(&opts.JSON, "json", false, "Output JSON envelope")
	cmd.PersistentFlags().StringVar(&opts.Format, "format", "table", "Output format: table|json|yaml")
	cmd.PersistentFlags().BoolVar(&opts.Verbose, "verbose", false, "Verbose logging")

	cmd.AddCommand(&cobra.Command{
		Use:   "commands",
		Short: "List supported commands",
		RunE: func(cmd *cobra.Command, args []string) error {
			format := decideFormat(opts)
			env := output.Success(opts.Instance, CommandRegistry{Product: in.Use, Commands: in.Commands})
			return output.Print(cmd.OutOrStdout(), format, env)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "schema <command>",
		Short: "Show schema for a command",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format := decideFormat(opts)
			schema := in.SchemaFor(args[0])
			env := output.Success(opts.Instance, schema)
			return output.Print(cmd.OutOrStdout(), format, env)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "help llm",
		Short: "LLM usage tips",
		RunE: func(cmd *cobra.Command, args []string) error {
			tips := []string{"Always use --json for machine parsing", "Prefer --instance when multiple profiles exist", "Use --dry-run before write operations", "Use --yes for deletes", "Check error.code and error.hint on failure"}
			env := output.Success(opts.Instance, map[string]interface{}{"tips": tips})
			return output.Print(cmd.OutOrStdout(), decideFormat(opts), env)
		},
	})

	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		env := output.Failure("invalid_flag", err.Error(), "use --help")
		_ = output.Print(os.Stderr, decideFormat(opts), env)
		return fmt.Errorf("invalid_flag")
	})
	return cmd
}

func decideFormat(opts *RootOptions) string {
	if opts.JSON {
		return "json"
	}
	f := strings.ToLower(opts.Format)
	if f == "" {
		return "table"
	}
	return f
}
