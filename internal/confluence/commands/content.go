package commands

import (
	"encoding/json"

	"engineering-flow-platform-tools/internal/httpclient"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func contentCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "content"}
	c.AddCommand(&cobra.Command{Use: "get <content-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return do(o, cmd, "GET", "content/"+args[0], nil, nil) }})
	c.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		q := map[string]string{}
		for _, k := range []string{"space", "type", "limit", "start", "expand"} {
			v, _ := cmd.Flags().GetString(k)
			if v != "" {
				if k == "space" {
					q["spaceKey"] = v
				} else {
					q[k] = v
				}
			}
		}
		return do(o, cmd, "GET", "content", q, nil)
	}})
	cl := c.Commands()[1]
	for _, k := range []string{"space", "type", "limit", "start", "expand"} {
		cl.Flags().String(k, "", "")
	}
	c.AddCommand(&cobra.Command{Use: "create", RunE: func(cmd *cobra.Command, args []string) error {
		t, _ := cmd.Flags().GetString("type")
		sp, _ := cmd.Flags().GetString("space")
		ti, _ := cmd.Flags().GetString("title")
		b, err := readBody(cmd)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", err.Error(), "", 400))
		}
		return do(o, cmd, "POST", "content", nil, map[string]any{"type": t, "title": ti, "space": map[string]string{"key": sp}, "body": confluenceBody(cmd, b)})
	}})
	c.Commands()[2].Flags().String("type", "page", "")
	c.Commands()[2].Flags().String("space", "", "")
	c.Commands()[2].Flags().String("title", "", "")
	c.Commands()[2].Flags().String("body", "", "")
	c.Commands()[2].Flags().String("body-file", "", "")
	c.Commands()[2].Flags().Bool("body-stdin", false, "")
	c.Commands()[2].Flags().String("body-format", "storage", "")
	c.AddCommand(&cobra.Command{Use: "update <content-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cx, err := loadCtx(o, "")
		if err != nil {
			return print(cmd, o, envelopeError(err, "config_error"))
		}
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
		b, err := readBody(cmd)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", err.Error(), "", 400))
		}
		return do(o, cmd, "PUT", "content/"+args[0], nil, map[string]any{"title": ti, "version": map[string]any{"number": v}, "body": confluenceBody(cmd, b)})
	}})
	c.Commands()[3].Flags().Int("version", 0, "")
	c.Commands()[3].Flags().String("title", "", "")
	c.Commands()[3].Flags().String("body", "", "")
	c.Commands()[3].Flags().String("body-file", "", "")
	c.Commands()[3].Flags().Bool("body-stdin", false, "")
	c.Commands()[3].Flags().String("body-format", "storage", "")
	c.AddCommand(&cobra.Command{Use: "delete <content-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		return do(o, cmd, "DELETE", "content/"+args[0], nil, nil)
	}})
	return c
}
