package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"

	"engineering-flow-platform-tools/internal/catalog"
	"engineering-flow-platform-tools/internal/clihelp"
	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/files"
	"engineering-flow-platform-tools/internal/httpclient"
	"engineering-flow-platform-tools/internal/instance"
	"engineering-flow-platform-tools/internal/jira"
	"engineering-flow-platform-tools/internal/output"
	"engineering-flow-platform-tools/internal/version"
	"github.com/spf13/cobra"
)

type Opts struct {
	Instance, Config, Format   string
	JSON, Verbose, DryRun, Yes bool
}

func NewRoot() *cobra.Command {
	cobra.EnableCommandSorting = false
	o := &Opts{}
	cmd := &cobra.Command{Use: "jira", SilenceErrors: true, SilenceUsage: true}
	cmd.PersistentFlags().StringVar(&o.Instance, "instance", "", "")
	cmd.PersistentFlags().StringVar(&o.Config, "config", "", "")
	cmd.PersistentFlags().BoolVar(&o.JSON, "json", false, "")
	cmd.PersistentFlags().StringVar(&o.Format, "format", "table", "")
	cmd.PersistentFlags().BoolVar(&o.Verbose, "verbose", false, "")
	cmd.PersistentFlags().BoolVar(&o.DryRun, "dry-run", false, "")
	cmd.PersistentFlags().BoolVar(&o.Yes, "yes", false, "")
	cmd.AddCommand(instanceCmd(o), authCmd(o), myselfCmd(o), serverInfoCmd(o), resolveCmd(o), commandsCmd(), schemaCmd(), helpLLMCmd(), issueCmd(o), zephyrCmd(o), attachmentCmd(o), rawAPICmd(o), projectCmd(o), userGroupCmd(o), groupCmd(o), filterDashboardCmd(o), dashboardCmd(o), componentCmd(o), versionCmd(o))
	cmd.AddCommand(metadataCmds(o)...)
	cmd.AddCommand(boardCmd(o), sprintCmd(o), backlogCmd(o))
	cmd.AddCommand(hiddenCmd(commentCmd(o)), hiddenCmd(worklogCmd(o)), hiddenCmd(agileCmd(o)), hiddenCmd(metaCmd(o)))
	clihelp.ApplyCatalogHelp(cmd, clihelp.ProductHelp{
		Product: "jira",
		Binary:  "jira",
		Short:   "Operate Jira issues, projects, metadata, Agile resources, and Zephyr test management",
		Long: strings.TrimSpace(`jira is a terminal-invoked CLI for agents and scripts that need stable JSON access to Jira Server/Data Center resources.

Use it for issues, search, transitions, comments, attachments, projects, users, groups, metadata, filters, dashboards, Agile boards and sprints, and Zephyr test-management resources. Always use --json for agent workflows. Use --dry-run before write operations and --yes only after explicit user confirmation for destructive operations.

Configuration uses the shared Atlassian config file, normally ~/.config/atlassian/config.json on Linux/macOS or %APPDATA%\atlassian\config.json on Windows.`),
		Examples: []string{
			`jira issue get PROJ-123 --json`,
			`jira issue search --jql "project = PROJ ORDER BY updated DESC" --json`,
			`jira issue update PROJ-123 --summary "New summary" --dry-run --json`,
			`jira zephyr doctor --project PROJ --json`,
			`jira schema issue.update --json`,
			`jira help llm --json`,
		},
		Instructions: "copy cmd/jira/jira-cli.instructions.md to ~/.copilot/instructions/jira-cli.instructions.md.",
		Groups: map[string]string{
			"instance":           "Manage configured Jira instances.",
			"auth":               "Manage Jira credentials stored in the Atlassian config.",
			"issue":              "Work with Jira issues by key or URL.",
			"issue.comment":      "Manage comments on Jira issues.",
			"issue.attachment":   "Manage attachments on Jira issues.",
			"issue.worklog":      "Manage Jira issue worklogs.",
			"issue.link":         "Manage Jira issue links.",
			"issue.remote-link":  "Manage Jira remote issue links.",
			"issue.property":     "Read and write Jira issue properties.",
			"zephyr":             "Work with Zephyr test-management resources through Jira.",
			"zephyr.status":      "Inspect Zephyr execution status catalogs.",
			"zephyr.util":        "Inspect Zephyr utility endpoints.",
			"zephyr.test":        "Work with Jira Test issues under Zephyr.",
			"zephyr.cycle":       "Work with Zephyr test cycles.",
			"zephyr.execution":   "Work with Zephyr test executions.",
			"zephyr.zql":         "Search and inspect Zephyr ZQL metadata.",
			"zephyr.step-result": "Work with Zephyr step results.",
			"zephyr.attachment":  "Manage Zephyr attachments.",
			"zephyr.folder":      "Manage Zephyr folders.",
			"zephyr.teststep":    "Manage Zephyr test steps.",
			"zephyr.defect":      "Manage Zephyr execution defects.",
			"zephyr.report":      "Inspect Zephyr reports.",
			"zephyr.api":         "Call cataloged or raw Zephyr ZAPI endpoints.",
			"attachment":         "Read, download, or delete Jira attachments.",
			"project":            "Inspect Jira projects and project metadata.",
			"component":          "Manage Jira project components.",
			"version":            "Manage Jira project versions.",
			"user":               "Inspect and search Jira users.",
			"group":              "Inspect Jira groups and members.",
			"field":              "List Jira fields.",
			"issue-type":         "List Jira issue types.",
			"status":             "List Jira statuses.",
			"priority":           "List Jira priorities.",
			"resolution":         "List Jira resolutions.",
			"workflow":           "Inspect Jira workflows.",
			"permissions":        "Inspect Jira permissions.",
			"settings":           "Inspect Jira settings.",
			"config":             "Inspect Jira configuration metadata.",
			"filter":             "Manage Jira filters.",
			"dashboard":          "Inspect Jira dashboards.",
			"api":                "Call raw Jira REST endpoints on the selected instance.",
			"board":              "Inspect Jira Agile boards.",
			"sprint":             "Inspect Jira Agile sprints.",
			"backlog":            "Inspect Jira Agile backlog issues.",
		},
	})
	return cmd
}
func fmtOut(o *Opts) string {
	if o.JSON {
		return "json"
	}
	if o.Format != "" {
		return o.Format
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

func invalidArgs(cmd *cobra.Command, o *Opts, msg, hint string) error {
	return print(cmd, o, output.Failure("invalid_args", msg, hint, 400))
}

func readJiraBody(cmd *cobra.Command) (string, error) {
	b, err := files.ReadBodyFromFlags(mustS(cmd, "body"), mustS(cmd, "body-file"), mustB(cmd, "body-stdin"))
	if err != nil {
		switch {
		case mustS(cmd, "body-file") != "":
			return "", fmt.Errorf("failed to read --body-file: %w", err)
		case mustB(cmd, "body-stdin"):
			return "", fmt.Errorf("failed to read --body-stdin: %w", err)
		default:
			return "", fmt.Errorf("failed to read body: %w", err)
		}
	}
	return b, nil
}

func readJiraJSONValue(cmd *cobra.Command) (interface{}, error) {
	v, err := files.ReadJSONValueFromFlags(mustS(cmd, "value"), mustS(cmd, "value-file"))
	if err != nil {
		if mustS(cmd, "value-file") != "" {
			return nil, fmt.Errorf("failed to read --value-file: %w", err)
		}
		return nil, fmt.Errorf("invalid --value: %w", err)
	}
	return v, nil
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

func hiddenCmd(cmd *cobra.Command) *cobra.Command {
	cmd.Hidden = true
	return cmd
}

func instanceCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "instance"}
	c.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		for i := range cfg.Jira.Instances {
			cfg.Jira.Instances[i].Auth = config.RedactAuth(cfg.Jira.Instances[i].Auth)
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
	c.AddCommand(&cobra.Command{Use: "add <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := loadCfg(o)
		baseURL, _ := cmd.Flags().GetString("base-url")
		restPath, _ := cmd.Flags().GetString("rest-path")
		apiVersion, _ := cmd.Flags().GetString("api-version")
		auth, authErr := authFromFlags(cmd)
		if authErr != nil {
			return print(cmd, o, output.Failure("invalid_args", "missing required auth secret", "", 400))
		}
		if restPath == "" {
			restPath = "/rest/api/2"
		}
		if apiVersion == "" {
			apiVersion = "2"
		}
		in := config.InstanceConfig{Name: args[0], BaseURL: baseURL, RESTPath: restPath, APIVersion: apiVersion, Auth: auth}
		cfg.Jira.Instances = append(cfg.Jira.Instances, in)
		if mustB(cmd, "default") {
			cfg.Jira.DefaultInstance = args[0]
		}
		p, _ := config.ResolvePath(o.Config)
		if err := config.Save(p, cfg); err != nil {
			return print(cmd, o, output.Failure("config_error", err.Error(), "", 500))
		}
		return print(cmd, o, output.Success(args[0], map[string]interface{}{"added": true}))
	}})
	c.Commands()[2].Flags().String("base-url", "", "")
	c.Commands()[2].Flags().String("rest-path", "/rest/api/2", "")
	c.Commands()[2].Flags().String("api-version", "2", "")
	addAuthFlags(c.Commands()[2])
	c.Commands()[2].Flags().Bool("default", false, "")
	c.AddCommand(&cobra.Command{Use: "update <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		baseURL, _ := cmd.Flags().GetString("base-url")
		for i := range cfg.Jira.Instances {
			if cfg.Jira.Instances[i].Name == args[0] {
				if baseURL != "" {
					cfg.Jira.Instances[i].BaseURL = baseURL
				}
			}
		}
		p, _ := config.ResolvePath(o.Config)
		if err := config.Save(p, cfg); err != nil {
			return print(cmd, o, output.Failure("config_error", err.Error(), "", 500))
		}
		return print(cmd, o, output.Success(args[0], map[string]interface{}{"updated": true}))
	}})
	c.Commands()[3].Flags().String("base-url", "", "")
	c.AddCommand(&cobra.Command{Use: "remove <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		out := []config.InstanceConfig{}
		for _, in := range cfg.Jira.Instances {
			if in.Name != args[0] {
				out = append(out, in)
			}
		}
		cfg.Jira.Instances = out
		if cfg.Jira.DefaultInstance == args[0] {
			cfg.Jira.DefaultInstance = ""
		}
		p, _ := config.ResolvePath(o.Config)
		if err := config.Save(p, cfg); err != nil {
			return print(cmd, o, output.Failure("config_error", err.Error(), "", 500))
		}
		return print(cmd, o, output.Success(args[0], map[string]interface{}{"removed": true}))
	}})
	c.AddCommand(&cobra.Command{Use: "default [name]", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		if len(args) == 0 {
			return print(cmd, o, output.Success("", map[string]interface{}{"default_instance": cfg.Jira.DefaultInstance}))
		}
		cfg.Jira.DefaultInstance = args[0]
		p, _ := config.ResolvePath(o.Config)
		if err := config.Save(p, cfg); err != nil {
			return print(cmd, o, output.Failure("config_error", err.Error(), "", 500))
		}
		return print(cmd, o, output.Success(args[0], map[string]interface{}{"default_instance": args[0]}))
	}})
	return c
}

func authCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "auth"}
	c.AddCommand(&cobra.Command{Use: "login", RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		auth, authErr := authFromFlags(cmd)
		if authErr != nil {
			return print(cmd, o, output.Failure("invalid_args", "missing required auth secret", "", 400))
		}
		for i := range cfg.Jira.Instances {
			if cfg.Jira.Instances[i].Name == cfg.Jira.DefaultInstance || o.Instance == cfg.Jira.Instances[i].Name {
				cfg.Jira.Instances[i].Auth = auth
			}
		}
		p, _ := config.ResolvePath(o.Config)
		if err := config.Save(p, cfg); err != nil {
			return print(cmd, o, output.Failure("config_error", err.Error(), "", 500))
		}
		return print(cmd, o, output.Success("", map[string]interface{}{"logged_in": true}))
	}})
	addAuthFlags(c.Commands()[0])
	c.AddCommand(&cobra.Command{Use: "logout", RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		for i := range cfg.Jira.Instances {
			if cfg.Jira.Instances[i].Name == cfg.Jira.DefaultInstance || o.Instance == cfg.Jira.Instances[i].Name {
				cfg.Jira.Instances[i].Auth = config.AuthConfig{}
			}
		}
		p, _ := config.ResolvePath(o.Config)
		if err := config.Save(p, cfg); err != nil {
			return print(cmd, o, output.Failure("config_error", err.Error(), "", 500))
		}
		return print(cmd, o, output.Success("", map[string]interface{}{"logged_out": true}))
	}})
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
	return &cobra.Command{Use: "myself", RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		ctx, err := jira.NewContext(cfg, o.Instance, "", false)
		if err != nil {
			return print(cmd, o, output.Failure(err.Error(), err.Error(), "", 400))
		}
		resp, err := ctx.Client.Do(httpclient.Request{Method: "GET", Path: "/rest/api/2/myself"})
		if err != nil {
			return print(cmd, o, envelopeError(err, "auth_failed"))
		}
		defer resp.Body.Close()
		d, _ := jira.ReadJSON(resp.Body)
		return print(cmd, o, output.Success(ctx.Instance, d))
	}}
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
			return print(cmd, o, envelopeError(err, "server_error"))
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
		return output.Print(cmd.OutOrStdout(), "json", output.Success("", map[string]interface{}{"commands": catalog.CommandsFromCobra("jira", cmd.Root())}))
	}}
}
func cliVersionCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "version", RunE: func(cmd *cobra.Command, args []string) error {
		return print(cmd, o, output.Success("", map[string]any{"version": version.Version, "commit": version.Commit, "date": version.Date}))
	}}
}
func schemaCmd() *cobra.Command {
	return &cobra.Command{Use: "schema <command>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		schema, ok := catalog.SchemaFromCobra("jira", args[0], cmd.Root())
		if !ok {
			return output.Print(cmd.OutOrStdout(), "json", output.Failure("not_found", "command not found", "Run jira commands --json to list command names.", 404))
		}
		return output.Print(cmd.OutOrStdout(), "json", output.Success("", schema))
	}}
}
func helpLLMCmd() *cobra.Command {
	return &cobra.Command{Use: "help llm", RunE: func(cmd *cobra.Command, args []string) error {
		tips := []string{
			"Always use --json for machine-readable output.",
			"Use --instance when multiple instances are configured.",
			"Full Jira/Confluence URLs can auto-select the instance.",
			"Use --dry-run before write operations.",
			"Use --yes for destructive operations.",
			"Inspect error.code and error.hint before retrying.",
			"Command parsing failures return an invalid_args JSON envelope when --json is present.",
			"On Windows cmd, use double quotes, cmd-native commands such as where/dir/cd/type, and avoid Bash-only quoting.",
			"If terminal output capture is unreliable, rerun the exact .exe path from where jira and redirect stdout to a file, then inspect the JSON envelope.",
			"If a Jira URL contains selectedItem=com.thed.zephyr.je, treat it as a Zephyr test-management page.",
			"Use jira zephyr doctor --project <PROJECT> --json first for a Jira project you have not inspected.",
			"Use Jira core commands for issues, stories, bugs, comments, attachments, and workflows.",
			"Use jira zephyr commands for test cycles, executions, execution status, step results, defects, attachments, ZQL, reports, and test summaries.",
			"A Zephyr Test Cycle is a Zephyr execution container, not a Jira issue.",
			"To update case X in cycle Y, use jira zephyr execution update-status --cycle-id Y --issue X --status PASSED --json.",
			"Use jira zephyr execution resolve before writes when the target execution is uncertain.",
			"Use jira zephyr cycle resolve when the user gives a cycle name instead of a cycle id.",
			"Use jira zephyr status list instead of hard-coding numeric Zephyr status ids.",
			"Use jira zephyr api catalog and jira zephyr api describe for official long-tail ZAPI endpoint discovery.",
			"Use --dry-run before jira zephyr write operations unless the user already approved the action.",
			"Zephyr delete commands and raw jira zephyr api delete require --yes after explicit confirmation.",
			"Do not browser-scrape Jira Test pages unless the API is unavailable and the user explicitly asks for UI investigation.",
		}
		return output.Print(cmd.OutOrStdout(), "json", output.Success("", map[string]interface{}{"tips": tips, "commands": catalog.Commands("jira")}))
	}}
}

func issueCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "issue"}
	getCmd := &cobra.Command{Use: "get <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		q := map[string]string{}
		if fields := mustS(cmd, "fields"); fields != "" {
			q["fields"] = fields
		}
		if expand := mustS(cmd, "expand"); expand != "" {
			q["expand"] = expand
		}
		return issueEntityGet(o, cmd, args[0], "", q)
	}}
	getCmd.Flags().String("fields", "", "")
	getCmd.Flags().String("expand", "", "")
	c.AddCommand(getCmd)
	c.AddCommand(&cobra.Command{Use: "search", RunE: func(cmd *cobra.Command, args []string) error {
		jql, _ := cmd.Flags().GetString("jql")
		if jql == "" {
			return print(cmd, o, output.Failure("invalid_args", "missing --jql", "", 400))
		}
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		ctx, err := jira.NewContext(cfg, o.Instance, "", o.DryRun)
		if err != nil {
			return print(cmd, o, output.Failure(err.Error(), err.Error(), "", 400))
		}
		body := map[string]interface{}{"jql": jql}
		if limit, _ := cmd.Flags().GetString("limit"); limit != "" {
			n, err := strconv.Atoi(limit)
			if err != nil {
				return print(cmd, o, output.Failure("invalid_args", "--limit must be an integer", "", 400))
			}
			body["maxResults"] = n
		}
		if start, _ := cmd.Flags().GetString("start"); start != "" {
			n, err := strconv.Atoi(start)
			if err != nil {
				return print(cmd, o, output.Failure("invalid_args", "--start must be an integer", "", 400))
			}
			body["startAt"] = n
		}
		if fields, _ := cmd.Flags().GetStringArray("fields"); len(fields) > 0 {
			body["fields"] = fields
		}
		if o.DryRun {
			return print(cmd, o, output.Success(ctx.Instance, jira.DryRunData("POST", "search", nil, body)))
		}
		resp, err := ctx.Client.Do(httpclient.Request{Method: "POST", Path: "search", JSONBody: body})
		if err != nil {
			return print(cmd, o, envelopeError(err, "server_error"))
		}
		defer resp.Body.Close()
		d, _ := jira.ReadJSON(resp.Body)
		return print(cmd, o, output.Success(ctx.Instance, d))
	}})
	c.Commands()[1].Flags().String("jql", "", "")
	c.Commands()[1].Flags().String("limit", "", "")
	c.Commands()[1].Flags().String("start", "", "")
	c.Commands()[1].Flags().StringArray("fields", nil, "")
	c.AddCommand(&cobra.Command{Use: "create", RunE: func(cmd *cobra.Command, args []string) error {
		if raw := mustS(cmd, "json-body"); raw != "" {
			var override map[string]interface{}
			if err := json.Unmarshal([]byte(raw), &override); err != nil {
				return print(cmd, o, output.Failure("invalid_args", "invalid --json-body", "", 400))
			}
			return issueCreateWithBody(o, cmd, override)
		}
		if bodyFile := mustS(cmd, "json-body-file"); bodyFile != "" {
			b, err := os.ReadFile(bodyFile)
			if err != nil {
				return print(cmd, o, output.Failure("invalid_args", err.Error(), "", 400))
			}
			var override map[string]interface{}
			if err := json.Unmarshal(b, &override); err != nil {
				return print(cmd, o, output.Failure("invalid_args", "invalid --json-body-file", "", 400))
			}
			return issueCreateWithBody(o, cmd, override)
		}
		project, _ := cmd.Flags().GetString("project")
		typ, _ := cmd.Flags().GetString("type")
		summary, _ := cmd.Flags().GetString("summary")
		desc, _ := cmd.Flags().GetString("description")
		if project == "" || typ == "" || summary == "" {
			return print(cmd, o, output.Failure("invalid_args", "--project, --type, and --summary required", "", 400))
		}
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
		return issueCreateWithBody(o, cmd, body)
	}})
	c.Commands()[2].Flags().String("project", "", "")
	c.Commands()[2].Flags().String("type", "", "")
	c.Commands()[2].Flags().String("summary", "", "")
	c.Commands()[2].Flags().String("description", "", "")
	c.Commands()[2].Flags().StringArray("field", nil, "")
	c.Commands()[2].Flags().String("json-body", "", "")
	c.Commands()[2].Flags().String("json-body-file", "", "")
	c.AddCommand(&cobra.Command{Use: "transition <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		to, _ := cmd.Flags().GetString("to")
		tid, _ := cmd.Flags().GetString("transition-id")
		if tid == "" && to == "" {
			return print(cmd, o, output.Failure("invalid_args", "--transition-id or --to required", "Use jira issue transitions <issue> --json to inspect available transitions.", 400))
		}
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		ctx, err := jira.NewContext(cfg, o.Instance, args[0], o.DryRun)
		if err != nil {
			return print(cmd, o, output.Failure(err.Error(), err.Error(), "", 400))
		}
		issue := jira.IssueKey(args[0])
		if tid == "" && to != "" {
			resp, err := ctx.Client.Do(httpclient.Request{Method: "GET", Path: "issue/" + issue + "/transitions"})
			if err != nil {
				return print(cmd, o, envelopeError(err, "server_error"))
			}
			defer resp.Body.Close()
			d, _ := jira.ReadJSON(resp.Body)
			arr, ok := d["transitions"].([]interface{})
			if !ok {
				return print(cmd, o, output.Failure("server_error", "transitions response missing array", "Use jira issue transitions <issue> --json to inspect available transitions.", 500))
			}
			for _, it := range arr {
				m, ok := it.(map[string]interface{})
				if !ok {
					continue
				}
				if strings.EqualFold(fmt.Sprint(m["name"]), to) {
					tid = fmt.Sprint(m["id"])
					break
				}
			}
		}
		if tid == "" {
			return print(cmd, o, output.Failure("invalid_args", "transition not found", "Use jira issue transitions to inspect available transitions.", 400))
		}
		body := map[string]interface{}{"transition": map[string]string{"id": tid}}
		if fields := parseKeyValueFields(mustStringArray(cmd, "field")); len(fields) > 0 {
			body["fields"] = fields
		}
		if comment := mustS(cmd, "comment"); comment != "" {
			body["update"] = map[string]interface{}{"comment": []map[string]interface{}{{"add": map[string]string{"body": comment}}}}
		}
		if o.DryRun {
			return print(cmd, o, output.Success(ctx.Instance, jira.DryRunData("POST", "issue/"+issue+"/transitions", nil, body)))
		}
		resp, err := ctx.Client.Do(httpclient.Request{Method: "POST", Path: "issue/" + issue + "/transitions", JSONBody: body})
		if err != nil {
			return print(cmd, o, envelopeError(err, "server_error"))
		}
		defer resp.Body.Close()
		return print(cmd, o, output.Success(ctx.Instance, map[string]interface{}{"ok": true}))
	}})
	c.Commands()[3].Flags().String("to", "", "")
	c.Commands()[3].Flags().String("transition-id", "", "")
	c.Commands()[3].Flags().String("comment", "", "")
	c.Commands()[3].Flags().StringArray("field", nil, "")
	c.AddCommand(&cobra.Command{Use: "delete <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if err := jira.RequireYes(o.Yes); err != nil {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		ctx, err := jira.NewContext(cfg, o.Instance, args[0], o.DryRun)
		if err != nil {
			return print(cmd, o, output.Failure(err.Error(), err.Error(), "", 400))
		}
		p := "issue/" + jira.IssueKey(args[0])
		if o.DryRun {
			return print(cmd, o, output.Success(ctx.Instance, jira.DryRunData("DELETE", p, nil, nil)))
		}
		_, err = ctx.Client.Do(httpclient.Request{Method: "DELETE", Path: p})
		if err != nil {
			return print(cmd, o, envelopeError(err, "server_error"))
		}
		return print(cmd, o, output.Success(ctx.Instance, map[string]interface{}{"deleted": true}))
	}})

	c.AddCommand(&cobra.Command{Use: "update <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		summary, _ := cmd.Flags().GetString("summary")
		desc, _ := cmd.Flags().GetString("description")
		fields, _ := cmd.Flags().GetStringArray("field")
		body := map[string]interface{}{"fields": map[string]interface{}{}}
		if raw := mustS(cmd, "json-body"); raw != "" {
			if err := json.Unmarshal([]byte(raw), &body); err != nil {
				return print(cmd, o, output.Failure("invalid_args", "invalid --json-body", "", 400))
			}
		} else if bodyFile := mustS(cmd, "json-body-file"); bodyFile != "" {
			b, err := os.ReadFile(bodyFile)
			if err != nil {
				return print(cmd, o, output.Failure("invalid_args", err.Error(), "", 400))
			}
			if err := json.Unmarshal(b, &body); err != nil {
				return print(cmd, o, output.Failure("invalid_args", "invalid --json-body-file", "", 400))
			}
		} else {
			if summary == "" && desc == "" && len(fields) == 0 {
				return print(cmd, o, output.Failure("invalid_args", "--summary, --description, --field, --json-body, or --json-body-file required", "", 400))
			}
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
			if summary != "" {
				body["fields"].(map[string]interface{})["summary"] = summary
			}
			if desc != "" {
				body["fields"].(map[string]interface{})["description"] = desc
			}
		}
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		ctx, err := jira.NewContext(cfg, o.Instance, args[0], o.DryRun)
		if err != nil {
			return print(cmd, o, output.Failure(err.Error(), err.Error(), "", 400))
		}
		p := "issue/" + jira.IssueKey(args[0])
		if o.DryRun {
			return print(cmd, o, output.Success(ctx.Instance, jira.DryRunData("PUT", p, nil, body)))
		}
		_, err = ctx.Client.Do(httpclient.Request{Method: "PUT", Path: p, JSONBody: body})
		if err != nil {
			return print(cmd, o, envelopeError(err, "server_error"))
		}
		return print(cmd, o, output.Success(ctx.Instance, map[string]interface{}{"updated": true}))
	}})
	c.Commands()[5].Flags().String("summary", "", "")
	c.Commands()[5].Flags().String("description", "", "")
	c.Commands()[5].Flags().StringArray("field", nil, "")
	c.Commands()[5].Flags().String("json-body", "", "")
	c.Commands()[5].Flags().String("json-body-file", "", "")
	c.AddCommand(&cobra.Command{Use: "assign <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		user, _ := cmd.Flags().GetString("user")
		if user == "" {
			return print(cmd, o, output.Failure("invalid_args", "--user required", "", 400))
		}
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		ctx, err := jira.NewContext(cfg, o.Instance, args[0], o.DryRun)
		if err != nil {
			return print(cmd, o, output.Failure(err.Error(), err.Error(), "", 400))
		}
		p := "issue/" + jira.IssueKey(args[0]) + "/assignee"
		b := map[string]string{"name": user}
		if o.DryRun {
			return print(cmd, o, output.Success(ctx.Instance, jira.DryRunData("PUT", p, nil, b)))
		}
		_, err = ctx.Client.Do(httpclient.Request{Method: "PUT", Path: p, JSONBody: b})
		if err != nil {
			return print(cmd, o, envelopeError(err, "server_error"))
		}
		return print(cmd, o, output.Success(ctx.Instance, map[string]interface{}{"assigned": user}))
	}})
	c.Commands()[6].Flags().String("user", "", "")
	c.AddCommand(&cobra.Command{Use: "transitions <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		ctx, err := jira.NewContext(cfg, o.Instance, args[0], o.DryRun)
		if err != nil {
			return print(cmd, o, output.Failure(err.Error(), err.Error(), "", 400))
		}
		p := "issue/" + jira.IssueKey(args[0]) + "/transitions"
		if o.DryRun {
			return print(cmd, o, output.Success(ctx.Instance, jira.DryRunData("GET", p, nil, nil)))
		}
		resp, err := ctx.Client.Do(httpclient.Request{Method: "GET", Path: p})
		if err != nil {
			return print(cmd, o, envelopeError(err, "server_error"))
		}
		defer resp.Body.Close()
		d, _ := jira.ReadJSON(resp.Body)
		return print(cmd, o, output.Success(ctx.Instance, d))
	}})
	c.AddCommand(&cobra.Command{Use: "changelog <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return issueEntityGet(o, cmd, args[0], "/changelog", nil)
	}})
	c.AddCommand(&cobra.Command{Use: "fields <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return issueEntityGet(o, cmd, args[0], "", map[string]string{"expand": "names,schema"})
	}})
	createMetaCmd := &cobra.Command{Use: "createmeta", RunE: func(cmd *cobra.Command, args []string) error {
		return issueCreateMeta(o, cmd)
	}}
	createMetaCmd.Flags().String("project", "", "")
	createMetaCmd.Flags().String("project-id", "", "")
	createMetaCmd.Flags().String("type", "", "")
	createMetaCmd.Flags().String("type-id", "", "")
	createMetaCmd.Flags().String("from-issue", "", "")
	createMetaCmd.Flags().Bool("legacy", false, "")
	createMetaCmd.Flags().String("expand", "", "")
	c.AddCommand(createMetaCmd)
	c.AddCommand(&cobra.Command{Use: "editmeta <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return issueEntityGet(o, cmd, args[0], "/editmeta", nil)
	}})
	c.AddCommand(&cobra.Command{Use: "edit <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return print(cmd, o, output.Failure("not_interactive_supported", "interactive editor is not supported", "use jira issue update", 400))
	}})

	c.AddCommand(&cobra.Command{Use: "watchers <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return issueEntityGet(o, cmd, args[0], "/watchers", nil)
	}})
	c.AddCommand(&cobra.Command{Use: "watch <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		user, _ := cmd.Flags().GetString("user")
		if user == "" {
			user = "current"
		}
		return issueEntityWrite(o, cmd, "POST", args[0], "/watchers", nil, user)
	}})
	c.AddCommand(&cobra.Command{Use: "unwatch <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		user, _ := cmd.Flags().GetString("user")
		if user == "" {
			user = "current"
		}
		return issueEntityWrite(o, cmd, "DELETE", args[0], "/watchers", map[string]string{"username": user}, nil)
	}})
	c.AddCommand(&cobra.Command{Use: "votes <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return issueEntityGet(o, cmd, args[0], "/votes", nil)
	}})
	c.AddCommand(&cobra.Command{Use: "vote <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return issueEntityWrite(o, cmd, "POST", args[0], "/votes", nil, map[string]any{})
	}})
	c.AddCommand(&cobra.Command{Use: "unvote <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return issueEntityWrite(o, cmd, "DELETE", args[0], "/votes", nil, nil)
	}})
	notifyCmd := &cobra.Command{Use: "notify <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		subj, _ := cmd.Flags().GetString("subject")
		if subj == "" {
			return invalidArgs(cmd, o, "--subject required", "Use jira issue notify PROJ-123 --subject 'Review' --body 'Please review' --to alice --json.")
		}
		if mustS(cmd, "body") == "" && mustS(cmd, "body-file") == "" && !mustB(cmd, "body-stdin") {
			return invalidArgs(cmd, o, "--body, --body-file, or --body-stdin required", "Use jira issue notify PROJ-123 --subject 'Review' --body 'Please review' --to alice --json.")
		}
		b, err := readJiraBody(cmd)
		if err != nil {
			return invalidArgs(cmd, o, err.Error(), "")
		}
		to, _ := cmd.Flags().GetStringArray("to")
		if len(to) == 0 {
			return invalidArgs(cmd, o, "--to required", "Use jira issue notify PROJ-123 --subject 'Review' --body 'Please review' --to alice --json.")
		}
		body := map[string]any{"subject": subj, "textBody": b, "to": to}
		return issueEntityWrite(o, cmd, "POST", args[0], "/notify", nil, body)
	}}
	notifyCmd.Flags().String("subject", "", "")
	notifyCmd.Flags().String("body", "", "")
	notifyCmd.Flags().String("body-file", "", "")
	notifyCmd.Flags().Bool("body-stdin", false, "")
	notifyCmd.Flags().StringArray("to", nil, "")
	c.AddCommand(notifyCmd)
	c.AddCommand(issueCommentCmd(o))
	c.AddCommand(issueAttachmentCmd(o))
	c.AddCommand(issueWorklogCmd(o))
	c.AddCommand(issueLinkCmd(o))
	c.AddCommand(issueRemoteLinkCmd(o))
	c.AddCommand(issuePropertyCmd(o))
	c.AddCommand(issueMapCSVCmd(o), issueBulkCreateCmd(o), issueBulkValidateCmd(o))

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
			cfg, err := loadCfg(o)
			if err != nil {
				return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
			}
			ctx, err := jira.NewContext(cfg, o.Instance, "", o.DryRun)
			if err != nil {
				return print(cmd, o, output.Failure(err.Error(), err.Error(), "", 400))
			}
			qf, _ := cmd.Flags().GetStringArray("query")
			q := map[string]string{}
			for _, s := range qf {
				kv := strings.SplitN(s, "=", 2)
				if len(kv) == 2 {
					q[kv[0]] = kv[1]
				}
			}
			body, err := readJiraBody(cmd)
			if err != nil {
				return invalidArgs(cmd, o, err.Error(), "")
			}
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
			_, err = ctx.Client.Do(httpclient.Request{Method: method, Path: args[0], Query: q, JSONBody: jb})
			if err != nil {
				return print(cmd, o, envelopeError(err, "server_error"))
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
func mustStringArray(cmd *cobra.Command, n string) []string {
	v, _ := cmd.Flags().GetStringArray(n)
	return v
}

func parseKeyValueFields(values []string) map[string]interface{} {
	out := map[string]interface{}{}
	for _, f := range values {
		kv := strings.SplitN(f, "=", 2)
		if len(kv) != 2 || strings.TrimSpace(kv[0]) == "" {
			continue
		}
		var any interface{}
		if json.Unmarshal([]byte(kv[1]), &any) == nil {
			out[kv[0]] = any
		} else {
			out[kv[0]] = kv[1]
		}
	}
	return out
}

func issueCreateWithBody(o *Opts, cmd *cobra.Command, body map[string]interface{}) error {
	cfg, err := loadCfg(o)
	if err != nil {
		return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
	}
	ctx, err := jira.NewContext(cfg, o.Instance, "", o.DryRun)
	if err != nil {
		return print(cmd, o, output.Failure(err.Error(), err.Error(), "", 400))
	}
	if o.DryRun {
		return print(cmd, o, output.Success(ctx.Instance, jira.DryRunData("POST", "issue", nil, body)))
	}
	resp, err := ctx.Client.Do(httpclient.Request{Method: "POST", Path: "issue", JSONBody: body})
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	defer resp.Body.Close()
	d, _ := jira.ReadJSON(resp.Body)
	return print(cmd, o, output.Success(ctx.Instance, d))
}

type createMetaOptions struct {
	ProjectKey string
	ProjectID  string
	TypeName   string
	TypeID     string
	FromIssue  string
	Legacy     bool
	Expand     string
}

const createMetaUnavailableCSVHint = "The example issue exists, but Jira createmeta endpoints are unavailable for this instance. For CSV bulk-create, run `jira issue map-csv --metadata-mode auto ...` to use editmeta-degraded fallback. (createmeta unavailable)"

func issueCreateMeta(o *Opts, cmd *cobra.Command) error {
	opts := createMetaOptions{
		ProjectKey: mustS(cmd, "project"),
		ProjectID:  mustS(cmd, "project-id"),
		TypeName:   mustS(cmd, "type"),
		TypeID:     mustS(cmd, "type-id"),
		FromIssue:  mustS(cmd, "from-issue"),
		Legacy:     mustB(cmd, "legacy"),
		Expand:     mustS(cmd, "expand"),
	}
	if opts.ProjectKey == "" && opts.ProjectID == "" && opts.TypeName == "" && opts.TypeID == "" && opts.FromIssue == "" && !opts.Legacy && opts.Expand == "" {
		return issueSubGet(o, cmd, "issue/createmeta")
	}

	cfg, err := loadCfg(o)
	if err != nil {
		return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
	}
	ctxEntity := opts.FromIssue
	ctx, err := jira.NewContext(cfg, o.Instance, ctxEntity, o.DryRun)
	if err != nil {
		return print(cmd, o, output.Failure(err.Error(), err.Error(), "", 400))
	}
	if o.DryRun {
		return print(cmd, o, output.Success(ctx.Instance, createMetaDryRun(opts)))
	}

	if opts.FromIssue != "" {
		if err := populateCreateMetaFromIssue(ctx, &opts); err != nil {
			return print(cmd, o, envelopeError(err, "server_error"))
		}
	}
	fallbackFromSplitError := false
	var splitFallbackErr error
	if !opts.Legacy {
		out, err := fetchSplitCreateMeta(ctx, opts)
		if err == nil {
			if len(mapValue(out, "fields")) > 0 {
				return print(cmd, o, output.Success(ctx.Instance, out))
			}
			legacy, legacyErr := fetchLegacyCreateMeta(ctx, opts)
			if legacyErr != nil {
				return print(cmd, o, envelopeError(createMetaUnavailableHintError(legacyErr, opts), "server_error"))
			}
			if err := ensureCreateMetaFields(legacy); err != nil {
				return print(cmd, o, envelopeError(err, "server_error"))
			}
			legacy["source"] = "legacy_after_empty_split"
			legacy["warnings"] = []interface{}{map[string]interface{}{
				"code":    "split_createmeta_empty_fields",
				"message": "Split createmeta returned no fields; legacy createmeta was used.",
			}}
			return print(cmd, o, output.Success(ctx.Instance, legacy))
		}
		if !isCreateMetaFallbackError(err) {
			return print(cmd, o, envelopeError(err, "server_error"))
		}
		fallbackFromSplitError = true
		splitFallbackErr = err
	}
	out, err := fetchLegacyCreateMeta(ctx, opts)
	if err != nil {
		return print(cmd, o, envelopeError(createMetaUnavailableHintError(err, opts), "server_error"))
	}
	if err := ensureCreateMetaFields(out); err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	if fallbackFromSplitError {
		out["source"] = "legacy_after_split_fallback_error"
		out["warnings"] = []interface{}{splitCreateMetaFallbackWarning(splitFallbackErr)}
	}
	return print(cmd, o, output.Success(ctx.Instance, out))
}

func ensureCreateMetaFields(out map[string]interface{}) error {
	if len(mapValue(out, "fields")) == 0 {
		return createMetaFieldsEmptyError()
	}
	return nil
}

func createMetaFieldsEmptyError() error {
	return &httpclient.HTTPError{
		Code:    "createmeta_fields_empty",
		Message: "Jira createmeta returned no creatable fields for the project and issue type.",
		Hint:    "Verify Jira create screens or run with --legacy and inspect the response.",
		Status:  400,
	}
}

func createMetaUnavailableHintError(err error, opts createMetaOptions) error {
	if opts.FromIssue == "" || !isCreateMetaFallbackError(err) {
		return err
	}
	var httpErr *httpclient.HTTPError
	if errors.As(err, &httpErr) {
		hinted := *httpErr
		hinted.Hint = createMetaUnavailableCSVHint
		return &hinted
	}
	return &httpclient.HTTPError{
		Code:    "invalid_args",
		Message: err.Error(),
		Hint:    createMetaUnavailableCSVHint,
		Status:  400,
	}
}

func createMetaDryRun(opts createMetaOptions) map[string]interface{} {
	requests := []interface{}{}
	if opts.FromIssue != "" {
		issuePath := "issue/" + jira.IssueKey(opts.FromIssue)
		requests = append(requests, jira.DryRunData("GET", issuePath, map[string]string{"fields": "project,issuetype", "expand": "names,schema"}, nil))
		fullIssue := jira.DryRunData("GET", issuePath, map[string]string{"fields": "*all", "expand": "names,schema,editmeta"}, nil)
		fullIssue["fallback_candidate"] = true
		requests = append(requests, fullIssue)
	}
	projectRef := opts.ProjectID
	if projectRef == "" {
		projectRef = opts.ProjectKey
	}
	if projectRef == "" && opts.FromIssue != "" {
		projectRef = "{projectIdOrKey}"
	}
	if !opts.Legacy && projectRef != "" {
		base := "issue/createmeta/" + dryRunPathEscape(projectRef) + "/issuetypes"
		requests = append(requests, jira.DryRunData("GET", base, nil, nil))
		typeRef := opts.TypeID
		if typeRef == "" && (opts.TypeName != "" || opts.FromIssue != "") {
			typeRef = "{issueTypeId}"
		}
		if typeRef != "" {
			requests = append(requests, jira.DryRunData("GET", base+"/"+dryRunPathEscape(typeRef), nil, nil))
		}
		if opts.FromIssue != "" {
			legacy := jira.DryRunData("GET", "issue/createmeta", legacyCreateMetaDryRunQuery(opts), nil)
			legacy["fallback_candidate"] = true
			requests = append(requests, legacy)
		}
	} else {
		requests = append(requests, jira.DryRunData("GET", "issue/createmeta", legacyCreateMetaDryRunQuery(opts), nil))
	}
	return map[string]interface{}{"dry_run": true, "requests": requests}
}

func populateCreateMetaFromIssue(ctx *jira.Context, opts *createMetaOptions) error {
	d, err := readCreateMetaFromIssue(ctx, opts.FromIssue, "project,issuetype", "names,schema")
	if err != nil {
		if !isIssueReadMissingError(err) {
			return err
		}
		d, err = readCreateMetaFromIssue(ctx, opts.FromIssue, "*all", "names,schema,editmeta")
		if err != nil {
			return fromIssueReadError(opts.FromIssue, err)
		}
	}
	fields := mapValue(d, "fields")
	project := mapValue(fields, "project")
	issueType := mapValue(fields, "issuetype")
	if opts.ProjectKey == "" {
		opts.ProjectKey = stringValue(project, "key")
	}
	if opts.ProjectID == "" {
		opts.ProjectID = stringValue(project, "id")
	}
	if opts.TypeName == "" {
		opts.TypeName = stringValue(issueType, "name")
	}
	if opts.TypeID == "" {
		opts.TypeID = stringValue(issueType, "id")
	}
	return nil
}

func readCreateMetaFromIssue(ctx *jira.Context, issueKey, fields, expand string) (map[string]interface{}, error) {
	resp, err := ctx.Client.Do(httpclient.Request{
		Method: "GET",
		Path:   "issue/" + jira.IssueKey(issueKey),
		Query:  map[string]string{"fields": fields, "expand": expand},
	})
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		return nil, err
	}
	defer resp.Body.Close()
	d, _ := jira.ReadJSON(resp.Body)
	return d, nil
}

