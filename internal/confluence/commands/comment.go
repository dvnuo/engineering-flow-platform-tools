package commands

import (
	"encoding/json"

	"engineering-flow-platform-tools/internal/httpclient"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func commentCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "comment"}
	c.AddCommand(&cobra.Command{Use: "get <comment-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return do(o, cmd, "GET", "content/"+args[0], nil, nil) }})
	c.AddCommand(&cobra.Command{Use: "update <comment-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		b, err := readBody(cmd)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", err.Error(), "", 400))
		}
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
		payload := map[string]any{"version": map[string]any{"number": v}, "type": "comment", "body": confluenceBody(cmd, b)}
		return do(o, cmd, "PUT", "content/"+args[0], nil, payload)
	}})
	c.Commands()[1].Flags().String("body", "", "")
	c.Commands()[1].Flags().String("body-file", "", "")
	c.Commands()[1].Flags().Bool("body-stdin", false, "")
	c.Commands()[1].Flags().String("body-format", "storage", "")
	c.Commands()[1].Flags().Int("version", 0, "")
	c.AddCommand(&cobra.Command{Use: "delete <comment-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		return do(o, cmd, "DELETE", "content/"+args[0], nil, nil)
	}})
	return c
}
