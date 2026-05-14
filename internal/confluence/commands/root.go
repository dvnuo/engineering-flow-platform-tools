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
			q := map[string]string{}
			for _, kv := range mustStringSlice(cmd, "query") {
				parts := strings.SplitN(kv, "=", 2)
				if len(parts) == 2 && strings.TrimSpace(parts[0]) != "" {
					q[strings.TrimSpace(parts[0])] = parts[1]
				}
			}
			var bj any
			if b != "" {
				_ = json.Unmarshal([]byte(b), &bj)
			}
			return do(o, cmd, method, p, q, bj)
		}}
		cmd.Flags().StringSlice("query", nil, "")
		cmd.Flags().String("body", "", "")
		cmd.Flags().String("body-file", "", "")
		cmd.Flags().Bool("body-stdin", false, "")
		c.AddCommand(cmd)
	}
	return c
}

func mustStringSlice(cmd *cobra.Command, name string) []string {
	v, _ := cmd.Flags().GetStringSlice(name)
	return v
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
