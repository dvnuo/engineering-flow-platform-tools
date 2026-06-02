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
	"engineering-flow-platform-tools/internal/clihelp"
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

type PageRef struct {
	Ctx        *ctx
	ID         string
	Space      string
	Title      string
	SourceURL  string
	EntityType string
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
	clihelp.ApplyCatalogHelp(c, clihelp.ProductHelp{
		Product: "confluence",
		Binary:  "confluence",
		Short:   "Operate Confluence pages, spaces, content, attachments, comments, and search",
		Long: strings.TrimSpace(`confluence is a terminal-invoked CLI for agents and scripts that need stable JSON access to Confluence Server/Data Center resources.

Use it for pages, spaces, content, blogs, attachments, comments, labels, restrictions, users, groups, long tasks, webhooks, and raw REST calls. For agent workflows, default every command and subcommand to --json. Use --dry-run before write operations and --yes only after explicit user confirmation for destructive operations.

Configuration uses the shared EFP config file, normally ~/.efp/config.yaml.`),
		Examples: []string{
			`confluence page get --id 123 --json`,
			`confluence page update --id 123 --title "Runbook" --body-file page.html --dry-run --json`,
			`confluence schema page.update --json`,
			`confluence help llm --json`,
		},
		Instructions: "copy cmd/confluence/confluence-cli.instructions.md to ~/.copilot/instructions/confluence-cli.instructions.md.",
		Groups: map[string]string{
			"instance":         "Manage configured Confluence instances.",
			"auth":             "Manage Confluence credentials stored in the EFP config.",
			"search":           "Search Confluence content and users.",
			"space":            "Work with Confluence spaces.",
			"space.permission": "Inspect Confluence space permissions.",
			"space.property":   "Read and write Confluence space properties.",
			"page":             "Work with Confluence pages by id, URL, space, or title.",
			"page.attachment":  "Manage attachments on Confluence pages.",
			"page.comment":     "Manage comments on Confluence pages.",
			"page.label":       "Manage labels on Confluence pages.",
			"page.property":    "Read and write Confluence page properties.",
			"page.restriction": "Manage Confluence page restrictions.",
			"page.watcher":     "Manage Confluence page watchers.",
			"content":          "Work with generic Confluence content resources.",
			"blog":             "Work with Confluence blog posts.",
			"label":            "Inspect Confluence labels.",
			"user":             "Inspect and search Confluence users.",
			"group":            "Inspect Confluence groups and members.",
			"webhook":          "Manage Confluence webhooks.",
			"longtask":         "Inspect Confluence long-running tasks.",
			"attachment":       "Read, download, or delete Confluence attachments.",
			"comment":          "Read, update, or delete Confluence comments.",
			"api":              "Call raw Confluence REST endpoints on the selected instance.",
		},
	})
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
	msg := httpclient.SanitizeErrorText(err.Error())
	if isStableErrorCode(msg) {
		return output.Failure(msg, msg, "", 400)
	}
	if fallbackCode == "" {
		fallbackCode = "server_error"
	}
	return output.Failure(fallbackCode, msg, "", 500)
}

func isStableErrorCode(code string) bool {
	switch code {
	case "config_missing", "no_instance_configured", "instance_required", "ambiguous_instance", "instance_url_mismatch", "invalid_args", "not_found", "not_supported", "auth_failed", "permission_denied", "network_error", "server_error":
		return true
	default:
		return false
	}
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
		return print(cmd, o, envelopeError(err, "config_error"))
	}
	return doWithCtx(o, cmd, cx, method, p, q, body)
}

