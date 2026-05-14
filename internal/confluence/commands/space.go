package commands

import (
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func spaceCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "space"}
	c.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error { return do(o, cmd, "GET", "space", nil, nil) }})
	c.AddCommand(&cobra.Command{Use: "get <space-key>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return do(o, cmd, "GET", "space/"+args[0], nil, nil) }})
	c.AddCommand(&cobra.Command{Use: "content <space-key>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return do(o, cmd, "GET", "space/"+args[0]+"/content", nil, nil)
	}})
	c.AddCommand(&cobra.Command{Use: "pages <space-key>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return do(o, cmd, "GET", "space/"+args[0]+"/content/page", nil, nil)
	}})
	c.AddCommand(&cobra.Command{Use: "blogs <space-key>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return do(o, cmd, "GET", "space/"+args[0]+"/content/blog", nil, nil)
	}})
	c.AddCommand(&cobra.Command{Use: "labels <space-key>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return do(o, cmd, "GET", "space/"+args[0]+"/label", nil, nil)
	}})
	c.AddCommand(&cobra.Command{Use: "create", RunE: func(cmd *cobra.Command, args []string) error {
		k, _ := cmd.Flags().GetString("key")
		n, _ := cmd.Flags().GetString("name")
		d, _ := cmd.Flags().GetString("description")
		return do(o, cmd, "POST", "space", nil, map[string]any{"key": k, "name": n, "description": map[string]any{"plain": map[string]any{"value": d}}})
	}})
	c.Commands()[len(c.Commands())-1].Flags().String("key", "", "")
	c.Commands()[len(c.Commands())-1].Flags().String("name", "", "")
	c.Commands()[len(c.Commands())-1].Flags().String("description", "", "")
	c.AddCommand(&cobra.Command{Use: "update <space-key>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		n, _ := cmd.Flags().GetString("name")
		return do(o, cmd, "PUT", "space/"+args[0], nil, map[string]any{"name": n})
	}})
	c.Commands()[len(c.Commands())-1].Flags().String("name", "", "")
	c.AddCommand(&cobra.Command{Use: "delete <space-key>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		return do(o, cmd, "DELETE", "space/"+args[0], nil, nil)
	}})
	c.AddCommand(&cobra.Command{Use: "watchers <space-key>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return do(o, cmd, "GET", "space/"+args[0]+"/watch", nil, nil)
	}})
	c.AddCommand(&cobra.Command{Use: "permission list <space-key>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return do(o, cmd, "GET", "space/"+args[0]+"/permission", nil, nil)
	}})
	sp := &cobra.Command{Use: "property"}
	sp.AddCommand(&cobra.Command{Use: "list <space-key>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return do(o, cmd, "GET", "space/"+args[0]+"/property", nil, nil)
	}})
	sp.AddCommand(&cobra.Command{Use: "get <space-key> <key>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		return do(o, cmd, "GET", "space/"+args[0]+"/property/"+args[1], nil, nil)
	}})
	sp.AddCommand(&cobra.Command{Use: "set <space-key> <key>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		return do(o, cmd, "PUT", "space/"+args[0]+"/property/"+args[1], nil, map[string]any{"key": args[1], "value": readBody(cmd)})
	}})
	sp.Commands()[2].Flags().String("body", "", "")
	sp.Commands()[2].Flags().String("body-file", "", "")
	sp.Commands()[2].Flags().Bool("body-stdin", false, "")
	sp.AddCommand(&cobra.Command{Use: "delete <space-key> <key>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		return do(o, cmd, "DELETE", "space/"+args[0]+"/property/"+args[1], nil, nil)
	}})
	c.AddCommand(sp)
	return c
}
