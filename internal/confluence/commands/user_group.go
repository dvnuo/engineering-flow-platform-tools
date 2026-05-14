package commands

import "github.com/spf13/cobra"

func userGroupCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "user"}
	c.AddCommand(&cobra.Command{Use: "get", RunE: func(cmd *cobra.Command, args []string) error {
		q := map[string]string{}
		u, _ := cmd.Flags().GetString("username")
		k, _ := cmd.Flags().GetString("user-key")
		if u != "" {
			q["username"] = u
		}
		if k != "" {
			q["key"] = k
		}
		return do(o, cmd, "GET", "user", q, nil)
	}})
	c.Commands()[0].Flags().String("username", "", "")
	c.Commands()[0].Flags().String("user-key", "", "")
	c.AddCommand(&cobra.Command{Use: "search", RunE: func(cmd *cobra.Command, args []string) error {
		q, _ := cmd.Flags().GetString("query")
		return do(o, cmd, "GET", "user/search", map[string]string{"query": q}, nil)
	}})
	c.Commands()[1].Flags().String("query", "", "")
	return c
}

func groupCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "group"}
	c.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error { return do(o, cmd, "GET", "group", nil, nil) }})
	c.AddCommand(&cobra.Command{Use: "get <group-name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return do(o, cmd, "GET", "group/"+args[0], nil, nil) }})
	c.AddCommand(&cobra.Command{Use: "members <group-name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return do(o, cmd, "GET", "group/"+args[0]+"/member", nil, nil)
	}})
	return c
}
