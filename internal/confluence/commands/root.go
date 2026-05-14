package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/httpclient"
	"engineering-flow-platform-tools/internal/instance"
	"engineering-flow-platform-tools/internal/output"
	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/spf13/cobra"
)

type Opts struct {
	Instance, Config  string
	JSON, DryRun, Yes bool
}

type ctx struct {
	cfg    config.RootConfig
	inst   config.InstanceConfig
	client *httpclient.Client
}

func NewRoot() *cobra.Command {
	cobra.EnableCommandSorting = false
	o := &Opts{}
	c := &cobra.Command{Use: "confluence", SilenceErrors: true, SilenceUsage: true}
	c.PersistentFlags().StringVar(&o.Instance, "instance", "", "")
	c.PersistentFlags().StringVar(&o.Config, "config", "", "")
	c.PersistentFlags().BoolVar(&o.JSON, "json", false, "")
	c.PersistentFlags().BoolVar(&o.DryRun, "dry-run", false, "")
	c.PersistentFlags().BoolVar(&o.Yes, "yes", false, "")
	c.AddCommand(commandsCmd(), schemaCmd(), helpLLMCmd(), instanceCmd(o), authCmd(o), myselfCmd(o), serverInfoCmd(o), resolveCmd(o), searchCmd(o), cqlCmd(o), spaceCmd(o), pageCmd(o), contentCmd(o), blogCmd(o), labelCmd(o), userGroupCmd(o), groupCmd(o), webhookCmd(o), longtaskCmd(o), attachmentCmd(o), commentCmd(o), restrictionCmd(o), apiCmd(o))
	return c
}
func print(cmd *cobra.Command, o *Opts, e output.Envelope) error {
	f := "table"
	if o.JSON {
		f = "json"
	}
	return output.Print(cmd.OutOrStdout(), f, e)
}
func loadCtx(o *Opts, entity string) (*ctx, error) {
	p, _ := config.ResolvePath(o.Config)
	cfg, err := config.Load(p)
	if err != nil {
		return nil, err
	}
	res, err := instance.Resolve(cfg.Confluence, o.Instance, entity, "confluence")
	if err != nil {
		return nil, err
	}
	cl, err := httpclient.New(res.Instance)
	if err != nil {
		return nil, err
	}
	return &ctx{cfg: cfg, inst: res.Instance, client: cl}, nil
}
func do(o *Opts, cmd *cobra.Command, method, p string, q map[string]string, body any) error {
	cx, err := loadCtx(o, p)
	if err != nil {
		return print(cmd, o, output.Failure("config_error", err.Error(), "", 400))
	}
	if o.DryRun {
		return print(cmd, o, output.Success(cx.inst.Name, map[string]any{"dry_run": true, "method": method, "path": p, "query": q, "body": body}))
	}
	resp, err := cx.client.Do(httpclient.Request{Method: method, Path: p, Query: q, JSONBody: body})
	if err != nil {
		return print(cmd, o, output.Failure("server_error", err.Error(), "", 500))
	}
	defer resp.Body.Close()
	d, _ := io.ReadAll(resp.Body)
	out := map[string]any{"ok": true}
	_ = json.Unmarshal(d, &out)
	return print(cmd, o, output.Success(cx.inst.Name, out))
}

func helpLLMCmd() *cobra.Command {
	return &cobra.Command{Use: "help llm", RunE: func(cmd *cobra.Command, args []string) error {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Confluence LLM help: use commands/schema for structured usage.")
		return nil
	}}
}

