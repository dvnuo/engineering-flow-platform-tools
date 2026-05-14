package commands

import (
	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/httpclient"
	"engineering-flow-platform-tools/internal/jira"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

type Opts struct {
	Instance, Config  string
	JSON, DryRun, Yes bool
}

func NewRoot() *cobra.Command {
	o := &Opts{}
	c := &cobra.Command{Use: "confluence", SilenceErrors: true, SilenceUsage: true}
	c.PersistentFlags().StringVar(&o.Instance, "instance", "", "")
	c.PersistentFlags().StringVar(&o.Config, "config", "", "")
	c.PersistentFlags().BoolVar(&o.JSON, "json", false, "")
	c.PersistentFlags().BoolVar(&o.DryRun, "dry-run", false, "")
	c.PersistentFlags().BoolVar(&o.Yes, "yes", false, "")
	c.AddCommand(schemaCmd(), pageCmd(o), contentCmd(o), blogCmd(o), apiCmd(o))
	return c
}
func fmtOut(o *Opts) string {
	if o.JSON {
		return "json"
	}
	return "table"
}
func print(cmd *cobra.Command, o *Opts, env output.Envelope) error {
	return output.Print(cmd.OutOrStdout(), fmtOut(o), env)
}
func loadCfg(o *Opts) (config.RootConfig, error) {
	p, _ := config.ResolvePath(o.Config)
	return config.Load(p)
}

func schemaCmd() *cobra.Command {
	return &cobra.Command{Use: "schema <command>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		req := []string{}
		switch args[0] {
		case "page.create", "content.create", "blog.create":
			req = []string{"space", "title", "body"}
		case "page.update", "content.update", "blog.update":
			req = []string{"title|body"}
		}
		return output.Print(cmd.OutOrStdout(), "json", output.Success("", map[string]any{"command": args[0], "required": req}))
	}}
}

func pageCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "page"}
	c.AddCommand(&cobra.Command{Use: "create", RunE: func(cmd *cobra.Command, args []string) error {
		space := mustS(cmd, "space")
		title := mustS(cmd, "title")
		body := mustS(cmd, "body")
		if space == "" || title == "" || body == "" {
			return print(cmd, o, output.Failure("invalid_args", "--space --title --body required", "", 400))
		}
		return confluencePost(o, cmd, "content", map[string]any{"type": "page", "title": title, "space": map[string]string{"key": space}, "body": body})
	}})
	c.Commands()[0].Flags().String("space", "", "")
	c.Commands()[0].Flags().String("title", "", "")
	c.Commands()[0].Flags().String("body", "", "")
	c.AddCommand(&cobra.Command{Use: "update <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		title := mustS(cmd, "title")
		body := mustS(cmd, "body")
		if title == "" && body == "" {
			return print(cmd, o, output.Failure("invalid_args", "at least one field required", "", 400))
		}
		return confluencePut(o, cmd, "content/"+args[0], map[string]any{"title": title, "body": body})
	}})
	c.Commands()[1].Flags().String("title", "", "")
	c.Commands()[1].Flags().String("body", "", "")
	c.AddCommand(&cobra.Command{Use: "delete <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		return confluenceDelete(o, cmd, "content/"+args[0])
	}})
	return c
}

func contentCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "content"}
	c.AddCommand(&cobra.Command{Use: "create", RunE: func(cmd *cobra.Command, args []string) error {
		title := mustS(cmd, "title")
		body := mustS(cmd, "body")
		if title == "" || body == "" {
			return print(cmd, o, output.Failure("invalid_args", "--title --body required", "", 400))
		}
		return confluencePost(o, cmd, "content", map[string]any{"title": title, "body": body})
	}})
	c.Commands()[0].Flags().String("title", "", "")
	c.Commands()[0].Flags().String("body", "", "")
	c.AddCommand(&cobra.Command{Use: "update <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		title := mustS(cmd, "title")
		body := mustS(cmd, "body")
		if title == "" && body == "" {
			return print(cmd, o, output.Failure("invalid_args", "at least one field required", "", 400))
		}
		return confluencePut(o, cmd, "content/"+args[0], map[string]any{"title": title, "body": body})
	}})
	c.Commands()[1].Flags().String("title", "", "")
	c.Commands()[1].Flags().String("body", "", "")
	c.AddCommand(&cobra.Command{Use: "delete <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		return confluenceDelete(o, cmd, "content/"+args[0])
	}})
	return c
}

func blogCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "blog"}
	c.AddCommand(&cobra.Command{Use: "create", RunE: func(cmd *cobra.Command, args []string) error {
		space := mustS(cmd, "space")
		title := mustS(cmd, "title")
		body := mustS(cmd, "body")
		if space == "" || title == "" || body == "" {
			return print(cmd, o, output.Failure("invalid_args", "--space --title --body required", "", 400))
		}
		return confluencePost(o, cmd, "content", map[string]any{"type": "blogpost", "title": title, "space": map[string]string{"key": space}, "body": body})
	}})
	c.Commands()[0].Flags().String("space", "", "")
	c.Commands()[0].Flags().String("title", "", "")
	c.Commands()[0].Flags().String("body", "", "")
	c.AddCommand(&cobra.Command{Use: "update <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		title := mustS(cmd, "title")
		body := mustS(cmd, "body")
		if title == "" && body == "" {
			return print(cmd, o, output.Failure("invalid_args", "at least one field required", "", 400))
		}
		return confluencePut(o, cmd, "content/"+args[0], map[string]any{"title": title, "body": body})
	}})
	c.Commands()[1].Flags().String("title", "", "")
	c.Commands()[1].Flags().String("body", "", "")
	c.AddCommand(&cobra.Command{Use: "delete <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		return confluenceDelete(o, cmd, "content/"+args[0])
	}})
	return c
}

func apiCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "api"}
	for _, m := range []string{"get", "post", "put", "delete"} {
		mm := m
		method := map[string]string{"get": "GET", "post": "POST", "put": "PUT", "delete": "DELETE"}[m]
		c.AddCommand(&cobra.Command{Use: mm + " <path>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			if method == "DELETE" && !o.Yes {
				return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
			}
			return confluenceReq(o, cmd, method, args[0], nil)
		}})
	}
	return c
}

func confluenceReq(o *Opts, cmd *cobra.Command, method, path string, body interface{}) error {
	cfg, err := loadCfg(o)
	if err != nil {
		return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
	}
	ctx, err := jira.NewContext(cfg, o.Instance, "", o.DryRun)
	if err != nil {
		return print(cmd, o, output.Failure(err.Error(), err.Error(), "", 400))
	}
	reqPath := path
	if o.DryRun {
		return print(cmd, o, output.Success(ctx.Instance, map[string]any{"dry_run": true, "method": method, "path": reqPath, "body": body}))
	}
	_, err = ctx.Client.Do(httpclient.Request{Method: method, Path: reqPath, JSONBody: body})
	if err != nil {
		return print(cmd, o, output.Failure("server_error", err.Error(), "", 500))
	}
	return print(cmd, o, output.Success(ctx.Instance, map[string]any{"ok": true}))
}
func confluencePost(o *Opts, cmd *cobra.Command, path string, body interface{}) error {
	return confluenceReq(o, cmd, "POST", path, body)
}
func confluencePut(o *Opts, cmd *cobra.Command, path string, body interface{}) error {
	return confluenceReq(o, cmd, "PUT", path, body)
}
func confluenceDelete(o *Opts, cmd *cobra.Command, path string) error {
	return confluenceReq(o, cmd, "DELETE", path, nil)
}
func mustS(cmd *cobra.Command, n string) string { v, _ := cmd.Flags().GetString(n); return v }
