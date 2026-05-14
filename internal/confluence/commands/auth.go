package commands

import (
	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func authCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "auth"}
	c.AddCommand(&cobra.Command{Use: "login", RunE: func(cmd *cobra.Command, args []string) error {
		p, _ := config.ResolvePath(o.Config)
		cfg, err := config.Load(p)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		u, _ := cmd.Flags().GetString("username")
		t, _ := cmd.Flags().GetString("auth-type")
		target := cfg.Confluence.DefaultInstance
		if o.Instance != "" {
			target = o.Instance
		}
		for i := range cfg.Confluence.Instances {
			if cfg.Confluence.Instances[i].Name == target {
				cfg.Confluence.Instances[i].Auth.Username = u
				cfg.Confluence.Instances[i].Auth.Type = t
			}
		}
		_ = config.Save(p, cfg)
		return print(cmd, o, output.Success(target, map[string]any{"logged_in": true}))
	}})
	c.Commands()[0].Flags().String("username", "", "")
	c.Commands()[0].Flags().String("auth-type", "", "")
	c.AddCommand(&cobra.Command{Use: "logout", RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		p, _ := config.ResolvePath(o.Config)
		cfg, err := config.Load(p)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		target := cfg.Confluence.DefaultInstance
		if o.Instance != "" {
			target = o.Instance
		}
		for i := range cfg.Confluence.Instances {
			if cfg.Confluence.Instances[i].Name == target {
				cfg.Confluence.Instances[i].Auth = config.AuthConfig{}
			}
		}
		_ = config.Save(p, cfg)
		return print(cmd, o, output.Success(target, map[string]any{"logged_out": true}))
	}})
	c.AddCommand(&cobra.Command{Use: "test", RunE: func(cmd *cobra.Command, args []string) error { return do(o, cmd, "GET", "user/current", nil, nil) }})
	return c
}

func myselfCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "myself", RunE: func(cmd *cobra.Command, args []string) error { return do(o, cmd, "GET", "user/current", nil, nil) }}
}

func serverInfoCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "server-info", RunE: func(cmd *cobra.Command, args []string) error {
		return do(o, cmd, "GET", "settings/systemInfo", nil, nil)
	}}
}

func resolveCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "resolve-url <url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cx, err := loadCtx(o, args[0])
		if err != nil {
			return print(cmd, o, output.Failure("instance_required", err.Error(), "", 400))
		}
		return print(cmd, o, output.Success(cx.inst.Name, map[string]any{"url": args[0]}))
	}}
}