func readBody(cmd *cobra.Command) string {
	b, _ := cmd.Flags().GetString("body")
	if b != "" {
		return b
	}
	f, _ := cmd.Flags().GetString("body-file")
	if f != "" {
		d, _ := os.ReadFile(f)
		return string(d)
	}
	s, _ := cmd.Flags().GetBool("body-stdin")
	if s {
		d, _ := io.ReadAll(cmd.InOrStdin())
		return string(d)
	}
	return ""
}
func bodyFormat(cmd *cobra.Command) string {
	f, _ := cmd.Flags().GetString("body-format")
	if f == "" {
		return "storage"
	}
	return f
}
func confluenceBody(cmd *cobra.Command, v string) map[string]any {
	f := bodyFormat(cmd)
	return map[string]any{f: map[string]string{"value": v, "representation": f}}
}
func pageID(cmd *cobra.Command, o *Opts) (string, error) {
	id, _ := cmd.Flags().GetString("id")
	u, _ := cmd.Flags().GetString("url")
	if (id == "") == (u == "") {
		return "", fmt.Errorf("invalid_args")
	}
	if id != "" {
		return id, nil
	}
	pu, err := url.Parse(u)
	if err != nil {
		return "", err
	}
	if pu.IsAbs() && o.Instance != "" {
		cx, e := loadCtx(o, "")
		if e == nil && !strings.HasPrefix(strings.TrimRight(u, "/"), strings.TrimRight(cx.inst.BaseURL, "/")) {
			return "", fmt.Errorf("instance_url_mismatch")
		}
	}
	pid := pu.Query().Get("pageId")
	if pid != "" {
		return pid, nil
	}
	seg := strings.Split(strings.Trim(pu.Path, "/"), "/")
	for i := len(seg) - 1; i >= 0; i-- {
		if _, e := strconv.Atoi(seg[i]); e == nil {
			return seg[i], nil
		}
	}
	return "", fmt.Errorf("invalid_args")
}
func pageCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "page"}
	get := &cobra.Command{Use: "get", RunE: func(cmd *cobra.Command, args []string) error {
		id, err := pageID(cmd, o)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", "exactly one of --id/--url", "", 400))
		}
		return do(o, cmd, "GET", "content/"+id, nil, nil)
	}}
	get.Flags().String("id", "", "")
	get.Flags().String("url", "", "")
	c.AddCommand(get)
	gbt := &cobra.Command{Use: "get-by-title", RunE: func(cmd *cobra.Command, args []string) error {
		sp, _ := cmd.Flags().GetString("space")
		ti, _ := cmd.Flags().GetString("title")
		return do(o, cmd, "GET", "content", map[string]string{"spaceKey": sp, "title": ti, "type": "page"}, nil)
	}}
	gbt.Flags().String("space", "", "")
	gbt.Flags().String("title", "", "")
	c.AddCommand(gbt)
	cr := &cobra.Command{Use: "create", RunE: func(cmd *cobra.Command, args []string) error {
		sp, _ := cmd.Flags().GetString("space")
		ti, _ := cmd.Flags().GetString("title")
		b := readBody(cmd)
		if sp == "" || ti == "" || b == "" {
			return print(cmd, o, output.Failure("invalid_args", "missing required args", "", 400))
		}
		payload := map[string]any{"type": "page", "title": ti, "space": map[string]string{"key": sp}, "body": confluenceBody(cmd, b)}
		return do(o, cmd, "POST", "content", nil, payload)
	}}
	cr.Flags().String("space", "", "")
	cr.Flags().String("title", "", "")
	cr.Flags().String("body", "", "")
	cr.Flags().String("body-file", "", "")
	cr.Flags().Bool("body-stdin", false, "")
	cr.Flags().String("body-format", "storage", "")
	c.AddCommand(cr)
	upd := &cobra.Command{Use: "update", RunE: func(cmd *cobra.Command, args []string) error {
		id, err := pageID(cmd, o)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", "exactly one of --id/--url", "", 400))
		}
		v, _ := cmd.Flags().GetInt("version")
		cx, _ := loadCtx(o, "")
		if v == 0 && !o.DryRun {
			r, e := cx.client.Do(httpclient.Request{Method: "GET", Path: "content/" + id})
			if e != nil {
				return print(cmd, o, output.Failure("server_error", "version fetch failed", "", 500))
			}
			defer r.Body.Close()
			var m map[string]any
			_ = json.NewDecoder(r.Body).Decode(&m)
			v = int(m["version"].(map[string]any)["number"].(float64)) + 1
		}
		payload := map[string]any{"version": map[string]any{"number": v}}
		if t, _ := cmd.Flags().GetString("title"); t != "" {
			payload["title"] = t
		}
		if b := readBody(cmd); b != "" {
			payload["body"] = confluenceBody(cmd, b)
		}
		return do(o, cmd, "PUT", "content/"+id, nil, payload)
	}}
	upd.Flags().String("id", "", "")
	upd.Flags().String("url", "", "")
	upd.Flags().Int("version", 0, "")
	upd.Flags().String("title", "", "")
	upd.Flags().String("body", "", "")
	upd.Flags().String("body-file", "", "")
	upd.Flags().Bool("body-stdin", false, "")
	upd.Flags().String("body-format", "storage", "")
	c.AddCommand(upd)
	del := &cobra.Command{Use: "delete", RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		id, err := pageID(cmd, o)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", "exactly one of --id/--url", "", 400))
		}
		return do(o, cmd, "DELETE", "content/"+id, nil, nil)
	}}
	del.Flags().String("id", "", "")
	del.Flags().String("url", "", "")
	c.AddCommand(del)
	ex := &cobra.Command{Use: "export-markdown", RunE: func(cmd *cobra.Command, args []string) error {
		id, err := pageID(cmd, o)
		if err != nil {
			return err
		}
		cx, _ := loadCtx(o, "")
		r, e := cx.client.Do(httpclient.Request{Method: "GET", Path: "content/" + id, Query: map[string]string{"expand": "body.view"}})
		if e != nil {
			return print(cmd, o, output.Failure("server_error", e.Error(), "", 500))
		}
		defer r.Body.Close()
		var m map[string]any
		_ = json.NewDecoder(r.Body).Decode(&m)
		html := m["body"].(map[string]any)["view"].(map[string]any)["value"].(string)
		md, _ := htmltomarkdown.ConvertString(html)
		out, _ := cmd.Flags().GetString("output")
		if out != "" {
			_ = os.WriteFile(out, []byte(md), 0644)
			return print(cmd, o, output.Success(cx.inst.Name, map[string]any{"output": filepath.Clean(out)}))
		}
		return print(cmd, o, output.Success(cx.inst.Name, map[string]any{"markdown": md}))
	}}
	ex.Flags().String("id", "", "")
	ex.Flags().String("url", "", "")
	ex.Flags().String("output", "", "")
	c.AddCommand(ex)
	exh := &cobra.Command{Use: "export-html", RunE: func(cmd *cobra.Command, args []string) error {
		id, err := pageID(cmd, o)
		if err != nil {
			return err
		}
		cx, _ := loadCtx(o, "")
		r, e := cx.client.Do(httpclient.Request{Method: "GET", Path: "content/" + id, Query: map[string]string{"expand": "body.export_view"}})
		if e != nil {
			return print(cmd, o, output.Failure("server_error", e.Error(), "", 500))
		}
		defer r.Body.Close()
		var m map[string]any
		_ = json.NewDecoder(r.Body).Decode(&m)
		html := m["body"].(map[string]any)["export_view"].(map[string]any)["value"].(string)
		out, _ := cmd.Flags().GetString("output")
		if out != "" {
			_ = os.WriteFile(out, []byte(html), 0644)
			return print(cmd, o, output.Success(cx.inst.Name, map[string]any{"output": filepath.Clean(out)}))
		}
		return print(cmd, o, output.Success(cx.inst.Name, map[string]any{"html": html}))
	}}
	exh.Flags().String("id", "", "")
	exh.Flags().String("url", "", "")
	exh.Flags().String("output", "", "")
	c.AddCommand(exh)
	c.AddCommand(&cobra.Command{Use: "body-storage", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "GET", "content/"+id, map[string]string{"expand": "body.storage"}, nil)
	}})
	c.AddCommand(&cobra.Command{Use: "body-view", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "GET", "content/"+id, map[string]string{"expand": "body.view"}, nil)
	}})
	c.AddCommand(&cobra.Command{Use: "label-list", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "GET", "content/"+id+"/label", nil, nil)
	}})
	c.AddCommand(&cobra.Command{Use: "label-add", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		labels, _ := cmd.Flags().GetStringSlice("label")
		arr := []map[string]string{}
		for _, l := range labels {
			arr = append(arr, map[string]string{"prefix": "global", "name": l})
		}
		return do(o, cmd, "POST", "content/"+id+"/label", nil, arr)
	}})
	c.Commands()[len(c.Commands())-1].Flags().StringSlice("label", nil, "")
	c.AddCommand(&cobra.Command{Use: "label-delete", RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		n, _ := cmd.Flags().GetString("label")
		return do(o, cmd, "DELETE", "content/"+id+"/label", map[string]string{"name": n}, nil)
	}})
	c.Commands()[len(c.Commands())-1].Flags().String("label", "", "")
	c.AddCommand(&cobra.Command{Use: "comment-list", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "GET", "content/"+id+"/child/comment", nil, nil)
	}})
	c.AddCommand(&cobra.Command{Use: "comment-add", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		b := readBody(cmd)
		return do(o, cmd, "POST", "content/"+id+"/child/comment", nil, map[string]any{"type": "comment", "body": confluenceBody(cmd, b)})
	}})
	c.Commands()[len(c.Commands())-1].Flags().String("body", "", "")
	c.Commands()[len(c.Commands())-1].Flags().String("body-file", "", "")
	c.Commands()[len(c.Commands())-1].Flags().Bool("body-stdin", false, "")
	c.Commands()[len(c.Commands())-1].Flags().String("body-format", "storage", "")
	prop := &cobra.Command{Use: "property"}
	prop.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		id, err := pageID(cmd, o)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "GET", "content/"+id+"/property", nil, nil)
	}})
	prop.AddCommand(&cobra.Command{Use: "get <key>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		id, err := pageID(cmd, o)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "GET", "content/"+id+"/property/"+args[0], nil, nil)
	}})
	prop.AddCommand(&cobra.Command{Use: "set <key>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		id, err := pageID(cmd, o)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "PUT", "content/"+id+"/property/"+args[0], nil, map[string]any{"key": args[0], "value": readBody(cmd)})
	}})
	prop.Commands()[2].Flags().String("body", "", "")
	prop.Commands()[2].Flags().String("body-file", "", "")
	prop.Commands()[2].Flags().Bool("body-stdin", false, "")
	prop.AddCommand(&cobra.Command{Use: "delete <key>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		id, err := pageID(cmd, o)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "DELETE", "content/"+id+"/property/"+args[0], nil, nil)
	}})
	for _, pc := range prop.Commands() {
		pc.Flags().String("id", "", "")
		pc.Flags().String("url", "", "")
	}
	c.AddCommand(prop)
	c.AddCommand(&cobra.Command{Use: "children", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "GET", "content/"+id+"/child", nil, nil)
	}})
	c.AddCommand(&cobra.Command{Use: "descendants", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "GET", "content/"+id+"/descendant", nil, nil)
	}})
	c.AddCommand(&cobra.Command{Use: "ancestors", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "GET", "content/"+id, map[string]string{"expand": "ancestors"}, nil)
	}})
	c.AddCommand(&cobra.Command{Use: "body", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		f, _ := cmd.Flags().GetString("format")
		if f == "" {
			f = "storage"
		}
		return do(o, cmd, "GET", "content/"+id, map[string]string{"expand": "body." + f}, nil)
	}})
	c.Commands()[len(c.Commands())-1].Flags().String("format", "storage", "")
	c.AddCommand(&cobra.Command{Use: "history", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "GET", "content/"+id+"/version", nil, nil)
	}})
	c.AddCommand(&cobra.Command{Use: "version", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "GET", "content/"+id+"/version/latest", nil, nil)
	}})
	c.AddCommand(&cobra.Command{Use: "restore", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		v, _ := cmd.Flags().GetInt("version")
		return do(o, cmd, "POST", "content/"+id+"/version", nil, map[string]any{"operationKey": "restore", "params": map[string]any{"versionNumber": v}})
	}})
	c.Commands()[len(c.Commands())-1].Flags().Int("version", 0, "")
	c.AddCommand(&cobra.Command{Use: "move", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		pid, _ := cmd.Flags().GetString("parent-id")
		pos, _ := cmd.Flags().GetString("position")
		return do(o, cmd, "PUT", "content/"+id+"/move/"+pos+"/"+pid, nil, nil)
	}})
	c.Commands()[len(c.Commands())-1].Flags().String("parent-id", "", "")
	c.Commands()[len(c.Commands())-1].Flags().String("position", "append", "")
	c.AddCommand(&cobra.Command{Use: "watch", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "POST", "user/watch/content/"+id, nil, nil)
	}})
	c.AddCommand(&cobra.Command{Use: "unwatch", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "DELETE", "user/watch/content/"+id, nil, nil)
	}})
	for _, pc := range []*cobra.Command{c.Commands()[len(c.Commands())-11], c.Commands()[len(c.Commands())-10], c.Commands()[len(c.Commands())-9], c.Commands()[len(c.Commands())-8], c.Commands()[len(c.Commands())-7], c.Commands()[len(c.Commands())-6], c.Commands()[len(c.Commands())-5], c.Commands()[len(c.Commands())-4], c.Commands()[len(c.Commands())-3], c.Commands()[len(c.Commands())-2], c.Commands()[len(c.Commands())-1]} {
		if pc.Flags().Lookup("id") == nil {
			pc.Flags().String("id", "", "")
		}
		if pc.Flags().Lookup("url") == nil {
			pc.Flags().String("url", "", "")
		}
	}
	return c
}
func apiCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "api"}
	for _, m := range []string{"get", "post", "put", "delete"} {
		mm := m
		method := strings.ToUpper(m)
		cmd := &cobra.Command{Use: mm + " <path>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			if method == "DELETE" && !o.Yes {
				return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
			}
			p := args[0]
			if strings.HasPrefix(p, "http") {
				cx, _ := loadCtx(o, "")
				if !strings.HasPrefix(strings.TrimRight(p, "/"), strings.TrimRight(cx.inst.BaseURL, "/")) {
					return print(cmd, o, output.Failure("instance_url_mismatch", "off-instance url", "", 400))
				}
				p = strings.TrimPrefix(p, cx.inst.BaseURL)
			}
			b := readBody(cmd)
			var bj any
			if b != "" {
				_ = json.Unmarshal([]byte(b), &bj)
			}
			return do(o, cmd, method, p, nil, bj)
		}}
		cmd.Flags().String("body", "", "")
		cmd.Flags().String("body-file", "", "")
		cmd.Flags().Bool("body-stdin", false, "")
		c.AddCommand(cmd)
	}
	return c
}

