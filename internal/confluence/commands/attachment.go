package commands

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"engineering-flow-platform-tools/internal/httpclient"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func attachmentCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "attachment"}
	c.AddCommand(&cobra.Command{Use: "get <attachment-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return do(o, cmd, "GET", "content/"+args[0], nil, nil) }})
	c.AddCommand(&cobra.Command{Use: "upload <page-id> <file>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		if o.DryRun {
			return print(cmd, o, output.Success("", map[string]any{"dry_run": true, "method": "POST", "path": "content/" + args[0] + "/child/attachment"}))
		}
		cx, _ := loadCtx(o, "")
		f, err := os.Open(args[1])
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", err.Error(), "", 400))
		}
		defer f.Close()
		resp, err := cx.client.Do(httpclient.Request{Method: "POST", Path: "content/" + args[0] + "/child/attachment", Multipart: f, MultipartField: "file", MultipartName: filepath.Base(args[1]), Headers: map[string]string{"X-Atlassian-Token": "no-check"}})
		if err != nil {
			return print(cmd, o, output.Failure("server_error", err.Error(), "", 500))
		}
		defer resp.Body.Close()
		return print(cmd, o, output.Success(cx.inst.Name, map[string]any{"uploaded": true}))
	}})
	c.AddCommand(&cobra.Command{Use: "update <page-id> <attachment-id> <file>", Args: cobra.ExactArgs(3), RunE: func(cmd *cobra.Command, args []string) error {
		if o.DryRun {
			return print(cmd, o, output.Success("", map[string]any{"dry_run": true, "method": "POST", "path": "content/" + args[0] + "/child/attachment/" + args[1] + "/data"}))
		}
		cx, _ := loadCtx(o, "")
		f, err := os.Open(args[2])
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", err.Error(), "", 400))
		}
		defer f.Close()
		resp, err := cx.client.Do(httpclient.Request{Method: "POST", Path: "content/" + args[0] + "/child/attachment/" + args[1] + "/data", Multipart: f, MultipartField: "file", MultipartName: filepath.Base(args[2]), Headers: map[string]string{"X-Atlassian-Token": "no-check"}})
		if err != nil {
			return print(cmd, o, output.Failure("server_error", err.Error(), "", 500))
		}
		defer resp.Body.Close()
		return print(cmd, o, output.Success(cx.inst.Name, map[string]any{"updated": true}))
	}})
	c.AddCommand(&cobra.Command{Use: "download <attachment-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cx, _ := loadCtx(o, "")
		if o.DryRun {
			return print(cmd, o, output.Success(cx.inst.Name, map[string]any{"dry_run": true}))
		}
		r, err := cx.client.Do(httpclient.Request{Method: "GET", Path: "content/" + args[0], Query: map[string]string{"expand": "_links"}})
		if err != nil {
			return print(cmd, o, output.Failure("server_error", err.Error(), "", 500))
		}
		defer r.Body.Close()
		var m map[string]any
		_ = json.NewDecoder(r.Body).Decode(&m)
		dl := m["_links"].(map[string]any)["download"].(string)
		if strings.HasPrefix(dl, "http") && !strings.HasPrefix(strings.TrimRight(dl, "/"), strings.TrimRight(cx.inst.BaseURL, "/")) {
			return print(cmd, o, output.Failure("instance_url_mismatch", "off-instance download url", "", 400))
		}
		out, _ := cmd.Flags().GetString("output")
		if out == "" {
			return print(cmd, o, output.Success(cx.inst.Name, map[string]any{"metadata": m}))
		}
		rr, err := cx.client.Do(httpclient.Request{Method: "GET", Path: dl})
		if err != nil {
			return print(cmd, o, output.Failure("server_error", err.Error(), "", 500))
		}
		defer rr.Body.Close()
		b, _ := io.ReadAll(rr.Body)
		_ = os.WriteFile(out, b, 0644)
		return print(cmd, o, output.Success(cx.inst.Name, map[string]any{"output": out}))
	}})
	c.Commands()[1].Flags().String("output", "", "")
	c.AddCommand(&cobra.Command{Use: "delete <attachment-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		return do(o, cmd, "DELETE", "content/"+args[0], nil, nil)
	}})
	return c
}