func fetchSplitCreateMeta(ctx *jira.Context, opts createMetaOptions) (map[string]interface{}, error) {
	projectRef := opts.ProjectID
	if projectRef == "" {
		projectRef = opts.ProjectKey
	}
	if projectRef == "" {
		return nil, fmt.Errorf("invalid_args")
	}
	base := "issue/createmeta/" + url.PathEscape(projectRef) + "/issuetypes"
	issueTypesResp, err := getJiraMap(ctx, base, nil)
	if err != nil {
		return nil, err
	}
	selected := selectCreateMetaIssueType(issueTypesResp, opts.TypeID, opts.TypeName)
	if opts.TypeID == "" {
		opts.TypeID = stringValue(selected, "id")
	}
	if opts.TypeName == "" {
		opts.TypeName = stringValue(selected, "name")
	}
	if opts.TypeID == "" {
		return nil, fmt.Errorf("invalid_args")
	}
	fieldsResp, err := getJiraMap(ctx, base+"/"+url.PathEscape(opts.TypeID), nil)
	if err != nil {
		return nil, err
	}
	fields := mapValue(fieldsResp, "fields")
	project := map[string]interface{}{"key": opts.ProjectKey, "id": opts.ProjectID}
	issueType := map[string]interface{}{"id": opts.TypeID, "name": opts.TypeName}
	for _, key := range []string{"id", "name"} {
		if issueType[key] == "" {
			if v := stringValue(selected, key); v != "" {
				issueType[key] = v
			}
		}
	}
	return map[string]interface{}{
		"project":   project,
		"issuetype": issueType,
		"fields":    fields,
		"source":    "split",
	}, nil
}

func fetchLegacyCreateMeta(ctx *jira.Context, opts createMetaOptions) (map[string]interface{}, error) {
	return fetchLegacyCreateMetaPath(ctx, opts, "issue/createmeta")
}

func fetchRawLegacyCreateMeta(ctx *jira.Context, opts createMetaOptions) (map[string]interface{}, error) {
	return fetchLegacyCreateMetaPath(ctx, opts, "/rest/api/2/issue/createmeta")
}

func fetchLegacyCreateMetaPath(ctx *jira.Context, opts createMetaOptions, path string) (map[string]interface{}, error) {
	resp, err := getJiraMap(ctx, path, legacyCreateMetaQuery(opts))
	if err != nil {
		return nil, err
	}
	project, issueType := selectLegacyCreateMeta(resp, opts)
	fields := mapValue(issueType, "fields")
	return map[string]interface{}{
		"project": map[string]interface{}{
			"key": firstNonEmpty(stringValue(project, "key"), opts.ProjectKey),
			"id":  firstNonEmpty(stringValue(project, "id"), opts.ProjectID),
		},
		"issuetype": map[string]interface{}{
			"id":   firstNonEmpty(stringValue(issueType, "id"), opts.TypeID),
			"name": firstNonEmpty(stringValue(issueType, "name"), opts.TypeName),
		},
		"fields": fields,
		"source": "legacy",
	}, nil
}

func legacyCreateMetaQuery(opts createMetaOptions) map[string]string {
	q := map[string]string{}
	if opts.ProjectID != "" {
		q["projectIds"] = opts.ProjectID
	} else if opts.ProjectKey != "" {
		q["projectKeys"] = opts.ProjectKey
	}
	if opts.TypeID != "" {
		q["issuetypeIds"] = opts.TypeID
	} else if opts.TypeName != "" {
		q["issuetypeNames"] = opts.TypeName
	}
	if opts.Expand != "" {
		q["expand"] = opts.Expand
	} else {
		q["expand"] = "projects.issuetypes.fields"
	}
	return q
}

func legacyCreateMetaDryRunQuery(opts createMetaOptions) map[string]string {
	q := legacyCreateMetaQuery(opts)
	if opts.FromIssue == "" {
		return q
	}
	if _, ok := q["projectIds"]; !ok {
		if _, ok := q["projectKeys"]; !ok {
			q["projectIds"] = "{projectIdOrKey}"
		}
	}
	if _, ok := q["issuetypeIds"]; !ok {
		if _, ok := q["issuetypeNames"]; !ok {
			q["issuetypeIds"] = "{issueTypeId}"
		}
	}
	return q
}