// helper for multipart tests
func multipartData(path string) (*bytes.Buffer, string, error) {
	b := &bytes.Buffer{}
	w := multipart.NewWriter(b)
	f, err := w.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return nil, "", err
	}
	in, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer in.Close()
	_, _ = io.Copy(f, in)
	_ = w.Close()
	return b, w.FormDataContentType(), nil
}

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
		b := readBody(cmd)
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
	c.AddCommand(&cobra.Command{Use: "delete <content-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		return do(o, cmd, "DELETE", "content/"+args[0], nil, nil)
	}})
	return c
}
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

func labelCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "label"}
	c.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		p, _ := cmd.Flags().GetString("prefix")
		q := map[string]string{}
		if p != "" {
			q["prefix"] = p
		}
		return do(o, cmd, "GET", "label", q, nil)
	}})
	c.Commands()[0].Flags().String("prefix", "", "")
	return c
}

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
func commentCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "comment"}
	c.AddCommand(&cobra.Command{Use: "get <comment-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return do(o, cmd, "GET", "content/"+args[0], nil, nil) }})
	c.AddCommand(&cobra.Command{Use: "update <comment-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		b := readBody(cmd)
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
func restrictionCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "restriction"}
	c.AddCommand(&cobra.Command{Use: "list <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return do(o, cmd, "GET", "content/"+args[0]+"/restriction", nil, nil)
	}})
	c.AddCommand(&cobra.Command{Use: "add <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		op, _ := cmd.Flags().GetString("operation")
		users, _ := cmd.Flags().GetStringSlice("user")
		groups, _ := cmd.Flags().GetStringSlice("group")
		body := map[string]any{"restrictions": map[string]any{"user": users, "group": groups}}
		return do(o, cmd, "POST", "content/"+args[0]+"/restriction/byOperation/"+op, nil, body)
	}})
	c.Commands()[1].Flags().String("operation", "read", "")
	c.Commands()[1].Flags().StringSlice("user", nil, "")
	c.Commands()[1].Flags().StringSlice("group", nil, "")
	c.AddCommand(&cobra.Command{Use: "delete <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		op, _ := cmd.Flags().GetString("operation")
		return do(o, cmd, "DELETE", "content/"+args[0]+"/restriction/byOperation/"+op, nil, nil)
	}})
	c.Commands()[2].Flags().String("operation", "read", "")
	c.AddCommand(&cobra.Command{Use: "watcher-list <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if o.DryRun {
			return do(o, cmd, "GET", "content/"+args[0]+"/notification/child-created", nil, nil)
		}
		cx, _ := loadCtx(o, "")
		resp, err := cx.client.Do(httpclient.Request{Method: "GET", Path: "content/" + args[0] + "/notification/child-created"})
		if err != nil {
			if strings.Contains(err.Error(), "not_found") {
				return print(cmd, o, output.Failure("not_supported", "watcher endpoint not supported", "", 404))
			}
			return print(cmd, o, output.Failure("server_error", err.Error(), "", 500))
		}
		defer resp.Body.Close()
		return print(cmd, o, output.Success(cx.inst.Name, map[string]any{"ok": true}))
	}})
	return c
}
