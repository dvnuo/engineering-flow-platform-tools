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

	"engineering-flow-platform-tools/internal/app"
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
	o := &Opts{}
	c := &cobra.Command{Use: "confluence", SilenceErrors: true, SilenceUsage: true}
	c.PersistentFlags().StringVar(&o.Instance, "instance", "", "")
	c.PersistentFlags().StringVar(&o.Config, "config", "", "")
	c.PersistentFlags().BoolVar(&o.JSON, "json", false, "")
	c.PersistentFlags().BoolVar(&o.DryRun, "dry-run", false, "")
	c.PersistentFlags().BoolVar(&o.Yes, "yes", false, "")
	c.AddCommand(commandsCmd(), schemaCmd(), helpLLMCmd(), authCmd(o), searchCmd(o), spaceCmd(o), pageCmd(o), contentCmd(o), blogCmd(o), userGroupCmd(o), webhookCmd(o), longtaskCmd(o), apiCmd(o))
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

func commandsCmd() *cobra.Command {
	return &cobra.Command{Use: "commands", RunE: func(cmd *cobra.Command, args []string) error {
		return output.Print(cmd.OutOrStdout(), "json", output.Success("", map[string]any{"commands": app.ConfluenceCommandList()}))
	}}
}
func schemaCmd() *cobra.Command {
	return &cobra.Command{Use: "schema <command>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		required := map[string][]string{"page.create": {"space", "title", "body"}, "page.update": {"id|url"}, "content.create": {"type", "title", "body"}, "content.update": {"content-id"}, "blog.create": {"space", "title", "body"}, "blog.update": {"blog-id-or-url"}}
		r := required[args[0]]
		return output.Print(cmd.OutOrStdout(), "json", output.Success("", map[string]any{"command": args[0], "required": r, "available": app.ConfluenceCommandList()}))
	}}
}
func helpLLMCmd() *cobra.Command {
	return &cobra.Command{Use: "help llm", RunE: func(cmd *cobra.Command, args []string) error {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Confluence LLM help: use commands/schema for structured usage.")
		return nil
	}}
}

func authCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "auth"}
	c.AddCommand(&cobra.Command{Use: "test", RunE: func(cmd *cobra.Command, args []string) error { return do(o, cmd, "GET", "user/current", nil, nil) }})
	return c
}
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
	return c
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
		payload := map[string]any{"type": "page", "title": ti, "space": map[string]string{"key": sp}, "body": map[string]any{"storage": map[string]string{"value": b, "representation": "storage"}}}
		return do(o, cmd, "POST", "content", nil, payload)
	}}
	cr.Flags().String("space", "", "")
	cr.Flags().String("title", "", "")
	cr.Flags().String("body", "", "")
	cr.Flags().String("body-file", "", "")
	cr.Flags().Bool("body-stdin", false, "")
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
			payload["body"] = map[string]any{"storage": map[string]string{"value": b, "representation": "storage"}}
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
	return c
}
func blogCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "blog"}
	c.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		sp, _ := cmd.Flags().GetString("space")
		return do(o, cmd, "GET", "content", map[string]string{"type": "blogpost", "spaceKey": sp}, nil)
	}})
	c.Commands()[0].Flags().String("space", "", "")
	return c
}
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
func webhookCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "webhook"}
	c.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error { return do(o, cmd, "GET", "webhooks", nil, nil) }})
	c.AddCommand(&cobra.Command{Use: "get <webhook-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return do(o, cmd, "GET", "webhooks/"+args[0], nil, nil) }})
	return c
}
func longtaskCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "longtask"}
	c.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error { return do(o, cmd, "GET", "longtask", nil, nil) }})
	c.AddCommand(&cobra.Command{Use: "get <task-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return do(o, cmd, "GET", "longtask/"+args[0], nil, nil) }})
	return c
}
