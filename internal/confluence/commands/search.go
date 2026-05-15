package commands

import (
	"strings"

	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func searchCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "search", RunE: func(cmd *cobra.Command, args []string) error {
		cql, _ := cmd.Flags().GetString("cql")
		if cql == "" {
			return print(cmd, o, output.Failure("invalid_args", "--cql required", "", 400))
		}
		return do(o, cmd, "GET", "search", searchQuery(cmd, cql), nil)
	}}
	c.Flags().String("cql", "", "")
	c.Flags().String("limit", "", "")
	c.Flags().String("start", "", "")
	c.Flags().String("expand", "", "")
	c.AddCommand(&cobra.Command{Use: "content", RunE: func(cmd *cobra.Command, args []string) error {
		t, _ := cmd.Flags().GetString("text")
		s, _ := cmd.Flags().GetString("space")
		ty, _ := cmd.Flags().GetString("type")
		parts := []string{}
		if t != "" {
			parts = append(parts, "text ~ \""+t+"\"")
		}
		if s != "" {
			parts = append(parts, "space = \""+s+"\"")
		}
		if ty != "" {
			parts = append(parts, "type = \""+ty+"\"")
		}
		return do(o, cmd, "GET", "search", searchQuery(cmd, strings.Join(parts, " AND ")), nil)
	}})
	cc := c.Commands()[0]
	cc.Flags().String("text", "", "")
	cc.Flags().String("space", "", "")
	cc.Flags().String("type", "", "")
	cc.Flags().String("limit", "", "")
	cc.Flags().String("start", "", "")
	cc.Flags().String("expand", "", "")
	c.AddCommand(&cobra.Command{Use: "user", RunE: func(cmd *cobra.Command, args []string) error {
		q, _ := cmd.Flags().GetString("query")
		if q == "" {
			return print(cmd, o, output.Failure("invalid_args", "--query required", "", 400))
		}
		return do(o, cmd, "GET", "user/search", map[string]string{"query": q}, nil)
	}})
	c.Commands()[1].Flags().String("query", "", "")
	return c
}

func cqlCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "cql", RunE: func(cmd *cobra.Command, args []string) error {
		q, _ := cmd.Flags().GetString("query")
		if q == "" {
			return print(cmd, o, output.Failure("invalid_args", "--query required", "", 400))
		}
		return do(o, cmd, "GET", "search", searchQuery(cmd, q), nil)
	}}
	c.Flags().String("query", "", "")
	c.Flags().String("limit", "", "")
	c.Flags().String("start", "", "")
	c.Flags().String("expand", "", "")
	return c
}

func searchQuery(cmd *cobra.Command, cql string) map[string]string {
	q := map[string]string{"cql": cql}
	for _, k := range []string{"limit", "start", "expand"} {
		if v, _ := cmd.Flags().GetString(k); v != "" {
			q[k] = v
		}
	}
	return q
}
