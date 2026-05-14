package commands

import (
	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func instanceCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "instance"}
	c.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		p, _ := config.ResolvePath(o.Config)
		cfg, err := config.Load(p)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		for i := range cfg.Confluence.Instances {
			cfg.Confluence.Instances[i].Auth = config.RedactAuth(cfg.Confluence.Instances[i].Auth)
		}
		return print(cmd, o, output.Success("", map[string]any{"instances": cfg.Confluence.Instances, "default_instance": cfg.Confluence.DefaultInstance}))
	}})
	c.AddCommand(&cobra.Command{Use: "get <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		p, _ := config.ResolvePath(o.Config)
		cfg, err := config.Load(p)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		for _, in := range cfg.Confluence.Instances {
			if in.Name == args[0] {
				in.Auth = config.RedactAuth(in.Auth)
				return print(cmd, o, output.Success(in.Name, in))
			}
		}
		return print(cmd, o, output.Failure("not_found", "instance not found", "", 404))
	}})
	c.AddCommand(&cobra.Command{Use: "add <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		p, _ := config.ResolvePath(o.Config)
		cfg, _ := config.Load(p)
		v := true
		base, _ := cmd.Flags().GetString("base-url")
		rest, _ := cmd.Flags().GetString("rest-path")
		if rest == "" {
			rest = "/rest/api"
		}
		auth, authErr := authFromFlags(cmd)
		if authErr != nil {
			return print(cmd, o, output.Failure("invalid_args", "missing required auth secret", "", 400))
		}
		cfg.Confluence.Instances = append(cfg.Confluence.Instances, config.InstanceConfig{Name: args[0], BaseURL: base, RESTPath: rest, VerifySSL: &v, Auth: auth})
		if d, _ := cmd.Flags().GetBool("default"); d {
			cfg.Confluence.DefaultInstance = args[0]
		}
		if err := config.Save(p, cfg); err != nil {
			return print(cmd, o, output.Failure("config_error", err.Error(), "", 500))
		}
		return print(cmd, o, output.Success(args[0], map[string]any{"added": true}))
	}})
	c.Commands()[2].Flags().String("base-url", "", "")
	c.Commands()[2].Flags().String("rest-path", "/rest/api", "")
	addAuthFlags(c.Commands()[2])
	c.Commands()[2].Flags().Bool("default", false, "")
	c.AddCommand(&cobra.Command{Use: "update <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		p, _ := config.ResolvePath(o.Config)
		cfg, err := config.Load(p)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		base, _ := cmd.Flags().GetString("base-url")
		rest, _ := cmd.Flags().GetString("rest-path")
		for i := range cfg.Confluence.Instances {
			if cfg.Confluence.Instances[i].Name == args[0] {
				if base != "" {
					cfg.Confluence.Instances[i].BaseURL = base
				}
				if rest != "" {
					cfg.Confluence.Instances[i].RESTPath = rest
				}
			}
		}
		_ = config.Save(p, cfg)
		return print(cmd, o, output.Success(args[0], map[string]any{"updated": true}))
	}})
	c.Commands()[3].Flags().String("base-url", "", "")
	c.Commands()[3].Flags().String("rest-path", "", "")
	c.AddCommand(&cobra.Command{Use: "remove <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		p, _ := config.ResolvePath(o.Config)
		cfg, err := config.Load(p)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		out := []config.InstanceConfig{}
		for _, i := range cfg.Confluence.Instances {
			if i.Name != args[0] {
				out = append(out, i)
			}
		}
		cfg.Confluence.Instances = out
		if cfg.Confluence.DefaultInstance == args[0] {
			cfg.Confluence.DefaultInstance = ""
		}
		_ = config.Save(p, cfg)
		return print(cmd, o, output.Success(args[0], map[string]any{"removed": true}))
	}})
	c.AddCommand(&cobra.Command{Use: "default [name]", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		p, _ := config.ResolvePath(o.Config)
		cfg, err := config.Load(p)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		if len(args) == 0 {
			return print(cmd, o, output.Success("", map[string]any{"default_instance": cfg.Confluence.DefaultInstance}))
		}
		cfg.Confluence.DefaultInstance = args[0]
		_ = config.Save(p, cfg)
		return print(cmd, o, output.Success(args[0], map[string]any{"default_instance": args[0]}))
	}})
	return c
}
