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
	c.AddCommand(&cobra.Command{Use: "download <attachment-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cx, _ := loadCtx(o, "")
		if o.DryRun {
			return print(cmd, o, output.Success(cx.inst.Name, map[string]any{"dry_run": true}))
		}
		r, err := cx.client.Do(httpclient.Request{Method: "GET", Path: "content/" + args[0], Query: map[string]string{"expand": "_links"}})
		if err != nil {
			return print(cmd, o, envelopeError(err, "server_error"))
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
			return print(cmd, o, envelopeError(err, "server_error"))
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
	c.AddCommand(hiddenAttachmentAlias("upload <page-id> <file>", uploadAttachmentRunE(o)))
	c.AddCommand(hiddenAttachmentAlias("update <page-id> <attachment-id> <file>", updateAttachmentRunE(o)))
	c.AddCommand(hiddenAttachmentAlias("list <page-id>", listAttachmentRunE(o)))
	return c
}

func pageAttachmentCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "attachment"}
	c.AddCommand(&cobra.Command{Use: "list", RunE: listAttachmentRunE(o)})
	c.AddCommand(&cobra.Command{Use: "upload", RunE: uploadAttachmentRunE(o)})
	c.AddCommand(&cobra.Command{Use: "update", RunE: updateAttachmentRunE(o)})
	for _, pc := range c.Commands() {
		pc.Flags().String("page-id", "", "")
		pc.Flags().String("file", "", "")
		pc.Flags().String("attachment-id", "", "")
	}
	return c
}

func listAttachmentRunE(o *Opts) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		pageID, _ := cmd.Flags().GetString("page-id")
		if pageID == "" && len(args) > 0 {
			pageID = args[0]
		}
		if pageID == "" {
			return print(cmd, o, output.Failure("invalid_args", "--page-id required", "", 400))
		}
		return do(o, cmd, "GET", "content/"+pageID+"/child/attachment", nil, nil)
	}
}

func uploadAttachmentRunE(o *Opts) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		pageID, _ := cmd.Flags().GetString("page-id")
		file, _ := cmd.Flags().GetString("file")
		if len(args) >= 2 {
			pageID, file = args[0], args[1]
		}
		if pageID == "" || file == "" {
			return print(cmd, o, output.Failure("invalid_args", "--page-id and --file required", "", 400))
		}
		if o.DryRun {
			return print(cmd, o, output.Success("", map[string]any{"dry_run": true, "method": "POST", "path": "content/" + pageID + "/child/attachment"}))
		}
		cx, _ := loadCtx(o, "")
		f, err := os.Open(file)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", err.Error(), "", 400))
		}
		defer f.Close()
		resp, err := cx.client.Do(httpclient.Request{Method: "POST", Path: "content/" + pageID + "/child/attachment", Multipart: f, MultipartField: "file", MultipartName: filepath.Base(file), Headers: map[string]string{"X-Atlassian-Token": "no-check"}})
		if err != nil {
			return print(cmd, o, envelopeError(err, "server_error"))
		}
		defer resp.Body.Close()
		return print(cmd, o, output.Success(cx.inst.Name, map[string]any{"uploaded": true}))
	}
}

func updateAttachmentRunE(o *Opts) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		pageID, _ := cmd.Flags().GetString("page-id")
		attachmentID, _ := cmd.Flags().GetString("attachment-id")
		file, _ := cmd.Flags().GetString("file")
		if len(args) >= 3 {
			pageID, attachmentID, file = args[0], args[1], args[2]
		}
		if pageID == "" || attachmentID == "" || file == "" {
			return print(cmd, o, output.Failure("invalid_args", "--page-id, --attachment-id, and --file required", "", 400))
		}
		if o.DryRun {
			return print(cmd, o, output.Success("", map[string]any{"dry_run": true, "method": "POST", "path": "content/" + pageID + "/child/attachment/" + attachmentID + "/data"}))
		}
		cx, _ := loadCtx(o, "")
		f, err := os.Open(file)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", err.Error(), "", 400))
		}
		defer f.Close()
		resp, err := cx.client.Do(httpclient.Request{Method: "POST", Path: "content/" + pageID + "/child/attachment/" + attachmentID + "/data", Multipart: f, MultipartField: "file", MultipartName: filepath.Base(file), Headers: map[string]string{"X-Atlassian-Token": "no-check"}})
		if err != nil {
			return print(cmd, o, envelopeError(err, "server_error"))
		}
		defer resp.Body.Close()
		return print(cmd, o, output.Success(cx.inst.Name, map[string]any{"updated": true}))
	}
}

func hiddenAttachmentAlias(use string, runE func(cmd *cobra.Command, args []string) error) *cobra.Command {
	return &cobra.Command{Use: use, Hidden: true, RunE: runE}
}
