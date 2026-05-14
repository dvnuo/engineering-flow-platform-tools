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
		return do(o, cmd, "GET", "search", map[string]string{"cql": cql}, nil)
	}}
	c.Flags().String("cql", "", "")
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
		return do(o, cmd, "GET", "search", map[string]string{"cql": strings.Join(parts, " AND ")}, nil)
	}})
	cc := c.Commands()[0]
	cc.Flags().String("text", "", "")
	cc.Flags().String("space", "", "")
	cc.Flags().String("type", "", "")
	c.AddCommand(&cobra.Command{Use: "user", RunE: func(cmd *cobra.Command, args []string) error {
		q, _ := cmd.Flags().GetString("query")
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
		return do(o, cmd, "GET", "search", map[string]string{"cql": q}, nil)
	}}
	c.Flags().String("query", "", "")
	return c
}