func dryRunPathEscape(ref string) string {
	if strings.HasPrefix(ref, "{") && strings.HasSuffix(ref, "}") {
		return ref
	}
	return url.PathEscape(ref)
}

func getJiraMap(ctx *jira.Context, path string, q map[string]string) (map[string]interface{}, error) {
	resp, err := ctx.Client.Do(httpclient.Request{Method: "GET", Path: path, Query: q})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	d, _ := jira.ReadJSON(resp.Body)
	return d, nil
}

func isCreateMetaFallbackError(err error) bool {
	var httpErr *httpclient.HTTPError
	if errors.As(err, &httpErr) {
		if httpErr.Code == "auth_failed" || httpErr.Code == "permission_denied" {
			return false
		}
		if httpErr.Status == 404 || httpErr.Status == 405 || httpErr.Status == 501 {
			return true
		}
		if httpErr.Code == "not_found" {
			return true
		}
		return createMetaFallbackMessage(httpErr.Message)
	}
	return err != nil && err.Error() == "invalid_args"
}

func createMetaFallbackMessage(message string) bool {
	message = strings.ToLower(message)
	for _, needle := range []string{"issue does not exist", "does not exist", "null for uri", "createmeta"} {
		if strings.Contains(message, needle) {
			return true
		}
	}
	return false
}

func isIssueReadMissingError(err error) bool {
	var httpErr *httpclient.HTTPError
	if errors.As(err, &httpErr) {
		if httpErr.Code == "auth_failed" || httpErr.Code == "permission_denied" {
			return false
		}
		return httpErr.Status == 404 || httpErr.Code == "not_found" || issueReadMissingMessage(httpErr.Message)
	}
	return false
}

func issueReadMissingMessage(message string) bool {
	message = strings.ToLower(message)
	return strings.Contains(message, "issue does not exist") || strings.Contains(message, "does not exist")
}

func fromIssueReadError(issueKey string, err error) error {
	message := "from issue " + jira.IssueKey(issueKey) + " could not be read"
	var httpErr *httpclient.HTTPError
	if errors.As(err, &httpErr) {
		if strings.TrimSpace(httpErr.Message) != "" {
			message += ": " + httpErr.Message
		}
		return &httpclient.HTTPError{Code: httpErr.Code, Message: message, Hint: httpErr.Hint, Status: httpErr.Status}
	}
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		message += ": " + err.Error()
	}
	return fmt.Errorf("%s", message)
}

func splitCreateMetaFallbackWarning(err error) map[string]interface{} {
	warning := map[string]interface{}{
		"code":    "split_createmeta_fallback_error",
		"message": "Split createmeta failed; legacy createmeta was used.",
	}
	var httpErr *httpclient.HTTPError
	if errors.As(err, &httpErr) && strings.TrimSpace(httpErr.Message) != "" {
		warning["detail"] = httpErr.Message
	}
	return warning
}

func selectCreateMetaIssueType(resp map[string]interface{}, typeID, typeName string) map[string]interface{} {
	values := arrayValue(resp, "issueTypes")
	if len(values) == 0 {
		values = arrayValue(resp, "values")
	}
	var first map[string]interface{}
	for _, item := range values {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if first == nil {
			first = m
		}
		if typeID != "" && stringValue(m, "id") == typeID {
			return m
		}
		if typeName != "" && strings.EqualFold(stringValue(m, "name"), typeName) {
			return m
		}
	}
	if typeID == "" && typeName == "" {
		return first
	}
	return nil
}

func selectLegacyCreateMeta(resp map[string]interface{}, opts createMetaOptions) (map[string]interface{}, map[string]interface{}) {
	projects := arrayValue(resp, "projects")
	var firstProject map[string]interface{}
	var firstIssueType map[string]interface{}
	for _, item := range projects {
		project, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if firstProject == nil {
			firstProject = project
		}
		projectMatch := opts.ProjectID == "" && opts.ProjectKey == ""
		if opts.ProjectID != "" && stringValue(project, "id") == opts.ProjectID {
			projectMatch = true
		}
		if opts.ProjectKey != "" && strings.EqualFold(stringValue(project, "key"), opts.ProjectKey) {
			projectMatch = true
		}
		for _, issueItem := range arrayValue(project, "issuetypes") {
			issueType, ok := issueItem.(map[string]interface{})
			if !ok {
				continue
			}
			if firstIssueType == nil {
				firstIssueType = issueType
			}
			typeMatch := opts.TypeID == "" && opts.TypeName == ""
			if opts.TypeID != "" && stringValue(issueType, "id") == opts.TypeID {
				typeMatch = true
			}
			if opts.TypeName != "" && strings.EqualFold(stringValue(issueType, "name"), opts.TypeName) {
				typeMatch = true
			}
			if projectMatch && typeMatch {
				return project, issueType
			}
		}
	}
	if firstProject == nil {
		firstProject = map[string]interface{}{}
	}
	if firstIssueType == nil {
		firstIssueType = map[string]interface{}{}
	}
	return firstProject, firstIssueType
}

func mapValue(m map[string]interface{}, key string) map[string]interface{} {
	if m == nil {
		return map[string]interface{}{}
	}
	v, ok := m[key].(map[string]interface{})
	if !ok {
		return map[string]interface{}{}
	}
	return v
}

func arrayValue(m map[string]interface{}, key string) []interface{} {
	if m == nil {
		return nil
	}
	v, _ := m[key].([]interface{})
	return v
}

func stringValue(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v := m[key]
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func issueLinkCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "link"}
	c.AddCommand(&cobra.Command{Use: "list <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return issueEntityGet(o, cmd, args[0], "", map[string]string{"fields": "issuelinks"})
	}})
	c.AddCommand(&cobra.Command{Use: "create", RunE: func(cmd *cobra.Command, args []string) error {
		if mustS(cmd, "type") == "" || mustS(cmd, "from") == "" || mustS(cmd, "to") == "" {
			return invalidArgs(cmd, o, "--type, --from, and --to required", "Use jira issue link create --type Relates --from PROJ-123 --to PROJ-124 --json.")
		}
		b := map[string]any{"type": map[string]string{"name": mustS(cmd, "type")}, "inwardIssue": map[string]string{"key": mustS(cmd, "from")}, "outwardIssue": map[string]string{"key": mustS(cmd, "to")}}
		if comment := mustS(cmd, "comment"); comment != "" {
			b["comment"] = map[string]string{"body": comment}
		}
		return issueSubPost(o, cmd, "issueLink", b)
	}})
	c.Commands()[1].Flags().String("type", "", "")
	c.Commands()[1].Flags().String("from", "", "")
	c.Commands()[1].Flags().String("to", "", "")
	c.Commands()[1].Flags().String("comment", "", "")
	c.AddCommand(&cobra.Command{Use: "delete <link-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		return issueSubDelete(o, cmd, "issueLink/"+args[0])
	}})
	return c
}

func issueRemoteLinkCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "remote-link"}
	c.AddCommand(&cobra.Command{Use: "list <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return issueEntityGet(o, cmd, args[0], "/remotelink", nil)
	}})
	c.AddCommand(&cobra.Command{Use: "add <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if mustS(cmd, "url") == "" || mustS(cmd, "title") == "" {
			return invalidArgs(cmd, o, "--url and --title required", "Use jira issue remote-link add PROJ-123 --url https://example.test/spec --title Spec --json.")
		}
		b := map[string]any{"object": map[string]string{"url": mustS(cmd, "url"), "title": mustS(cmd, "title")}}
		return issueEntityWrite(o, cmd, "POST", args[0], "/remotelink", nil, b)
	}})
	c.Commands()[1].Flags().String("url", "", "")
	c.Commands()[1].Flags().String("title", "", "")
	c.AddCommand(&cobra.Command{Use: "delete <issue-or-url> <link-id>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		return issueEntityWrite(o, cmd, "DELETE", args[0], "/remotelink/"+args[1], nil, nil)
	}})
	return c
}
func issuePropertyCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "property"}
	c.AddCommand(&cobra.Command{Use: "list <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return issueEntityGet(o, cmd, args[0], "/properties", nil)
	}})
	c.AddCommand(&cobra.Command{Use: "get <issue-or-url> <key>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		return issueEntityGet(o, cmd, args[0], "/properties/"+args[1], nil)
	}})
	c.AddCommand(&cobra.Command{Use: "set <issue-or-url> <key>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		if (mustS(cmd, "value") == "") == (mustS(cmd, "value-file") == "") {
			return invalidArgs(cmd, o, "--value or --value-file required, but not both", "Use jira issue property set PROJ-123 review.state --value '{\"ok\":true}' --json.")
		}
		v, err := readJiraJSONValue(cmd)
		if err != nil {
			return invalidArgs(cmd, o, err.Error(), "")
		}
		return issueEntityWrite(o, cmd, "PUT", args[0], "/properties/"+args[1], nil, v)
	}})
	c.Commands()[2].Flags().String("value", "", "")
	c.Commands()[2].Flags().String("value-file", "", "")
	c.AddCommand(&cobra.Command{Use: "delete <issue-or-url> <key>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		return issueEntityWrite(o, cmd, "DELETE", args[0], "/properties/"+args[1], nil, nil)
	}})
	return c
}

func issueCommentCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "comment"}
	c.AddCommand(&cobra.Command{Use: "list <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return issueEntityGet(o, cmd, args[0], "/comment", nil)
	}})
	c.AddCommand(&cobra.Command{Use: "get <issue-or-url> <comment-id>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		return issueEntityGet(o, cmd, args[0], "/comment/"+args[1], nil)
	}})
	c.AddCommand(&cobra.Command{Use: "add <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		b, err := readJiraBody(cmd)
		if err != nil {
			return invalidArgs(cmd, o, err.Error(), "")
		}
		if b == "" {
			return print(cmd, o, output.Failure("invalid_args", "comment body required", "", 400))
		}
		return issueEntityWrite(o, cmd, "POST", args[0], "/comment", nil, map[string]string{"body": b})
	}})
	c.Commands()[2].Flags().String("body", "", "")
	c.Commands()[2].Flags().String("body-file", "", "")
	c.Commands()[2].Flags().Bool("body-stdin", false, "")
	c.AddCommand(&cobra.Command{Use: "update <issue-or-url> <comment-id>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		b, err := readJiraBody(cmd)
		if err != nil {
			return invalidArgs(cmd, o, err.Error(), "")
		}
		if b == "" {
			return print(cmd, o, output.Failure("invalid_args", "comment body required", "", 400))
		}
		return issueEntityWrite(o, cmd, "PUT", args[0], "/comment/"+args[1], nil, map[string]string{"body": b})
	}})
	c.Commands()[3].Flags().String("body", "", "")
	c.Commands()[3].Flags().String("body-file", "", "")
	c.Commands()[3].Flags().Bool("body-stdin", false, "")
	c.AddCommand(&cobra.Command{Use: "delete <issue-or-url> <comment-id>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		return issueEntityWrite(o, cmd, "DELETE", args[0], "/comment/"+args[1], nil, nil)
	}})
	return c
}
func issueAttachmentCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "attachment"}
	c.AddCommand(&cobra.Command{Use: "list <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return issueEntityGet(o, cmd, args[0], "", map[string]string{"fields": "attachment"})
	}})
	c.AddCommand(&cobra.Command{Use: "upload <issue-or-url> <file>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		return issueEntityMultipart(o, cmd, args[0], "/attachments", args[1])
	}})
	return c
}
func issueWorklogCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "worklog"}
	c.AddCommand(&cobra.Command{Use: "list <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return issueEntityGet(o, cmd, args[0], "/worklog", nil)
	}})
	c.AddCommand(&cobra.Command{Use: "get <issue-or-url> <worklog-id>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		return issueEntityGet(o, cmd, args[0], "/worklog/"+args[1], nil)
	}})
	c.AddCommand(&cobra.Command{Use: "add <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ts := mustS(cmd, "time-spent")
		if ts == "" {
			return print(cmd, o, output.Failure("invalid_args", "--time-spent required", "", 400))
		}
		b := map[string]string{"timeSpent": ts, "started": mustS(cmd, "started"), "comment": mustS(cmd, "comment")}
		return issueEntityWrite(o, cmd, "POST", args[0], "/worklog", nil, b)
	}})
	c.Commands()[2].Flags().String("time-spent", "", "")
	c.Commands()[2].Flags().String("started", "", "")
	c.Commands()[2].Flags().String("comment", "", "")
	c.AddCommand(&cobra.Command{Use: "update <issue-or-url> <worklog-id>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		ts := mustS(cmd, "time-spent")
		st := mustS(cmd, "started")
		cm := mustS(cmd, "comment")
		if ts == "" && cm == "" {
			return print(cmd, o, output.Failure("invalid_args", "--time-spent or --comment required", "", 400))
		}
		b := map[string]string{"timeSpent": ts, "started": st, "comment": cm}
		return issueEntityWrite(o, cmd, "PUT", args[0], "/worklog/"+args[1], nil, b)
	}})
	c.Commands()[3].Flags().String("time-spent", "", "")
	c.Commands()[3].Flags().String("started", "", "")
	c.Commands()[3].Flags().String("comment", "", "")
	c.AddCommand(&cobra.Command{Use: "delete <issue-or-url> <worklog-id>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		return issueEntityWrite(o, cmd, "DELETE", args[0], "/worklog/"+args[1], nil, nil)
	}})
	return c
}

func commentCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "comment"}
	c.AddCommand(&cobra.Command{Use: "list <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return issueEntityGet(o, cmd, args[0], "/comment", nil)
	}})
	c.AddCommand(&cobra.Command{Use: "add <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		b, err := readJiraBody(cmd)
		if err != nil {
			return invalidArgs(cmd, o, err.Error(), "")
		}
		if b == "" {
			return print(cmd, o, output.Failure("invalid_args", "comment body required", "", 400))
		}
		return issueEntityWrite(o, cmd, "POST", args[0], "/comment", nil, map[string]string{"body": b})
	}})
	c.Commands()[1].Flags().String("body", "", "")
	c.Commands()[1].Flags().String("body-file", "", "")
	c.Commands()[1].Flags().Bool("body-stdin", false, "")
	return c
}
func attachmentCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "attachment"}
	c.AddCommand(&cobra.Command{Use: "get <attachment-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return issueSubGet(o, cmd, "attachment/"+args[0]) }})
	create := &cobra.Command{Use: "create", Hidden: true, RunE: func(cmd *cobra.Command, args []string) error {
		project := mustS(cmd, "project")
		name := mustS(cmd, "name")
		if project == "" || name == "" {
			return print(cmd, o, output.Failure("invalid_args", "--project and --name required", "", 400))
		}
		b := map[string]any{"project": project, "name": name, "description": mustS(cmd, "description"), "leadUserName": mustS(cmd, "lead")}
		return issueSubPost(o, cmd, "component", b)
	}}
	create.Flags().String("project", "", "")
	create.Flags().String("name", "", "")
	create.Flags().String("description", "", "")
	create.Flags().String("lead", "", "")
	c.AddCommand(create)
	update := &cobra.Command{Use: "update <attachment-id>", Hidden: true, Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		name := mustS(cmd, "name")
		desc := mustS(cmd, "description")
		if name == "" && desc == "" {
			return print(cmd, o, output.Failure("invalid_args", "at least one field required", "", 400))
		}
		b := map[string]any{"name": name, "description": desc}
		return issueSubPut(o, cmd, "component/"+args[0], b)
	}}
	update.Flags().String("name", "", "")
	update.Flags().String("description", "", "")
	c.AddCommand(update)
	c.AddCommand(&cobra.Command{Use: "delete <attachment-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		return issueSubDelete(o, cmd, "attachment/"+args[0])
	}})
	download := &cobra.Command{Use: "download <attachment-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		meta := mustB(cmd, "metadata-only")
		if meta {
			return issueSubGet(o, cmd, "attachment/"+args[0])
		}
		out := mustS(cmd, "output")
		if out == "" {
			return print(cmd, o, output.Failure("invalid_args", "--output required", "", 400))
		}
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		ctx, err := jira.NewContext(cfg, o.Instance, "", false)
		if err != nil {
			return print(cmd, o, output.Failure(err.Error(), err.Error(), "", 400))
		}
		resp, err := ctx.Client.Do(httpclient.Request{Method: "GET", Path: "attachment/" + args[0]})
		if err != nil {
			return print(cmd, o, envelopeError(err, "server_error"))
		}
		defer resp.Body.Close()
		d, _ := jira.ReadJSON(resp.Body)
		u, _ := d["content"].(string)
		if u == "" {
			return print(cmd, o, output.Failure("not_found", "attachment content URL missing", "Verify the attachment id or request metadata-only output.", 404))
		}
		if !jiraURLBelongsToBase(u, ctx.Inst.BaseURL) {
			return print(cmd, o, output.Failure("instance_url_mismatch", "off-instance content URL", "", 400))
		}
		r2, err := ctx.Client.Do(httpclient.Request{Method: "GET", Path: u})
		if err != nil {
			return print(cmd, o, envelopeError(err, "server_error"))
		}
		defer r2.Body.Close()
		b, err := io.ReadAll(r2.Body)
		if err != nil {
			return print(cmd, o, output.Failure("server_error", "failed to read attachment response", "", 500))
		}
		if err := os.WriteFile(out, b, 0o600); err != nil {
			return print(cmd, o, output.Failure("invalid_args", "failed to write --output: "+err.Error(), "Choose a writable output path.", 400))
		}
		return print(cmd, o, output.Success(ctx.Instance, map[string]any{"output": out, "saved": out}))
	}}
	download.Flags().String("output", "", "")
	download.Flags().Bool("metadata-only", false, "")
	c.AddCommand(download)
	c.AddCommand(&cobra.Command{Use: "meta", RunE: func(cmd *cobra.Command, args []string) error { return issueSubGet(o, cmd, "attachment/meta") }})
	return c
}
func worklogCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "worklog"}
	c.AddCommand(&cobra.Command{Use: "list <issue-or-url>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return issueEntityGet(o, cmd, args[0], "/worklog", nil)
	}})
	return c
}
func agileCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "agile"}
	c.AddCommand(&cobra.Command{Use: "board-list", RunE: func(cmd *cobra.Command, args []string) error {
		q := map[string]string{}
		if p := mustS(cmd, "project"); p != "" {
			q["projectKeyOrId"] = p
		}
		return agileGetQuery(o, cmd, "board", q)
	}})
	c.Commands()[0].Flags().String("project", "", "")
	c.AddCommand(&cobra.Command{Use: "board-get <comment-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return agileGet(o, cmd, "board/"+args[0]) }})
	c.AddCommand(&cobra.Command{Use: "sprint-list <board-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		q := map[string]string{}
		if st := mustS(cmd, "state"); st != "" {
			q["state"] = st
		}
		return agileGetQuery(o, cmd, "board/"+args[0]+"/sprint", q)
	}})
	c.Commands()[2].Flags().String("state", "", "")
	c.AddCommand(&cobra.Command{Use: "sprint-get <comment-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return agileGet(o, cmd, "sprint/"+args[0]) }})
	c.AddCommand(&cobra.Command{Use: "sprint-issues <comment-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return agileGet(o, cmd, "sprint/"+args[0]+"/issue") }})
	c.AddCommand(&cobra.Command{Use: "backlog-issues <board-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return agileGet(o, cmd, "board/"+args[0]+"/backlog") }})
	return c
}

func metaCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "meta"}
	c.AddCommand(&cobra.Command{Use: "field-list", RunE: func(cmd *cobra.Command, args []string) error { return issueSubGetValue(o, cmd, "field", "fields") }})
	c.AddCommand(&cobra.Command{Use: "issue-type-list", RunE: func(cmd *cobra.Command, args []string) error { return issueSubGet(o, cmd, "issuetype") }})
	c.AddCommand(&cobra.Command{Use: "status-list", RunE: func(cmd *cobra.Command, args []string) error { return issueSubGet(o, cmd, "status") }})
	c.AddCommand(&cobra.Command{Use: "priority-list", RunE: func(cmd *cobra.Command, args []string) error { return issueSubGet(o, cmd, "priority") }})
	c.AddCommand(&cobra.Command{Use: "resolution-list", RunE: func(cmd *cobra.Command, args []string) error { return issueSubGet(o, cmd, "resolution") }})
	c.AddCommand(&cobra.Command{Use: "workflow-list", RunE: func(cmd *cobra.Command, args []string) error { return issueSubGet(o, cmd, "workflow") }})
	c.AddCommand(&cobra.Command{Use: "workflow-get <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return issueSubGetQuery(o, cmd, "workflow", map[string]string{"workflowName": args[0]})
	}})
	c.AddCommand(&cobra.Command{Use: "permissions-myself", RunE: func(cmd *cobra.Command, args []string) error {
		return issueSubGetQuery(o, cmd, "mypermissions", map[string]string{"projectKey": mustS(cmd, "project"), "issueKey": mustS(cmd, "issue")})
	}})
	c.Commands()[7].Flags().String("project", "", "")
	c.Commands()[7].Flags().String("issue", "", "")
	c.AddCommand(&cobra.Command{Use: "settings-get", RunE: func(cmd *cobra.Command, args []string) error { return issueSubGet(o, cmd, "application-properties") }})
	c.AddCommand(&cobra.Command{Use: "config-get", RunE: func(cmd *cobra.Command, args []string) error { return issueSubGet(o, cmd, "configuration") }})
	return c
}

func projectCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "project"}
	c.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error { return issueSubGet(o, cmd, "project") }})
	c.AddCommand(&cobra.Command{Use: "get <project-key>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return issueSubGet(o, cmd, "project/"+args[0]) }})
	c.AddCommand(&cobra.Command{Use: "statuses <project-key>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return issueSubGet(o, cmd, "project/"+args[0]+"/statuses")
	}})
	c.AddCommand(&cobra.Command{Use: "roles <project-key>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return issueSubGet(o, cmd, "project/"+args[0]+"/role") }})
	role := &cobra.Command{Use: "role"}
	role.AddCommand(&cobra.Command{Use: "get <project-key> <role-id-or-name>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		return issueSubGet(o, cmd, "project/"+args[0]+"/role/"+args[1])
	}})
	c.AddCommand(role)
	c.AddCommand(&cobra.Command{Use: "components <project-key>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return issueSubGet(o, cmd, "project/"+args[0]+"/components")
	}})
	c.AddCommand(&cobra.Command{Use: "versions <project-key>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return issueSubGet(o, cmd, "project/"+args[0]+"/versions")
	}})
	return c
}

func userGroupCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "user"}
	getCmd := &cobra.Command{Use: "get", RunE: func(cmd *cobra.Command, args []string) error {
		q := mustS(cmd, "username")
		if q == "" {
			q = mustS(cmd, "key")
		}
		return issueSubGetQuery(o, cmd, "user", map[string]string{"username": q})
	}}
	getCmd.Flags().String("username", "", "")
	getCmd.Flags().String("key", "", "")
	c.AddCommand(getCmd)
	searchCmd := &cobra.Command{Use: "search", RunE: func(cmd *cobra.Command, args []string) error {
		return issueSubGetQuery(o, cmd, "user/search", map[string]string{"username": mustS(cmd, "query"), "maxResults": mustS(cmd, "limit")})
	}}
	searchCmd.Flags().String("query", "", "")
	searchCmd.Flags().String("limit", "50", "")
	c.AddCommand(searchCmd)
	assignCmd := &cobra.Command{Use: "assignable", RunE: func(cmd *cobra.Command, args []string) error {
		return issueSubGetQuery(o, cmd, "user/assignable/search", map[string]string{"project": mustS(cmd, "project"), "issue": mustS(cmd, "issue"), "username": mustS(cmd, "query")})
	}}
	assignCmd.Flags().String("project", "", "")
	assignCmd.Flags().String("issue", "", "")
	assignCmd.Flags().String("query", "", "")
	c.AddCommand(assignCmd)
	g := &cobra.Command{Use: "group"}
	g.AddCommand(&cobra.Command{Use: "get <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return issueSubGetQuery(o, cmd, "group", map[string]string{"groupname": args[0]})
	}})
	g.AddCommand(&cobra.Command{Use: "members <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return issueSubGetQuery(o, cmd, "group/member", map[string]string{"groupname": args[0]})
	}})
	gsearch := &cobra.Command{Use: "search", RunE: func(cmd *cobra.Command, args []string) error {
		return issueSubGetQuery(o, cmd, "groups/picker", map[string]string{"query": mustS(cmd, "query")})
	}}
	gsearch.Flags().String("query", "", "")
	g.AddCommand(gsearch)
	g.Hidden = true
	c.AddCommand(g)
	return c
}

func groupCmd(o *Opts) *cobra.Command {
	g := &cobra.Command{Use: "group"}
	g.AddCommand(&cobra.Command{Use: "get <group-name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return issueSubGetQuery(o, cmd, "group", map[string]string{"groupname": args[0]})
	}})
	g.AddCommand(&cobra.Command{Use: "members <group-name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return issueSubGetQuery(o, cmd, "group/member", map[string]string{"groupname": args[0]})
	}})
	search := &cobra.Command{Use: "search", RunE: func(cmd *cobra.Command, args []string) error {
		return issueSubGetQuery(o, cmd, "groups/picker", map[string]string{"query": mustS(cmd, "query")})
	}}
	search.Flags().String("query", "", "")
	g.AddCommand(search)
	return g
}

func filterDashboardCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "filter"}
	c.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		if mustB(cmd, "favorite") {
			return issueSubGet(o, cmd, "filter/favourite")
		}
		return issueSubGet(o, cmd, "filter/search")
	}})
	c.Commands()[0].Flags().Bool("favorite", false, "")
	c.AddCommand(&cobra.Command{Use: "get <filter-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return issueSubGet(o, cmd, "filter/"+args[0]) }})
	c.AddCommand(&cobra.Command{Use: "search", RunE: func(cmd *cobra.Command, args []string) error {
		return issueSubGetQuery(o, cmd, "filter/search", map[string]string{"filterName": mustS(cmd, "query")})
	}})
	c.Commands()[2].Flags().String("query", "", "")
	c.AddCommand(&cobra.Command{Use: "create", RunE: func(cmd *cobra.Command, args []string) error {
		name := mustS(cmd, "name")
		jql := mustS(cmd, "jql")
		if name == "" || jql == "" {
			return print(cmd, o, output.Failure("invalid_args", "--name and --jql required", "", 400))
		}
		b := map[string]any{"name": name, "jql": jql, "description": mustS(cmd, "description"), "favourite": mustB(cmd, "favorite")}
		return issueSubPost(o, cmd, "filter", b)
	}})
	c.Commands()[3].Flags().String("name", "", "")
	c.Commands()[3].Flags().String("jql", "", "")
	c.Commands()[3].Flags().String("description", "", "")
	c.Commands()[3].Flags().Bool("favorite", false, "")
	c.AddCommand(&cobra.Command{Use: "update <filter-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		name := mustS(cmd, "name")
		jql := mustS(cmd, "jql")
		if name == "" && jql == "" && mustS(cmd, "description") == "" {
			return print(cmd, o, output.Failure("invalid_args", "at least one field required", "", 400))
		}
		b := map[string]any{"name": name, "jql": jql, "description": mustS(cmd, "description"), "favourite": mustB(cmd, "favorite")}
		return issueSubPut(o, cmd, "filter/"+args[0], b)
	}})
	c.Commands()[4].Flags().String("name", "", "")
	c.Commands()[4].Flags().String("jql", "", "")
	c.Commands()[4].Flags().String("description", "", "")
	c.Commands()[4].Flags().Bool("favorite", false, "")
	c.AddCommand(&cobra.Command{Use: "delete <filter-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		return issueSubDelete(o, cmd, "filter/"+args[0])
	}})
	d := &cobra.Command{Use: "dashboard"}
	d.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error { return issueSubGet(o, cmd, "dashboard") }})
	d.AddCommand(&cobra.Command{Use: "get <dashboard-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return issueSubGet(o, cmd, "dashboard/"+args[0]) }})
	d.AddCommand(&cobra.Command{Use: "search", RunE: func(cmd *cobra.Command, args []string) error {
		return issueSubGetQuery(o, cmd, "dashboard", map[string]string{"query": mustS(cmd, "query")})
	}})
	d.Commands()[2].Flags().String("query", "", "")
	d.Hidden = true
	c.AddCommand(d)
	return c
}

func dashboardCmd(o *Opts) *cobra.Command {
	d := &cobra.Command{Use: "dashboard"}
	d.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error { return issueSubGet(o, cmd, "dashboard") }})
	d.AddCommand(&cobra.Command{Use: "get <dashboard-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return issueSubGet(o, cmd, "dashboard/"+args[0]) }})
	return d
}

func componentCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "component"}
	c.AddCommand(&cobra.Command{Use: "get <component-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return issueSubGet(o, cmd, "component/"+args[0]) }})
	c.AddCommand(&cobra.Command{Use: "create", RunE: func(cmd *cobra.Command, args []string) error {
		project := mustS(cmd, "project")
		name := mustS(cmd, "name")
		if project == "" || name == "" {
			return print(cmd, o, output.Failure("invalid_args", "--project and --name required", "", 400))
		}
		b := map[string]any{"project": project, "name": name, "description": mustS(cmd, "description"), "leadUserName": mustS(cmd, "lead")}
		return issueSubPost(o, cmd, "component", b)
	}})
	c.Commands()[1].Flags().String("project", "", "")
	c.Commands()[1].Flags().String("name", "", "")
	c.Commands()[1].Flags().String("description", "", "")
	c.Commands()[1].Flags().String("lead", "", "")
	c.AddCommand(&cobra.Command{Use: "update <component-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		name := mustS(cmd, "name")
		desc := mustS(cmd, "description")
		if name == "" && desc == "" {
			return print(cmd, o, output.Failure("invalid_args", "at least one field required", "", 400))
		}
		b := map[string]any{"name": name, "description": desc}
		return issueSubPut(o, cmd, "component/"+args[0], b)
	}})
	c.Commands()[2].Flags().String("name", "", "")
	c.Commands()[2].Flags().String("description", "", "")
	c.AddCommand(&cobra.Command{Use: "delete <component-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		return issueSubDelete(o, cmd, "component/"+args[0])
	}})
	return c
}

func versionCmd(o *Opts) *cobra.Command {
	v := cliVersionCmd(o)
	v.AddCommand(&cobra.Command{Use: "get <version-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return issueSubGet(o, cmd, "version/"+args[0]) }})
	v.AddCommand(&cobra.Command{Use: "create", RunE: func(cmd *cobra.Command, args []string) error {
		project := mustS(cmd, "project")
		name := mustS(cmd, "name")
		if project == "" || name == "" {
			return print(cmd, o, output.Failure("invalid_args", "--project and --name required", "", 400))
		}
		b := map[string]any{"project": project, "name": name, "description": mustS(cmd, "description"), "releaseDate": mustS(cmd, "release-date"), "released": mustB(cmd, "released")}
		return issueSubPost(o, cmd, "version", b)
	}})
	v.Commands()[1].Flags().String("project", "", "")
	v.Commands()[1].Flags().String("name", "", "")
	v.Commands()[1].Flags().String("description", "", "")
	v.Commands()[1].Flags().String("release-date", "", "")
	v.Commands()[1].Flags().Bool("released", false, "")
	v.AddCommand(&cobra.Command{Use: "update <version-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		name := mustS(cmd, "name")
		desc := mustS(cmd, "description")
		if name == "" && desc == "" {
			return print(cmd, o, output.Failure("invalid_args", "at least one field required", "", 400))
		}
		b := map[string]any{"name": name, "description": desc}
		return issueSubPut(o, cmd, "version/"+args[0], b)
	}})
	v.Commands()[2].Flags().String("name", "", "")
	v.Commands()[2].Flags().String("description", "", "")
	v.AddCommand(&cobra.Command{Use: "delete <version-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		return issueSubDelete(o, cmd, "version/"+args[0])
	}})
	return v
}

func metadataCmds(o *Opts) []*cobra.Command {
	field := &cobra.Command{Use: "field"}
	field.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error { return issueSubGetValue(o, cmd, "field", "fields") }})
	issueType := &cobra.Command{Use: "issue-type"}
	issueType.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error { return issueSubGet(o, cmd, "issuetype") }})
	status := &cobra.Command{Use: "status"}
	status.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error { return issueSubGet(o, cmd, "status") }})
	priority := &cobra.Command{Use: "priority"}
	priority.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error { return issueSubGet(o, cmd, "priority") }})
	resolution := &cobra.Command{Use: "resolution"}
	resolution.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error { return issueSubGet(o, cmd, "resolution") }})
	workflow := &cobra.Command{Use: "workflow"}
	workflow.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error { return issueSubGet(o, cmd, "workflow") }})
	workflow.AddCommand(&cobra.Command{Use: "get <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return issueSubGetQuery(o, cmd, "workflow", map[string]string{"workflowName": args[0]})
	}})
	permissions := &cobra.Command{Use: "permissions"}
	permissions.AddCommand(&cobra.Command{Use: "myself", RunE: func(cmd *cobra.Command, args []string) error {
		return issueSubGetQuery(o, cmd, "mypermissions", map[string]string{"projectKey": mustS(cmd, "project"), "issueKey": mustS(cmd, "issue")})
	}})
	permissions.Commands()[0].Flags().String("project", "", "")
	permissions.Commands()[0].Flags().String("issue", "", "")
	settings := &cobra.Command{Use: "settings"}
	settings.AddCommand(&cobra.Command{Use: "get", RunE: func(cmd *cobra.Command, args []string) error { return issueSubGet(o, cmd, "application-properties") }})
	configCmd := &cobra.Command{Use: "config"}
	configCmd.AddCommand(&cobra.Command{Use: "get", RunE: func(cmd *cobra.Command, args []string) error { return issueSubGet(o, cmd, "configuration") }})
	return []*cobra.Command{field, issueType, status, priority, resolution, workflow, permissions, settings, configCmd}
}

func boardCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "board"}
	c.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		q := map[string]string{}
		if p := mustS(cmd, "project"); p != "" {
			q["projectKeyOrId"] = p
		}
		return agileGetQuery(o, cmd, "board", q)
	}})
	c.Commands()[0].Flags().String("project", "", "")
	c.AddCommand(&cobra.Command{Use: "get <board-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return agileGet(o, cmd, "board/"+args[0]) }})
	return c
}

func sprintCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "sprint"}
	c.AddCommand(&cobra.Command{Use: "list <board-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		q := map[string]string{}
		if st := mustS(cmd, "state"); st != "" {
			q["state"] = st
		}
		return agileGetQuery(o, cmd, "board/"+args[0]+"/sprint", q)
	}})
	c.Commands()[0].Flags().String("state", "", "")
	c.AddCommand(&cobra.Command{Use: "get <sprint-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return agileGet(o, cmd, "sprint/"+args[0]) }})
	c.AddCommand(&cobra.Command{Use: "issues <sprint-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return agileGet(o, cmd, "sprint/"+args[0]+"/issue") }})
	return c
}

func backlogCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "backlog"}
	c.AddCommand(&cobra.Command{Use: "issues <board-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return agileGet(o, cmd, "board/"+args[0]+"/backlog") }})
	return c
}

func issueCtx(o *Opts, cmd *cobra.Command, issueOrURL string) (*jira.Context, string, error) {
	cfg, err := loadCfg(o)
	if err != nil {
		return nil, "", err
	}
	ctx, err := jira.NewContext(cfg, o.Instance, issueOrURL, o.DryRun)
	if err != nil {
		return nil, "", err
	}
	return ctx, jira.IssueKey(issueOrURL), nil
}

func issueCtxError(cmd *cobra.Command, o *Opts, err error) error {
	code := err.Error()
	if code != "config_missing" && code != "no_instance_configured" && code != "instance_required" && code != "ambiguous_instance" && code != "instance_url_mismatch" {
		code = "config_missing"
	}
	status := 400
	if code == "config_missing" {
		status = 404
	}
	return print(cmd, o, output.Failure(code, err.Error(), "", status))
}

func jiraURLBelongsToBase(raw, base string) bool {
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

func issueEntityGet(o *Opts, cmd *cobra.Command, issueOrURL, suffix string, q map[string]string) error {
	ctx, issue, err := issueCtx(o, cmd, issueOrURL)
	if err != nil {
		return issueCtxError(cmd, o, err)
	}
	path := "issue/" + issue + suffix
	if o.DryRun {
		return print(cmd, o, output.Success(ctx.Instance, jira.DryRunData("GET", path, q, nil)))
	}
	resp, err := ctx.Client.Do(httpclient.Request{Method: "GET", Path: path, Query: q})
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	defer resp.Body.Close()
	d, _ := jira.ReadJSON(resp.Body)
	return print(cmd, o, output.Success(ctx.Instance, d))
}

func issueEntityWrite(o *Opts, cmd *cobra.Command, method, issueOrURL, suffix string, q map[string]string, body interface{}) error {
	ctx, issue, err := issueCtx(o, cmd, issueOrURL)
	if err != nil {
		return issueCtxError(cmd, o, err)
	}
	path := "issue/" + issue + suffix
	if o.DryRun {
		return print(cmd, o, output.Success(ctx.Instance, jira.DryRunData(method, path, q, body)))
	}
	req := httpclient.Request{Method: method, Path: path, Query: q, JSONBody: body}
	resp, err := ctx.Client.Do(req)
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	if resp != nil {
		defer resp.Body.Close()
	}
	switch method {
	case "DELETE", "PUT":
		return print(cmd, o, output.Success(ctx.Instance, map[string]any{"ok": true}))
	default:
		d, _ := jira.ReadJSON(resp.Body)
		return print(cmd, o, output.Success(ctx.Instance, d))
	}
}

func issueEntityMultipart(o *Opts, cmd *cobra.Command, issueOrURL, suffix, file string) error {
	ctx, issue, err := issueCtx(o, cmd, issueOrURL)
	if err != nil {
		return issueCtxError(cmd, o, err)
	}
	path := "issue/" + issue + suffix
	if o.DryRun {
		return print(cmd, o, output.Success(ctx.Instance, jira.DryRunData("POST", path, nil, map[string]any{"file": file})))
	}
	f, err := os.Open(file)
	if err != nil {
		return print(cmd, o, output.Failure("invalid_args", err.Error(), "", 400))
	}
	defer f.Close()
	_, err = ctx.Client.Do(httpclient.Request{Method: "POST", Path: path, MultipartField: "file", MultipartName: file, Multipart: f, Headers: map[string]string{"X-Atlassian-Token": "no-check"}})
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	return print(cmd, o, output.Success(ctx.Instance, map[string]any{"uploaded": true}))
}

func issueSubGetQuery(o *Opts, cmd *cobra.Command, path string, q map[string]string) error {
	cfg, err := loadCfg(o)
	if err != nil {
		return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
	}
	ctx, err := jira.NewContext(cfg, o.Instance, "", o.DryRun)
	if err != nil {
		return print(cmd, o, output.Failure(err.Error(), err.Error(), "", 400))
	}
	if o.DryRun {
		return print(cmd, o, output.Success(ctx.Instance, jira.DryRunData("GET", path, q, nil)))
	}
	resp, err := ctx.Client.Do(httpclient.Request{Method: "GET", Path: path, Query: q})
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	defer resp.Body.Close()
	d, _ := jira.ReadJSON(resp.Body)
	return print(cmd, o, output.Success(ctx.Instance, d))
}

func issueSubGet(o *Opts, cmd *cobra.Command, path string) error {
	cfg, err := loadCfg(o)
	if err != nil {
		return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
	}
	ctx, err := jira.NewContext(cfg, o.Instance, "", o.DryRun)
	if err != nil {
		return print(cmd, o, output.Failure(err.Error(), err.Error(), "", 400))
	}
	if o.DryRun {
		return print(cmd, o, output.Success(ctx.Instance, jira.DryRunData("GET", path, nil, nil)))
	}
	resp, err := ctx.Client.Do(httpclient.Request{Method: "GET", Path: path})
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	defer resp.Body.Close()
	d, _ := jira.ReadJSON(resp.Body)
	return print(cmd, o, output.Success(ctx.Instance, d))
}

func issueSubGetValue(o *Opts, cmd *cobra.Command, path, wrapper string) error {
	cfg, err := loadCfg(o)
	if err != nil {
		return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
	}
	ctx, err := jira.NewContext(cfg, o.Instance, "", o.DryRun)
	if err != nil {
		return print(cmd, o, output.Failure(err.Error(), err.Error(), "", 400))
	}
	if o.DryRun {
		return print(cmd, o, output.Success(ctx.Instance, jira.DryRunData("GET", path, nil, nil)))
	}
	resp, err := ctx.Client.Do(httpclient.Request{Method: "GET", Path: path})
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	defer resp.Body.Close()
	d, _ := jira.ReadJSONValue(resp.Body)
	if wrapper != "" {
		d = map[string]interface{}{wrapper: d}
	}
	return print(cmd, o, output.Success(ctx.Instance, d))
}

func issueSubPost(o *Opts, cmd *cobra.Command, path string, body interface{}) error {
	cfg, err := loadCfg(o)
	if err != nil {
		return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
	}
	ctx, err := jira.NewContext(cfg, o.Instance, "", o.DryRun)
	if err != nil {
		return print(cmd, o, output.Failure(err.Error(), err.Error(), "", 400))
	}
	if o.DryRun {
		return print(cmd, o, output.Success(ctx.Instance, jira.DryRunData("POST", path, nil, body)))
	}
	resp, err := ctx.Client.Do(httpclient.Request{Method: "POST", Path: path, JSONBody: body})
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	defer resp.Body.Close()
	d, _ := jira.ReadJSON(resp.Body)
	return print(cmd, o, output.Success(ctx.Instance, d))
}

func issueSubPut(o *Opts, cmd *cobra.Command, path string, body interface{}) error {
	cfg, err := loadCfg(o)
	if err != nil {
		return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
	}
	ctx, err := jira.NewContext(cfg, o.Instance, "", o.DryRun)
	if err != nil {
		return print(cmd, o, output.Failure(err.Error(), err.Error(), "", 400))
	}
	if o.DryRun {
		return print(cmd, o, output.Success(ctx.Instance, jira.DryRunData("PUT", path, nil, body)))
	}
	_, err = ctx.Client.Do(httpclient.Request{Method: "PUT", Path: path, JSONBody: body})
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	return print(cmd, o, output.Success(ctx.Instance, map[string]any{"updated": true}))
}
func issueSubDeleteQuery(o *Opts, cmd *cobra.Command, path string, q map[string]string) error {
	cfg, err := loadCfg(o)
	if err != nil {
		return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
	}
	ctx, err := jira.NewContext(cfg, o.Instance, "", o.DryRun)
	if err != nil {
		return print(cmd, o, output.Failure(err.Error(), err.Error(), "", 400))
	}
	if o.DryRun {
		return print(cmd, o, output.Success(ctx.Instance, jira.DryRunData("DELETE", path, q, nil)))
	}
	_, err = ctx.Client.Do(httpclient.Request{Method: "DELETE", Path: path, Query: q})
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	return print(cmd, o, output.Success(ctx.Instance, map[string]any{"deleted": true}))
}
func issueSubMultipart(o *Opts, cmd *cobra.Command, path, file string) error {
	cfg, err := loadCfg(o)
	if err != nil {
		return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
	}
	ctx, err := jira.NewContext(cfg, o.Instance, "", o.DryRun)
	if err != nil {
		return print(cmd, o, output.Failure(err.Error(), err.Error(), "", 400))
	}
	if o.DryRun {
		return print(cmd, o, output.Success(ctx.Instance, jira.DryRunData("POST", path, nil, map[string]any{"file": file})))
	}
	f, err := os.Open(file)
	if err != nil {
		return print(cmd, o, output.Failure("invalid_args", err.Error(), "", 400))
	}
	defer f.Close()
	_, err = ctx.Client.Do(httpclient.Request{Method: "POST", Path: path, MultipartField: "file", MultipartName: file, Multipart: f})
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	return print(cmd, o, output.Success(ctx.Instance, map[string]any{"uploaded": true}))
}

func issueSubDelete(o *Opts, cmd *cobra.Command, path string) error {
	cfg, err := loadCfg(o)
	if err != nil {
		return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
	}
	ctx, err := jira.NewContext(cfg, o.Instance, "", o.DryRun)
	if err != nil {
		return print(cmd, o, output.Failure(err.Error(), err.Error(), "", 400))
	}
	if o.DryRun {
		return print(cmd, o, output.Success(ctx.Instance, jira.DryRunData("DELETE", path, nil, nil)))
	}
	_, err = ctx.Client.Do(httpclient.Request{Method: "DELETE", Path: path})
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	return print(cmd, o, output.Success(ctx.Instance, map[string]any{"deleted": true}))
}
func agileGetQuery(o *Opts, cmd *cobra.Command, path string, q map[string]string) error {
	cfg, err := loadCfg(o)
	if err != nil {
		return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
	}
	ctx, err := jira.NewContext(cfg, o.Instance, "", o.DryRun)
	if err != nil {
		return print(cmd, o, output.Failure(err.Error(), err.Error(), "", 400))
	}
	reqPath := "/rest/agile/1.0/" + strings.TrimLeft(path, "/")
	if o.DryRun {
		return print(cmd, o, output.Success(ctx.Instance, jira.DryRunData("GET", reqPath, q, nil)))
	}
	resp, err := ctx.Client.Do(httpclient.Request{Method: "GET", Path: reqPath, Query: q})
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	defer resp.Body.Close()
	d, _ := jira.ReadJSON(resp.Body)
	return print(cmd, o, output.Success(ctx.Instance, d))
}
func agileGet(o *Opts, cmd *cobra.Command, path string) error {
	cfg, err := loadCfg(o)
	if err != nil {
		return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
	}
	ctx, err := jira.NewContext(cfg, o.Instance, "", o.DryRun)
	if err != nil {
		return print(cmd, o, output.Failure(err.Error(), err.Error(), "", 400))
	}
	reqPath := "/rest/agile/1.0/" + strings.TrimLeft(path, "/")
	if o.DryRun {
		return print(cmd, o, output.Success(ctx.Instance, jira.DryRunData("GET", reqPath, nil, nil)))
	}
	resp, err := ctx.Client.Do(httpclient.Request{Method: "GET", Path: reqPath})
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	defer resp.Body.Close()
	d, _ := jira.ReadJSON(resp.Body)
	return print(cmd, o, output.Success(ctx.Instance, d))
}

func ParseJSON(b []byte) map[string]interface{} {
	out := map[string]interface{}{}
	_ = json.NewDecoder(bytes.NewReader(b)).Decode(&out)
	return out
}
