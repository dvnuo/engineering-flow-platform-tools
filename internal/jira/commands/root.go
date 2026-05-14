package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/files"
	"engineering-flow-platform-tools/internal/httpclient"
	"engineering-flow-platform-tools/internal/instance"
	"engineering-flow-platform-tools/internal/jira"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

type Opts struct {
	Instance, Config  string
	JSON, DryRun, Yes bool
}

func NewRoot() *cobra.Command {
	o := &Opts{}
	cmd := &cobra.Command{Use: "jira", SilenceErrors: true, SilenceUsage: true}
	cmd.PersistentFlags().StringVar(&o.Instance, "instance", "", "")
	cmd.PersistentFlags().StringVar(&o.Config, "config", "", "")
	cmd.PersistentFlags().BoolVar(&o.JSON, "json", false, "")
	cmd.PersistentFlags().BoolVar(&o.DryRun, "dry-run", false, "")
	cmd.PersistentFlags().BoolVar(&o.Yes, "yes", false, "")
	cmd.AddCommand(instanceCmd(o), authCmd(o), myselfCmd(o), serverInfoCmd(o), resolveCmd(o), commandsCmd(), schemaCmd(), helpLLMCmd(), issueCmd(o), rawAPICmd(o))
	return cmd
}
func fmtOut(o *Opts) string {
	if o.JSON {
		return "json"
	}
	return "table"
}
func loadCfg(o *Opts) (config.RootConfig, error) {
	p, _ := config.ResolvePath(o.Config)
	return config.Load(p)
}
func print(cmd *cobra.Command, o *Opts, env output.Envelope) error {
	return output.Print(cmd.OutOrStdout(), fmtOut(o), env)
}

func instanceCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "instance"}
	c.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		return print(cmd, o, output.Success("", map[string]interface{}{"instances": cfg.Jira.Instances}))
	}})
	c.AddCommand(&cobra.Command{Use: "get <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		for _, i := range cfg.Jira.Instances {
			if i.Name == args[0] {
				i.Auth = config.RedactAuth(i.Auth)
				return print(cmd, o, output.Success(i.Name, i))
			}
		}
		return print(cmd, o, output.Failure("not_found", "instance not found", "", 404))
	}})
	return c
}

func authCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "auth"}
	c.AddCommand(&cobra.Command{Use: "test", RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		ctx, err := jira.NewContext(cfg, o.Instance, "", false)
		if err != nil {
			return print(cmd, o, output.Failure(err.Error(), err.Error(), "", 400))
		}
		resp, err := ctx.Client.Do(httpclient.Request{Method: "GET", Path: "myself"})
		if err != nil {
			return print(cmd, o, output.Failure("auth_failed", err.Error(), "", 401))
		}
		defer resp.Body.Close()
		d, _ := jira.ReadJSON(resp.Body)
		return print(cmd, o, output.Success(ctx.Instance, d))
	}})
	return c
}

func myselfCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "myself", RunE: func(cmd *cobra.Command, args []string) error { return authCmd(o).Commands()[0].RunE(cmd, args) }}
}
func serverInfoCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "server-info", RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		ctx, err := jira.NewContext(cfg, o.Instance, "", false)
		if err != nil {
			return print(cmd, o, output.Failure(err.Error(), err.Error(), "", 400))
		}
		resp, err := ctx.Client.Do(httpclient.Request{Method: "GET", Path: "serverInfo"})
		if err != nil {
			return print(cmd, o, output.Failure("server_error", err.Error(), "", 500))
		}
		defer resp.Body.Close()
		d, _ := jira.ReadJSON(resp.Body)
		return print(cmd, o, output.Success(ctx.Instance, d))
	}}
}
func resolveCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "resolve-url <url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		r, err := instance.Resolve(cfg.Jira, o.Instance, args[0], "jira")
		if err != nil {
			return print(cmd, o, output.Failure(err.Error(), err.Error(), "", 400))
		}
		return print(cmd, o, output.Success(r.Instance.Name, r))
	}}
}
func commandsCmd() *cobra.Command {
	return &cobra.Command{Use: "commands", RunE: func(cmd *cobra.Command, args []string) error {
		return output.Print(cmd.OutOrStdout(), "json", output.Success("", map[string]interface{}{"commands": jiraCommands}))
	}}
}
func schemaCmd() *cobra.Command {
	return &cobra.Command{Use: "schema <command>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		req := []string{}
		if args[0] == "issue.create" {
			req = []string{"project", "type", "summary"}
		}
		return output.Print(cmd.OutOrStdout(), "json", output.Success("", map[string]interface{}{"command": args[0], "required": req}))
	}}
}
func helpLLMCmd() *cobra.Command {
	return &cobra.Command{Use: "help llm", RunE: func(cmd *cobra.Command, args []string) error {
		return output.Print(cmd.OutOrStdout(), "table", output.Success("", map[string]interface{}{"tips": []string{"use --json", "use --dry-run for writes"}}))
	}}
}

func issueCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "issue"}
	c.AddCommand(&cobra.Command{Use: "get <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := loadCfg(o)
		ctx, err := jira.NewContext(cfg, o.Instance, args[0], o.DryRun)
		if err != nil {
			return print(cmd, o, output.Failure(err.Error(), err.Error(), "", 400))
		}
		p := "issue/" + jira.IssueKey(args[0])
		if o.DryRun {
			return print(cmd, o, output.Success(ctx.Instance, jira.DryRunData("GET", p, nil, nil)))
		}
		resp, err := ctx.Client.Do(httpclient.Request{Method: "GET", Path: p})
		if err != nil {
			return print(cmd, o, output.Failure("server_error", err.Error(), "", 500))
		}
		defer resp.Body.Close()
		d, _ := jira.ReadJSON(resp.Body)
		return print(cmd, o, output.Success(ctx.Instance, d))
	}})
	c.AddCommand(&cobra.Command{Use: "search", RunE: func(cmd *cobra.Command, args []string) error {
		jql, _ := cmd.Flags().GetString("jql")
		if jql == "" {
			return print(cmd, o, output.Failure("invalid_args", "missing --jql", "", 400))
		}
		cfg, _ := loadCfg(o)
		ctx, _ := jira.NewContext(cfg, o.Instance, "", o.DryRun)
		body := map[string]interface{}{"jql": jql}
		if o.DryRun {
			return print(cmd, o, output.Success(ctx.Instance, jira.DryRunData("POST", "search", nil, body)))
		}
		resp, err := ctx.Client.Do(httpclient.Request{Method: "POST", Path: "search", JSONBody: body})
		if err != nil {
			return print(cmd, o, output.Failure("server_error", err.Error(), "", 500))
		}
		defer resp.Body.Close()
		d, _ := jira.ReadJSON(resp.Body)
		return print(cmd, o, output.Success(ctx.Instance, d))
	}})
	c.Commands()[1].Flags().String("jql", "", "")
	c.AddCommand(&cobra.Command{Use: "create", RunE: func(cmd *cobra.Command, args []string) error {
		project, _ := cmd.Flags().GetString("project")
		typ, _ := cmd.Flags().GetString("type")
		summary, _ := cmd.Flags().GetString("summary")
		desc, _ := cmd.Flags().GetString("description")
		fields, _ := cmd.Flags().GetStringArray("field")
		body := map[string]interface{}{"fields": map[string]interface{}{"project": map[string]string{"key": project}, "issuetype": map[string]string{"name": typ}, "summary": summary, "description": desc}}
		for _, f := range fields {
			kv := strings.SplitN(f, "=", 2)
			if len(kv) == 2 {
				var any interface{}
				if json.Unmarshal([]byte(kv[1]), &any) == nil {
					body["fields"].(map[string]interface{})[kv[0]] = any
				} else {
					body["fields"].(map[string]interface{})[kv[0]] = kv[1]
				}
			}
		}
		cfg, _ := loadCfg(o)
		ctx, _ := jira.NewContext(cfg, o.Instance, "", o.DryRun)
		if o.DryRun {
			return print(cmd, o, output.Success(ctx.Instance, jira.DryRunData("POST", "issue", nil, body)))
		}
		resp, err := ctx.Client.Do(httpclient.Request{Method: "POST", Path: "issue", JSONBody: body})
		if err != nil {
			return print(cmd, o, output.Failure("server_error", err.Error(), "", 500))
		}
		defer resp.Body.Close()
		d, _ := jira.ReadJSON(resp.Body)
		return print(cmd, o, output.Success(ctx.Instance, d))
	}})
	c.Commands()[2].Flags().String("project", "", "")
	c.Commands()[2].Flags().String("type", "", "")
	c.Commands()[2].Flags().String("summary", "", "")
	c.Commands()[2].Flags().String("description", "", "")
	c.Commands()[2].Flags().StringArray("field", nil, "")
	c.AddCommand(&cobra.Command{Use: "transition <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		to, _ := cmd.Flags().GetString("to")
		tid, _ := cmd.Flags().GetString("transition-id")
		cfg, _ := loadCfg(o)
		ctx, _ := jira.NewContext(cfg, o.Instance, args[0], o.DryRun)
		issue := jira.IssueKey(args[0])
		if tid == "" && to != "" {
			resp, err := ctx.Client.Do(httpclient.Request{Method: "GET", Path: "issue/" + issue + "/transitions"})
			if err != nil {
				return print(cmd, o, output.Failure("server_error", err.Error(), "", 500))
			}
			defer resp.Body.Close()
			d, _ := jira.ReadJSON(resp.Body)
			arr, _ := d["transitions"].([]interface{})
			for _, it := range arr {
				m := it.(map[string]interface{})
				if strings.EqualFold(fmt.Sprint(m["name"]), to) {
					tid = fmt.Sprint(m["id"])
					break
				}
			}
		}
		body := map[string]interface{}{"transition": map[string]string{"id": tid}}
		if o.DryRun {
			return print(cmd, o, output.Success(ctx.Instance, jira.DryRunData("POST", "issue/"+issue+"/transitions", nil, body)))
		}
		resp, err := ctx.Client.Do(httpclient.Request{Method: "POST", Path: "issue/" + issue + "/transitions", JSONBody: body})
		if err != nil {
			return print(cmd, o, output.Failure("server_error", err.Error(), "", 500))
		}
		defer resp.Body.Close()
		return print(cmd, o, output.Success(ctx.Instance, map[string]interface{}{"ok": true}))
	}})
	c.Commands()[3].Flags().String("to", "", "")
	c.Commands()[3].Flags().String("transition-id", "", "")
	c.AddCommand(&cobra.Command{Use: "delete <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if err := jira.RequireYes(o.Yes); err != nil {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		cfg, _ := loadCfg(o)
		ctx, _ := jira.NewContext(cfg, o.Instance, args[0], o.DryRun)
		p := "issue/" + jira.IssueKey(args[0])
		if o.DryRun {
			return print(cmd, o, output.Success(ctx.Instance, jira.DryRunData("DELETE", p, nil, nil)))
		}
		_, err := ctx.Client.Do(httpclient.Request{Method: "DELETE", Path: p})
		if err != nil {
			return print(cmd, o, output.Failure("server_error", err.Error(), "", 500))
		}
		return print(cmd, o, output.Success(ctx.Instance, map[string]interface{}{"deleted": true}))
	}})
	return c
}

func rawAPICmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "api"}
	for _, m := range []string{"get", "post", "put", "delete"} {
		method := strings.ToUpper(m)
		cc := &cobra.Command{Use: m + " <path>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			if method == "DELETE" && !o.Yes {
				return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
			}
			cfg, _ := loadCfg(o)
			ctx, _ := jira.NewContext(cfg, o.Instance, "", o.DryRun)
			qf, _ := cmd.Flags().GetStringArray("query")
			q := map[string]string{}
			for _, s := range qf {
				kv := strings.SplitN(s, "=", 2)
				if len(kv) == 2 {
					q[kv[0]] = kv[1]
				}
			}
			body, _ := files.ReadBodyFromFlags(mustS(cmd, "body"), mustS(cmd, "body-file"), mustB(cmd, "body-stdin"))
			var jb interface{}
			if body != "" {
				_ = json.Unmarshal([]byte(body), &jb)
				if jb == nil {
					jb = map[string]string{"body": body}
				}
			}
			if o.DryRun {
				return print(cmd, o, output.Success(ctx.Instance, jira.DryRunData(method, args[0], q, jb)))
			}
			_, err := ctx.Client.Do(httpclient.Request{Method: method, Path: args[0], Query: q, JSONBody: jb})
			if err != nil {
				return print(cmd, o, output.Failure("server_error", err.Error(), "", 500))
			}
			return print(cmd, o, output.Success(ctx.Instance, map[string]interface{}{"ok": true}))
		}}
		cc.Flags().StringArray("query", nil, "")
		cc.Flags().String("body", "", "")
		cc.Flags().String("body-file", "", "")
		cc.Flags().Bool("body-stdin", false, "")
		c.AddCommand(cc)
	}
	return c
}
func mustS(cmd *cobra.Command, n string) string { v, _ := cmd.Flags().GetString(n); return v }
func mustB(cmd *cobra.Command, n string) bool   { v, _ := cmd.Flags().GetBool(n); return v }

var jiraCommands = []string{"jira instance list", "jira instance get <name>", "jira auth test", "jira myself", "jira server-info", "jira resolve-url <url>", "jira commands", "jira schema <command>", "jira help llm", "jira issue get <issue-or-url>", "jira issue search", "jira issue create", "jira issue transition <issue-or-url>", "jira issue delete <issue-or-url>", "jira api get <path>", "jira api post <path>", "jira api put <path>", "jira api delete <path>"}

func ParseJSON(b []byte) map[string]interface{} {
	out := map[string]interface{}{}
	_ = json.NewDecoder(bytes.NewReader(b)).Decode(&out)
	return out
}
