package commands

import (
	"encoding/json"

	"engineering-flow-platform-tools/internal/httpclient"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func blogCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "blog"}
	c.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		sp, _ := cmd.Flags().GetString("space")
		return do(o, cmd, "GET", "content", map[string]string{"type": "blogpost", "spaceKey": sp}, nil)
	}})
	c.Commands()[0].Flags().String("space", "", "")
	c.AddCommand(&cobra.Command{Use: "get <blog-id-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return do(o, cmd, "GET", "content/"+args[0], nil, nil) }})
	c.AddCommand(&cobra.Command{Use: "create", RunE: func(cmd *cobra.Command, args []string) error {
		sp, _ := cmd.Flags().GetString("space")
		ti, _ := cmd.Flags().GetString("title")
		b := readBody(cmd)
		return do(o, cmd, "POST", "content", nil, map[string]any{"type": "blogpost", "title": ti, "space": map[string]string{"key": sp}, "body": confluenceBody(cmd, b)})
	}})
	c.Commands()[2].Flags().String("space", "", "")
	c.Commands()[2].Flags().String("title", "", "")
	c.Commands()[2].Flags().String("body", "", "")
	c.Commands()[2].Flags().String("body-file", "", "")
	c.Commands()[2].Flags().Bool("body-stdin", false, "")
	c.Commands()[2].Flags().String("body-format", "storage", "")
	c.AddCommand(&cobra.Command{Use: "update <blog-id-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cx, _ := loadCtx(o, "")
		v, _ := cmd.Flags().GetInt("version")
		if v == 0 && !o.DryRun {
			r, e := cx.client.Do(httpclient.Request{Method: "GET", Path: "content/" + args[0]})
			if e != nil {
				return print(cmd, o, output.Failure("server_error", "version fetch failed", "", 500))
			}
			defer r.Body.Close()
			var m map[string]any
			_ = json.NewDecoder(r.Body).Decode(&m)
			v = int(m["version"].(map[string]any)["number"].(float64)) + 1
		}
		ti, _ := cmd.Flags().GetString("title")
		b := readBody(cmd)
		return do(o, cmd, "PUT", "content/"+args[0], nil, map[string]any{"title": ti, "version": map[string]any{"number": v}, "body": confluenceBody(cmd, b)})
	}})
	c.Commands()[3].Flags().Int("version", 0, "")
	c.Commands()[3].Flags().String("title", "", "")
	c.Commands()[3].Flags().String("body", "", "")
	c.Commands()[3].Flags().String("body-file", "", "")
	c.Commands()[3].Flags().Bool("body-stdin", false, "")
	c.Commands()[3].Flags().String("body-format", "storage", "")
	c.AddCommand(&cobra.Command{Use: "delete <blog-id-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		return do(o, cmd, "DELETE", "content/"+args[0], nil, nil)
	}})
	return c
}
