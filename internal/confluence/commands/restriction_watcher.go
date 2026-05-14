package commands

import (
	"strings"

	"engineering-flow-platform-tools/internal/httpclient"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func restrictionCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "restriction"}
	c.AddCommand(&cobra.Command{Use: "list <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return do(o, cmd, "GET", "content/"+args[0]+"/restriction", nil, nil)
	}})
	c.AddCommand(&cobra.Command{Use: "add <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		op, _ := cmd.Flags().GetString("operation")
		users, _ := cmd.Flags().GetStringSlice("user")
		groups, _ := cmd.Flags().GetStringSlice("group")
		body := map[string]any{"restrictions": map[string]any{"user": users, "group": groups}}
		return do(o, cmd, "POST", "content/"+args[0]+"/restriction/byOperation/"+op, nil, body)
	}})
	c.Commands()[1].Flags().String("operation", "read", "")
	c.Commands()[1].Flags().StringSlice("user", nil, "")
	c.Commands()[1].Flags().StringSlice("group", nil, "")
	c.AddCommand(&cobra.Command{Use: "delete <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		op, _ := cmd.Flags().GetString("operation")
		return do(o, cmd, "DELETE", "content/"+args[0]+"/restriction/byOperation/"+op, nil, nil)
	}})
	c.Commands()[2].Flags().String("operation", "read", "")
	c.AddCommand(&cobra.Command{Use: "watcher-list <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if o.DryRun {
			return do(o, cmd, "GET", "content/"+args[0]+"/notification/child-created", nil, nil)
		}
		cx, _ := loadCtx(o, "")
		resp, err := cx.client.Do(httpclient.Request{Method: "GET", Path: "content/" + args[0] + "/notification/child-created"})
		if err != nil {
			if strings.Contains(err.Error(), "not_found") {
				return print(cmd, o, output.Failure("not_supported", "watcher endpoint not supported", "", 404))
			}
			return print(cmd, o, output.Failure("server_error", err.Error(), "", 500))
		}
		defer resp.Body.Close()
		return print(cmd, o, output.Success(cx.inst.Name, map[string]any{"ok": true}))
	}})
	return c
}
