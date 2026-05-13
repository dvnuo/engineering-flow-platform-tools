package cli

import (
	"errors"
	"os"
	"strings"

	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/llm"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

type RootOptions struct {
	Instance, Config, Format string
	JSON, Verbose            bool
}
type BuilderInput struct {
	Use, Short, Long string
	Registry         *llm.Registry
}

func NewRootCommand(in BuilderInput) *cobra.Command {
	opts := &RootOptions{Format: "table"}
	cmd := &cobra.Command{Use: in.Use, Short: in.Short, Long: in.Long, SilenceUsage: true, SilenceErrors: true}
	cmd.PersistentFlags().StringVar(&opts.Instance, "instance", "", "Instance name")
	cmd.PersistentFlags().StringVar(&opts.Config, "config", "", "Config path")
	cmd.PersistentFlags().BoolVar(&opts.JSON, "json", false, "Output JSON envelope")
	cmd.PersistentFlags().StringVar(&opts.Format, "format", "table", "Output format: table|json|yaml")
	cmd.PersistentFlags().BoolVar(&opts.Verbose, "verbose", false, "Verbose logging")

	cmd.AddCommand(&cobra.Command{Use: "commands", RunE: func(c *cobra.Command, args []string) error {
		items := in.Registry.List(in.Use)
		names := []string{}
		for _, it := range items {
			names = append(names, it.Name)
		}
		return output.Print(c.OutOrStdout(), decide(opts), output.Success(opts.Instance, map[string]interface{}{"product": in.Use, "commands": names}))
	}})
	cmd.AddCommand(&cobra.Command{Use: "schema <command>", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		m, ok := in.Registry.Get(args[0])
		if !ok {
			return output.Print(c.OutOrStdout(), decide(opts), output.Failure("not_found", "command not found", "run commands", 404))
		}
		return output.Print(c.OutOrStdout(), decide(opts), output.Success(opts.Instance, llm.SchemaFor(m)))
	}})
	cmd.AddCommand(&cobra.Command{Use: "help llm", RunE: func(c *cobra.Command, args []string) error {
		return output.Print(c.OutOrStdout(), decide(opts), output.Success(opts.Instance, llm.Help(in.Use)))
	}})
	cmd.AddCommand(instanceCmd(in.Use, opts))
	cmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		_ = output.Print(os.Stderr, decide(opts), output.Failure("invalid_flag", err.Error(), "use --help", 400))
		return errors.New("invalid_flag")
	})
	return cmd
}

func instanceCmd(product string, opts *RootOptions) *cobra.Command {
	c := &cobra.Command{Use: "instance"}
	c.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		p, err := config.ResolvePath(opts.Config)
		if err != nil {
			return output.Print(cmd.OutOrStdout(), decide(opts), output.Failure("config_error", err.Error(), "", 400))
		}
		cfg, err := config.Load(p)
		if err != nil {
			return output.Print(cmd.OutOrStdout(), decide(opts), output.Failure("config_missing", err.Error(), "create config file", 404))
		}
		var instances []config.InstanceConfig
		if product == "jira" {
			instances = cfg.Jira.Instances
		} else {
			instances = cfg.Confluence.Instances
		}
		if len(instances) == 0 {
			return output.Print(cmd.OutOrStdout(), decide(opts), output.Failure("no_instance_configured", "no instances configured", "add an instance", 404))
		}
		return output.Print(cmd.OutOrStdout(), decide(opts), output.Success(opts.Instance, map[string]interface{}{"instances": config.RedactRoot(cfg)}))
	}})
	return c
}
func decide(o *RootOptions) string {
	if o.JSON {
		return "json"
	}
	f := strings.ToLower(o.Format)
	if f == "" {
		return "table"
	}
	return f
}