func doWithCtx(o *Opts, cmd *cobra.Command, cx *ctx, method, p string, q map[string]string, body any) error {
	if o.DryRun {
		return print(cmd, o, output.Success(cx.inst.Name, map[string]any{"dry_run": true, "method": method, "path": p, "query": q, "body": redactDryRunBody(body)}))
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

func redactDryRunBody(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, v := range x {
			if isSecretKey(k) {
				out[k] = "***REDACTED***"
				continue
			}
			out[k] = redactDryRunBody(v)
		}
		return out
	case map[string]string:
		out := map[string]string{}
		for k, v := range x {
			if isSecretKey(k) {
				out[k] = "***REDACTED***"
				continue
			}
			out[k] = v
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, v := range x {
			out[i] = redactDryRunBody(v)
		}
		return out
	default:
		return v
	}
}

func isSecretKey(k string) bool {
	k = strings.ToLower(k)
	return strings.Contains(k, "password") || strings.Contains(k, "api_key") || strings.Contains(k, "apikey") || strings.Contains(k, "token") || k == "authorization"
}

func helpLLMCmd() *cobra.Command {
	return &cobra.Command{Use: "help llm", RunE: func(cmd *cobra.Command, args []string) error {
		tips := []string{
			"For agents, --json is the default way to use every confluence command and subcommand.",
			"Always add --json so results and failures use the stable ok/data/error envelope; omit it only when intentionally reading human-oriented --help text.",
			"Use --instance when multiple instances are configured.",
			"Full Jira/Confluence URLs can auto-select the instance.",
			"Use --dry-run before write operations.",
			"Use --yes for destructive operations.",
			"Inspect error.code and error.hint before retrying.",
			"Command parsing failures return an invalid_args JSON envelope when --json is present.",
			"On Windows cmd, use double quotes and cmd-native commands such as where/dir/cd/type; do not use Bash-only commands such as pwd, command -v, cat, ls, cd \"$PWD\", or single quotes.",
			"If terminal output capture is unreliable, rerun the exact .exe path from where confluence and redirect stdout to a workspace file, then inspect the JSON envelope with the file-read tool.",
		}
		return output.Print(cmd.OutOrStdout(), "json", output.Success("", map[string]any{"tips": tips, "commands": catalog.Commands("confluence")}))
	}}
}

func readBody(cmd *cobra.Command) (string, error) {
	b, _ := cmd.Flags().GetString("body")
	if b != "" {
		return b, nil
	}
	f, _ := cmd.Flags().GetString("body-file")
	if f != "" {
		d, err := os.ReadFile(f)
		if err != nil {
			return "", fmt.Errorf("failed to read --body-file: %w", err)
		}
		return string(d), nil
	}
	s, _ := cmd.Flags().GetBool("body-stdin")
	if s {
		d, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return "", fmt.Errorf("failed to read --body-stdin: %w", err)
		}
		return string(d), nil
	}
	return "", nil
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
	ref, err := resolvePageRef(cmd, o)
	if err != nil {
		return "", err
	}
	return ref.ID, nil
}

func resolvePageRef(cmd *cobra.Command, o *Opts) (*PageRef, error) {
	id, _ := cmd.Flags().GetString("id")
	u, _ := cmd.Flags().GetString("url")
	defer func() {
		_ = cmd.Flags().Set("id", "")
		_ = cmd.Flags().Set("url", "")
	}()
	if (id == "") == (u == "") {
		return nil, fmt.Errorf("invalid_args")
	}
	if id != "" {
		cx, err := loadCtx(o, "")
		if err != nil {
			return nil, err
		}
		return &PageRef{Ctx: cx, ID: id, EntityType: "page_id"}, nil
	}
	pu, err := url.Parse(u)
	if err != nil {
		return nil, fmt.Errorf("invalid_args")
	}
	cx, ent, err := loadCtxForPageURL(o, u)
	if err != nil {
		return nil, err
	}
	pid := pu.Query().Get("pageId")
	if pid != "" {
		return &PageRef{Ctx: cx, ID: pid, SourceURL: u, EntityType: "page_id"}, nil
	}
	if ent.Type == "page" {
		if pid = ent.Attrs["id"]; pid != "" {
			return &PageRef{Ctx: cx, ID: pid, Space: ent.Attrs["space"], SourceURL: u, EntityType: "page_id"}, nil
		}
		if title := ent.Attrs["title"]; title != "" {
			space, _ := url.PathUnescape(ent.Attrs["space"])
			title, _ = url.PathUnescape(title)
			title = strings.ReplaceAll(title, "+", " ")
			pid, err := lookupPageIDByTitleCtx(cx, space, title)
			if err != nil {
				return nil, err
			}
			return &PageRef{Ctx: cx, ID: pid, Space: space, Title: title, SourceURL: u, EntityType: "page_title"}, nil
		}
	}
	seg := strings.Split(strings.Trim(pu.Path, "/"), "/")
	for i := 0; i+2 < len(seg); i++ {
		if seg[i] == "display" {
			space, _ := url.PathUnescape(seg[i+1])
			title, _ := url.PathUnescape(strings.Join(seg[i+2:], "/"))
			title = strings.ReplaceAll(title, "+", " ")
			pid, err := lookupPageIDByTitleCtx(cx, space, title)
			if err != nil {
				return nil, err
			}
			return &PageRef{Ctx: cx, ID: pid, Space: space, Title: title, SourceURL: u, EntityType: "page_title"}, nil
		}
	}
	for i := len(seg) - 1; i >= 0; i-- {
		if _, e := strconv.Atoi(seg[i]); e == nil {
			return &PageRef{Ctx: cx, ID: seg[i], SourceURL: u, EntityType: "page_id"}, nil
		}
	}
	return nil, fmt.Errorf("invalid_args")
}

func loadCtxForPageURL(o *Opts, entityURL string) (*ctx, instance.ResolvedEntity, error) {
	p, _ := config.ResolvePath(o.Config)
	cfg, err := config.Load(p)
	if err != nil {
		return nil, instance.ResolvedEntity{}, err
	}
	res, err := instance.Resolve(cfg.Confluence, o.Instance, entityURL, "confluence")
	if err != nil {
		return nil, instance.ResolvedEntity{}, err
	}
	cl, err := httpclient.New(res.Instance)
	if err != nil {
		return nil, instance.ResolvedEntity{}, err
	}
	return &ctx{cfg: cfg, inst: res.Instance, client: cl}, res.Entity, nil
}

func loadCtxForConfluencePathOrURL(o *Opts, pathOrURL string) (*ctx, error) {
	if !isAbsoluteURL(pathOrURL) {
		return loadCtx(o, "")
	}
	p, _ := config.ResolvePath(o.Config)
	cfg, err := config.Load(p)
	if err != nil {
		return nil, err
	}
	res, err := instance.Resolve(cfg.Confluence, o.Instance, pathOrURL, "confluence")
	if err != nil {
		if err.Error() == "instance_required" {
			return nil, errors.New("instance_url_mismatch")
		}
		return nil, err
	}
	cl, err := httpclient.New(res.Instance)
	if err != nil {
		return nil, err
	}
	return &ctx{cfg: cfg, inst: res.Instance, client: cl}, nil
}

func lookupPageIDByTitleCtx(cx *ctx, space, title string) (string, error) {
	resp, err := cx.client.Do(httpclient.Request{Method: "GET", Path: "content", Query: map[string]string{"spaceKey": space, "title": title, "type": "page"}})
	if err != nil {
		return "", errors.New(envelopeError(err, "server_error").Error.Code)
	}
	defer resp.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("server_error")
	}
	results, ok := out["results"].([]any)
	if !ok {
		return "", fmt.Errorf("server_error")
	}
	if len(results) == 0 {
		return "", fmt.Errorf("not_found")
	}
	first, ok := results[0].(map[string]any)
	if !ok {
		return "", fmt.Errorf("server_error")
	}
	id, ok := first["id"].(string)
	if !ok || id == "" {
		return "", fmt.Errorf("server_error")
	}
	return id, nil
}

func urlBelongsToBase(raw, base string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	b, err := url.Parse(base)
	if err != nil {
		return false
	}
	if !strings.EqualFold(u.Scheme, b.Scheme) || !strings.EqualFold(u.Host, b.Host) {
		return false
	}
	basePath := "/" + strings.Trim(strings.ToLower(b.Path), "/")
	rawPath := "/" + strings.Trim(strings.ToLower(u.Path), "/")
	if basePath == "/" {
		return true
	}
	return rawPath == basePath || strings.HasPrefix(rawPath, strings.TrimRight(basePath, "/")+"/")
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
			cx, err := loadCtxForConfluencePathOrURL(o, p)
			if err != nil {
				return print(cmd, o, envelopeError(err, "config_error"))
			}
			b, err := readBody(cmd)
			if err != nil {
				return print(cmd, o, output.Failure("invalid_args", err.Error(), "", 400))
			}
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
			return doWithCtx(o, cmd, cx, method, p, q, bj)
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
