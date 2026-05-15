package commands

import (
	"encoding/json"
	"fmt"
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
	c.AddCommand(&cobra.Command{Use: "get <attachment-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return do(o, cmd, "GET", "content/"+args[0], nil, nil)
	}})
	download := &cobra.Command{Use: "download <attachment-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
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
		dl, err := attachmentDownloadURL(m)
		if err != nil {
			return print(cmd, o, output.Failure("not_found", "attachment download link missing", "Request metadata-only output or verify the attachment id.", 404))
		}
		if isAbsoluteURL(dl) && !urlBelongsToBase(dl, cx.inst.BaseURL) {
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
	}}
	download.Flags().String("output", "", "")
	c.AddCommand(download)
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
		pc.Flags().String("id", "", "")
		pc.Flags().String("url", "", "")
		pc.Flags().String("file", "", "")
		pc.Flags().String("attachment-id", "", "")
	}
	return c
}

func listAttachmentRunE(o *Opts) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		pageID, err := attachmentPageID(cmd, o, args)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", "--page-id, --id, or --url required", "", 400))
		}
		return do(o, cmd, "GET", "content/"+pageID+"/child/attachment", nil, nil)
	}
}

func uploadAttachmentRunE(o *Opts) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		pageID, err := attachmentPageID(cmd, o, args)
		file, _ := cmd.Flags().GetString("file")
		if len(args) >= 2 {
			file = args[1]
		}
		if err != nil || file == "" {
			return print(cmd, o, output.Failure("invalid_args", "--page-id/--id/--url and --file required", "", 400))
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
		pageID, err := attachmentPageID(cmd, o, args)
		attachmentID, _ := cmd.Flags().GetString("attachment-id")
		file, _ := cmd.Flags().GetString("file")
		if len(args) >= 3 {
			attachmentID, file = args[1], args[2]
		}
		if err != nil || attachmentID == "" || file == "" {
			return print(cmd, o, output.Failure("invalid_args", "--page-id/--id/--url, --attachment-id, and --file required", "", 400))
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

func attachmentPageID(cmd *cobra.Command, o *Opts, args []string) (string, error) {
	if pageID, _ := cmd.Flags().GetString("page-id"); pageID != "" {
		return pageID, nil
	}
	if id, _ := cmd.Flags().GetString("id"); id != "" {
		return id, nil
	}
	if u, _ := cmd.Flags().GetString("url"); u != "" {
		return pageIDFromURL(cmd, o, u)
	}
	if len(args) > 0 && args[0] != "" {
		return args[0], nil
	}
	return "", fmt.Errorf("invalid_args")
}

func pageIDFromURL(cmd *cobra.Command, o *Opts, raw string) (string, error) {
	o.Entity = raw
	return pageID(cmd, o)
}

func attachmentDownloadURL(m map[string]any) (string, error) {
	links, ok := m["_links"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("missing _links")
	}
	dl, ok := links["download"].(string)
	if !ok || dl == "" {
		return "", fmt.Errorf("missing download")
	}
	return dl, nil
}

func isAbsoluteURL(raw string) bool {
	return strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://")
}

func hiddenAttachmentAlias(use string, runE func(cmd *cobra.Command, args []string) error) *cobra.Command {
	return &cobra.Command{Use: use, Hidden: true, RunE: runE}
}
