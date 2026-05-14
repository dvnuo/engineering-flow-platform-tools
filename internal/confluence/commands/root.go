package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"engineering-flow-platform-tools/internal/catalog"
	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/httpclient"
	"engineering-flow-platform-tools/internal/instance"
	"engineering-flow-platform-tools/internal/output"
	"engineering-flow-platform-tools/internal/version"
	"github.com/spf13/cobra"
)

type Opts struct {
	Instance, Config, Format   string
	JSON, Verbose, DryRun, Yes bool
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
	c.PersistentFlags().StringVar(&o.Format, "format", "table", "")
	c.PersistentFlags().BoolVar(&o.Verbose, "verbose", false, "")
	c.PersistentFlags().BoolVar(&o.DryRun, "dry-run", false, "")
	c.PersistentFlags().BoolVar(&o.Yes, "yes", false, "")
	c.AddCommand(commandsCmd(), schemaCmd(), helpLLMCmd(), cliVersionCmd(o), instanceCmd(o), authCmd(o), myselfCmd(o), serverInfoCmd(o), resolveCmd(o), searchCmd(o), cqlCmd(o), spaceCmd(o), pageCmd(o), contentCmd(o), blogCmd(o), labelCmd(o), userGroupCmd(o), groupCmd(o), webhookCmd(o), longtaskCmd(o), attachmentCmd(o), commentCmd(o), hiddenCmd(restrictionCmd(o)), apiCmd(o))
	return c
}
func print(cmd *cobra.Command, o *Opts, e output.Envelope) error {
	f := "table"
	if o.JSON {
		f = "json"
	} else if o.Format != "" {
		f = o.Format
	}
	return output.Print(cmd.OutOrStdout(), f, e)
}

func envelopeError(err error, fallbackCode string) output.Envelope {
	var httpErr *httpclient.HTTPError
	if errors.As(err, &httpErr) {
		return output.Failure(httpErr.Code, httpErr.Message, httpErr.Hint, httpErr.Status)
	}
	if fallbackCode == "" {
		fallbackCode = "server_error"
	}
	return output.Failure(fallbackCode, err.Error(), "", 500)
}

func authFromFlags(cmd *cobra.Command) (config.AuthConfig, error) {
	username, _ := cmd.Flags().GetString("username")
	authType, _ := cmd.Flags().GetString("auth-type")
	auth := config.AuthConfig{Type: authType, Username: username}
	if mustB(cmd, "password-stdin") {
		secret, _ := io.ReadAll(cmd.InOrStdin())
		auth.Password = strings.TrimRight(string(secret), "\r\n")
	}
	if mustB(cmd, "api-key-stdin") {
		secret, _ := io.ReadAll(cmd.InOrStdin())
		auth.APIKey = strings.TrimRight(string(secret), "\r\n")
	}
	if mustB(cmd, "token-stdin") {
		secret, _ := io.ReadAll(cmd.InOrStdin())
		auth.Token = strings.TrimRight(string(secret), "\r\n")
	}
	auth.NormalizeType()
	switch auth.Type {
	case "basic_password":
		if auth.Username == "" || auth.Password == "" {
			return auth, fmt.Errorf("invalid_args")
		}
	case "basic_api_key":
		if auth.Username == "" || auth.APIKey == "" {
			return auth, fmt.Errorf("invalid_args")
		}
	case "bearer_token":
		if auth.Token == "" {
			return auth, fmt.Errorf("invalid_args")
		}
	}
	return auth, nil
}

func addAuthFlags(cmd *cobra.Command) {
	cmd.Flags().String("username", "", "")
	cmd.Flags().String("auth-type", "", "")
	cmd.Flags().Bool("password-stdin", false, "")
	cmd.Flags().Bool("api-key-stdin", false, "")
	cmd.Flags().Bool("token-stdin", false, "")
}
func cliVersionCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "version", RunE: func(cmd *cobra.Command, args []string) error {
		return print(cmd, o, output.Success("", map[string]any{"version": version.Version, "commit": version.Commit, "date": version.Date}))
	}}
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
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	defer resp.Body.Close()
	d, _ := io.ReadAll(resp.Body)
	out := map[string]any{"ok": true}
	_ = json.Unmarshal(d, &out)
	return print(cmd, o, output.Success(cx.inst.Name, out))
}

func helpLLMCmd() *cobra.Command {
	return &cobra.Command{Use: "help llm", RunE: func(cmd *cobra.Command, args []string) error {
		return output.Print(cmd.OutOrStdout(), "json", output.Success("", map[string]any{"tips": []string{"use commands --json", "use schema <command> --json"}, "commands": catalog.Commands("confluence")}))
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

func mustS(cmd *cobra.Command, name string) string {
	v, _ := cmd.Flags().GetString(name)
	return v
}

func mustB(cmd *cobra.Command, name string) bool {
	v, _ := cmd.Flags().GetBool(name)
	return v
}

func hiddenAlias(use string, target *cobra.Command) *cobra.Command {
	return &cobra.Command{Use: use, Hidden: true, RunE: func(cmd *cobra.Command, args []string) error {
		return target.RunE(cmd, args)
	}}
}

func hiddenCmd(cmd *cobra.Command) *cobra.Command {
	cmd.Hidden = true
	return cmd
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
