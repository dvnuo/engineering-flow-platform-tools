package commands

import (
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func webhookCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "webhook"}
	c.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error { return do(o, cmd, "GET", "webhooks", nil, nil) }})
	c.AddCommand(&cobra.Command{Use: "get <webhook-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return do(o, cmd, "GET", "webhooks/"+args[0], nil, nil) }})
	c.AddCommand(&cobra.Command{Use: "create", RunE: func(cmd *cobra.Command, args []string) error {
		n, _ := cmd.Flags().GetString("name")
		u, _ := cmd.Flags().GetString("url")
		e, _ := cmd.Flags().GetStringSlice("event")
		return do(o, cmd, "POST", "webhooks", nil, map[string]any{"name": n, "url": u, "events": e})
	}})
	c.Commands()[2].Flags().String("name", "", "")
	c.Commands()[2].Flags().String("url", "", "")
	c.Commands()[2].Flags().StringSlice("event", nil, "")
	c.AddCommand(&cobra.Command{Use: "delete <webhook-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		return do(o, cmd, "DELETE", "webhooks/"+args[0], nil, nil)
	}})
	return c
}

func longtaskCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "longtask"}
	c.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error { return do(o, cmd, "GET", "longtask", nil, nil) }})
	c.AddCommand(&cobra.Command{Use: "get <task-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return do(o, cmd, "GET", "longtask/"+args[0], nil, nil) }})
	return c
}
