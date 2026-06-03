package commands

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/catalog"
	"engineering-flow-platform-tools/internal/clihelp"
	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/files"
	"engineering-flow-platform-tools/internal/httpclient"
	"engineering-flow-platform-tools/internal/jenkins"
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
	client *jenkins.Client
}

func NewRoot() *cobra.Command {
	cobra.EnableCommandSorting = false
	o := &Opts{Format: "table"}
	c := &cobra.Command{Use: "jenkins", SilenceErrors: true, SilenceUsage: true}
	c.PersistentFlags().StringVar(&o.Instance, "instance", "", "Configured Jenkins instance name.")
	c.PersistentFlags().StringVar(&o.Config, "config", "", "Path to EFP config file.")
	c.PersistentFlags().BoolVar(&o.JSON, "json", false, "Print JSON envelope.")
	c.PersistentFlags().StringVar(&o.Format, "format", "table", "Output format: table|json|yaml.")
	c.PersistentFlags().BoolVar(&o.Verbose, "verbose", false, "Print non-secret diagnostics when available.")
	c.PersistentFlags().BoolVar(&o.DryRun, "dry-run", false, "Preview write request without sending it.")
	c.PersistentFlags().BoolVar(&o.Yes, "yes", false, "Confirm destructive or service-affecting operations.")
	c.AddCommand(instanceCmd(o), authCmd(o), whoamiCmd(o), serverInfoCmd(o), crumbCmd(o), jobCmd(o), queueCmd(o), buildCmd(o), artifactCmd(o), pipelineCmd(o), viewCmd(o), nodeCmd(o), pluginCmd(o), systemCmd(o), rawAPICmd(o), commandsCmd(), schemaCmd(), helpLLMCmd(), versionCmd(o))
	clihelp.ApplyCatalogHelp(c, clihelp.ProductHelp{
		Product: "jenkins",
		Binary:  "jenkins",
		Short:   "Operate Jenkins jobs, builds, queues, logs, artifacts, Pipeline data, and controller metadata",
		Long: strings.TrimSpace(`jenkins is a terminal-invoked CLI for agents and scripts that need stable JSON access to Jenkins controller resources.

Use it for jobs, builds, queues, console logs, artifacts, Pipeline REST data, views, nodes, plugins, selected controller actions, and raw Jenkins API calls. For agent workflows, default every command and subcommand to --json. Use --dry-run before write operations and --yes only after explicit user confirmation for destructive or service-affecting operations.

Configuration uses the shared EFP config file, normally ~/.efp/config.yaml, under the jenkins node.`),
		Examples: []string{
			`jenkins job build app/main --json`,
			`jenkins build status app/main lastBuild --json`,
			`jenkins build log app/main 42 --json`,
			`jenkins artifact download app/main 42 target/app.jar --output app.jar --json`,
			`jenkins schema job.build-with-params --json`,
			`jenkins help llm --json`,
		},
		Instructions: "copy cmd/jenkins/jenkins-cli.instructions.md to ~/.copilot/instructions/jenkins-cli.instructions.md.",
		Groups: map[string]string{
			"instance": "Manage configured Jenkins instances.",
			"auth":     "Manage Jenkins credentials stored in the EFP config.",
			"job":      "Inspect and manage Jenkins jobs by folder path.",
			"queue":    "Inspect and manage Jenkins queue items.",
			"build":    "Inspect builds, status, logs, and stop builds.",
			"artifact": "Download Jenkins build artifacts.",
			"pipeline": "Inspect Pipeline REST API runs, stages, node logs, and artifacts.",
			"view":     "Inspect and manage Jenkins views.",
			"node":     "Inspect Jenkins controller and agent nodes.",
			"plugin":   "Inspect installed Jenkins plugins.",
			"system":   "Run selected Jenkins controller actions.",
			"api":      "Call raw Jenkins API paths on the selected instance.",
		},
	})
	return c
}

func fmtOut(o *Opts) string {
	if o.JSON {
		return "json"
	}
	if o.Format != "" {
		return strings.ToLower(o.Format)
	}
	return "table"
}

func print(cmd *cobra.Command, o *Opts, e output.Envelope) error {
	return output.Print(cmd.OutOrStdout(), fmtOut(o), e)
}

func loadCfg(o *Opts) (config.RootConfig, error) {
	p, _ := config.ResolvePath(o.Config)
	return config.Load(p)
}

func saveCfg(o *Opts, cfg config.RootConfig) error {
	p, _ := config.ResolvePath(o.Config)
	return config.Save(p, cfg)
}

func loadCtx(o *Opts, entity string) (*ctx, error) {
	cfg, err := loadCfg(o)
	if err != nil {
		return nil, err
	}
	jcx, err := jenkins.NewContext(cfg, o.Instance, entity, o.DryRun)
	if err != nil {
		return nil, err
	}
	return &ctx{cfg: cfg, inst: jcx.Inst, client: jcx.Client}, nil
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
	default:
		return auth, fmt.Errorf("invalid_args")
	}
	return auth, nil
}

func addAuthFlags(cmd *cobra.Command) {
	cmd.Flags().String("username", "", "Username for basic authentication.")
	cmd.Flags().String("auth-type", "", "Authentication type: basic_password, basic_api_key, bearer_token, or alias.")
	cmd.Flags().Bool("password-stdin", false, "Read password from stdin.")
	cmd.Flags().Bool("api-key-stdin", false, "Read Jenkins API token from stdin.")
	cmd.Flags().Bool("token-stdin", false, "Read bearer token or PAT from stdin.")
}

func instanceCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "instance"}
	c.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		for i := range cfg.Jenkins.Instances {
			cfg.Jenkins.Instances[i].Auth = config.RedactAuth(cfg.Jenkins.Instances[i].Auth)
		}
		return print(cmd, o, output.Success("", map[string]any{"instances": cfg.Jenkins.Instances, "default_instance": cfg.Jenkins.DefaultInstance}))
	}})
	c.AddCommand(&cobra.Command{Use: "get <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		for _, in := range cfg.Jenkins.Instances {
			if in.Name == args[0] {
				in.Auth = config.RedactAuth(in.Auth)
				return print(cmd, o, output.Success(in.Name, in))
			}
		}
		return print(cmd, o, output.Failure("not_found", "instance not found", "", 404))
	}})
	add := &cobra.Command{Use: "add <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := loadCfg(o)
		baseURL := mustS(cmd, "base-url")
		if baseURL == "" {
			return print(cmd, o, output.Failure("invalid_args", "--base-url is required", "Pass the Jenkins controller base URL.", 400))
		}
		auth, authErr := authFromFlags(cmd)
		if authErr != nil {
			return print(cmd, o, output.Failure("invalid_args", "missing required auth secret", "Use --username with --api-key-stdin, --password-stdin, or --token-stdin.", 400))
		}
		in := config.InstanceConfig{Name: args[0], BaseURL: baseURL, RESTPath: mustS(cmd, "rest-path"), Auth: auth, CrumbMode: normalizedCrumbMode(mustS(cmd, "crumb-mode"))}
		cfg.Jenkins.Instances = append(cfg.Jenkins.Instances, in)
		if mustB(cmd, "default") {
			cfg.Jenkins.DefaultInstance = args[0]
		}
		if err := saveCfg(o, cfg); err != nil {
			return print(cmd, o, output.Failure("config_error", err.Error(), "", 500))
		}
		return print(cmd, o, output.Success(args[0], map[string]any{"added": true}))
	}}
	add.Flags().String("base-url", "", "Jenkins controller base URL.")
	add.Flags().String("rest-path", "", "Reserved Jenkins REST path override; normally empty.")
	add.Flags().String("crumb-mode", "auto", "Jenkins crumb behavior: auto, always, or never.")
	addAuthFlags(add)
	add.Flags().Bool("default", false, "Make the added Jenkins instance the default instance.")
	c.AddCommand(add)
	update := &cobra.Command{Use: "update <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		for i := range cfg.Jenkins.Instances {
			if cfg.Jenkins.Instances[i].Name != args[0] {
				continue
			}
			if v := mustS(cmd, "base-url"); v != "" {
				cfg.Jenkins.Instances[i].BaseURL = v
			}
			if v := mustS(cmd, "rest-path"); v != "" {
				cfg.Jenkins.Instances[i].RESTPath = v
			}
			if v := mustS(cmd, "crumb-mode"); v != "" {
				cfg.Jenkins.Instances[i].CrumbMode = normalizedCrumbMode(v)
			}
			if err := saveCfg(o, cfg); err != nil {
				return print(cmd, o, output.Failure("config_error", err.Error(), "", 500))
			}
			return print(cmd, o, output.Success(args[0], map[string]any{"updated": true}))
		}
		return print(cmd, o, output.Failure("not_found", "instance not found", "", 404))
	}}
	update.Flags().String("base-url", "", "New Jenkins controller base URL.")
	update.Flags().String("rest-path", "", "Reserved Jenkins REST path override; normally empty.")
	update.Flags().String("crumb-mode", "", "Jenkins crumb behavior: auto, always, or never.")
	c.AddCommand(update)
	c.AddCommand(&cobra.Command{Use: "remove <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "Pass --yes after confirming the instance removal.", 400))
		}
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		out := []config.InstanceConfig{}
		for _, in := range cfg.Jenkins.Instances {
			if in.Name != args[0] {
				out = append(out, in)
			}
		}
		cfg.Jenkins.Instances = out
		if cfg.Jenkins.DefaultInstance == args[0] {
			cfg.Jenkins.DefaultInstance = ""
		}
		if err := saveCfg(o, cfg); err != nil {
			return print(cmd, o, output.Failure("config_error", err.Error(), "", 500))
		}
		return print(cmd, o, output.Success(args[0], map[string]any{"removed": true}))
	}})
	c.AddCommand(&cobra.Command{Use: "default [name]", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		if len(args) == 0 {
			return print(cmd, o, output.Success("", map[string]any{"default_instance": cfg.Jenkins.DefaultInstance}))
		}
		cfg.Jenkins.DefaultInstance = args[0]
		if err := saveCfg(o, cfg); err != nil {
			return print(cmd, o, output.Failure("config_error", err.Error(), "", 500))
		}
		return print(cmd, o, output.Success(args[0], map[string]any{"default_instance": args[0]}))
	}})
	return c
}

func normalizedCrumbMode(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "", "auto":
		return "auto"
	case "always", "never":
		return v
	default:
		return "auto"
	}
}

func authCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "auth"}
	login := &cobra.Command{Use: "login", RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		auth, authErr := authFromFlags(cmd)
		if authErr != nil {
			return print(cmd, o, output.Failure("invalid_args", "missing required auth secret", "", 400))
		}
		idx, err := selectedInstanceIndex(cfg.Jenkins, o.Instance)
		if err != nil {
			return print(cmd, o, output.Failure(err.Error(), err.Error(), "", 400))
		}
		cfg.Jenkins.Instances[idx].Auth = auth
		if err := saveCfg(o, cfg); err != nil {
			return print(cmd, o, output.Failure("config_error", err.Error(), "", 500))
		}
		return print(cmd, o, output.Success(cfg.Jenkins.Instances[idx].Name, map[string]any{"logged_in": true}))
	}}
	addAuthFlags(login)
	c.AddCommand(login)
	c.AddCommand(&cobra.Command{Use: "logout", RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		idx, err := selectedInstanceIndex(cfg.Jenkins, o.Instance)
		if err != nil {
			return print(cmd, o, output.Failure(err.Error(), err.Error(), "", 400))
		}
		cfg.Jenkins.Instances[idx].Auth = config.AuthConfig{}
		if err := saveCfg(o, cfg); err != nil {
			return print(cmd, o, output.Failure("config_error", err.Error(), "", 500))
		}
		return print(cmd, o, output.Success(cfg.Jenkins.Instances[idx].Name, map[string]any{"logged_out": true}))
	}})
	c.AddCommand(&cobra.Command{Use: "test", RunE: func(cmd *cobra.Command, args []string) error {
		return getJSON(o, cmd, "/whoAmI/api/json", nil)
	}})
	return c
}

func selectedInstanceIndex(p config.ProductConfig, explicit string) (int, error) {
	if len(p.Instances) == 0 {
		return -1, fmt.Errorf("no_instance_configured")
	}
	target := explicit
	if target == "" {
		target = p.DefaultInstance
	}
	if target == "" && len(p.Instances) == 1 {
		return 0, nil
	}
	if target == "" {
		return -1, fmt.Errorf("instance_required")
	}
	for i := range p.Instances {
		if p.Instances[i].Name == target {
			return i, nil
		}
	}
	return -1, fmt.Errorf("instance_required")
}

func whoamiCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "whoami", RunE: func(cmd *cobra.Command, args []string) error {
		return getJSON(o, cmd, "/whoAmI/api/json", nil)
	}}
}

func serverInfoCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "server-info", RunE: func(cmd *cobra.Command, args []string) error {
		q := map[string]string{}
		if depth := mustI(cmd, "depth"); depth > 0 {
			q["depth"] = strconv.Itoa(depth)
		}
		return getJSON(o, cmd, "/api/json", q)
	}}
	c.Flags().Int("depth", 0, "Jenkins API depth for top-level metadata.")
	return c
}

func crumbCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "crumb"}
	c.AddCommand(&cobra.Command{Use: "get", RunE: func(cmd *cobra.Command, args []string) error {
		cx, err := loadCtx(o, "")
		if err != nil {
			return print(cmd, o, envelopeError(err, "config_error"))
		}
		crumb, err := cx.client.GetCrumb()
		if err != nil {
			return print(cmd, o, envelopeError(err, "server_error"))
		}
		if crumb == nil {
			return print(cmd, o, output.Success(cx.inst.Name, map[string]any{"crumb": nil, "crumb_mode": cx.inst.CrumbMode}))
		}
		return print(cmd, o, output.Success(cx.inst.Name, crumb))
	}})
	return c
}

func jobCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "job"}
	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		q := map[string]string{}
		if depth := mustI(cmd, "depth"); depth > 0 {
			q["depth"] = strconv.Itoa(depth)
		}
		if tree := mustS(cmd, "tree"); tree != "" {
			q["tree"] = tree
		} else {
			q["tree"] = "jobs[name,fullName,url,color,jobs[name,fullName,url,color]]"
		}
		return getJSON(o, cmd, "/api/json", q)
	}}
	list.Flags().Int("depth", 1, "Jenkins API depth for nested job data.")
	list.Flags().String("tree", "", "Jenkins tree selector for jobs.")
	c.AddCommand(list)
	get := &cobra.Command{Use: "get <job>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		q := map[string]string{}
		if depth := mustI(cmd, "depth"); depth > 0 {
			q["depth"] = strconv.Itoa(depth)
		}
		if tree := mustS(cmd, "tree"); tree != "" {
			q["tree"] = tree
		}
		return getJSON(o, cmd, jenkins.JobPath(args[0])+"/api/json", q)
	}}
	get.Flags().Int("depth", 0, "Jenkins API depth.")
	get.Flags().String("tree", "", "Jenkins tree selector.")
	c.AddCommand(get)
	configCmd := &cobra.Command{Use: "config"}
	configCmd.AddCommand(&cobra.Command{Use: "get <job>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return getText(o, cmd, jenkins.JobPath(args[0])+"/config.xml", "config_xml")
	}})
	update := &cobra.Command{Use: "update <job>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		body, err := readBody(cmd)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", err.Error(), "", 400))
		}
		if body == "" {
			return print(cmd, o, output.Failure("invalid_args", "missing --body, --body-file, or --body-stdin", "", 400))
		}
		return doBody(o, cmd, http.MethodPost, jenkins.JobPath(args[0])+"/config.xml", nil, body, "application/xml")
	}}
	addBodyFlags(update)
	configCmd.AddCommand(update)
	c.AddCommand(configCmd)
	create := &cobra.Command{Use: "create <job>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		body, err := readBody(cmd)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", err.Error(), "", 400))
		}
		if body == "" {
			return print(cmd, o, output.Failure("invalid_args", "missing --body, --body-file, or --body-stdin", "", 400))
		}
		parent := strings.TrimSpace(mustS(cmd, "folder"))
		createPath := "/createItem"
		name := args[0]
		if parent != "" {
			createPath = jenkins.JobPath(parent) + "/createItem"
			name = lastJobSegment(args[0])
		}
		return doBody(o, cmd, http.MethodPost, createPath, map[string]string{"name": name}, body, "application/xml")
	}}
	create.Flags().String("folder", "", "Parent folder path for creating the job.")
	addBodyFlags(create)
	c.AddCommand(create)
	copyCmd := &cobra.Command{Use: "copy <source> <target>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		parent := strings.TrimSpace(mustS(cmd, "folder"))
		createPath := "/createItem"
		name := args[1]
		if parent != "" {
			createPath = jenkins.JobPath(parent) + "/createItem"
			name = lastJobSegment(args[1])
		}
		return doBody(o, cmd, http.MethodPost, createPath, map[string]string{"name": name, "mode": "copy", "from": args[0]}, "", "application/x-www-form-urlencoded")
	}}
	copyCmd.Flags().String("folder", "", "Parent folder path for the copied job.")
	c.AddCommand(copyCmd)
	c.AddCommand(&cobra.Command{Use: "delete <job>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		return doBody(o, cmd, http.MethodPost, jenkins.JobPath(args[0])+"/doDelete", nil, "", "")
	}})
	c.AddCommand(&cobra.Command{Use: "enable <job>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return doBody(o, cmd, http.MethodPost, jenkins.JobPath(args[0])+"/enable", nil, "", "")
	}})
	c.AddCommand(&cobra.Command{Use: "disable <job>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return doBody(o, cmd, http.MethodPost, jenkins.JobPath(args[0])+"/disable", nil, "", "")
	}})
	build := &cobra.Command{Use: "build <job>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		q := map[string]string{}
		if delay := mustS(cmd, "delay"); delay != "" {
			q["delay"] = delay
		}
		return triggerBuild(o, cmd, jenkins.JobPath(args[0])+"/build", q, nil)
	}}
	build.Flags().String("delay", "", "Jenkins quiet period delay, for example 0sec.")
	c.AddCommand(build)
	buildParams := &cobra.Command{Use: "build-with-params <job>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		params, err := jenkins.ParseKeyValue(mustSA(cmd, "param"))
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", err.Error(), "Pass --param name=value for each parameter.", 400))
		}
		q := map[string]string{}
		if delay := mustS(cmd, "delay"); delay != "" {
			q["delay"] = delay
		}
		return triggerBuild(o, cmd, jenkins.JobPath(args[0])+"/buildWithParameters", q, params)
	}}
	buildParams.Flags().StringArray("param", nil, "Build parameter in name=value form; repeat for multiple parameters.")
	buildParams.Flags().String("delay", "", "Jenkins quiet period delay, for example 0sec.")
	c.AddCommand(buildParams)
	return c
}

func queueCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "queue"}
	c.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		return getJSON(o, cmd, "/queue/api/json", nil)
	}})
	c.AddCommand(&cobra.Command{Use: "get <queue-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return getJSON(o, cmd, "/queue/item/"+url.PathEscape(args[0])+"/api/json", nil)
	}})
	c.AddCommand(&cobra.Command{Use: "cancel <queue-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		return doBody(o, cmd, http.MethodPost, "/queue/cancelItem", map[string]string{"id": args[0]}, "", "")
	}})
	return c
}

func buildCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "build"}
	get := &cobra.Command{Use: "get <job> <build>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		q := map[string]string{}
		if tree := mustS(cmd, "tree"); tree != "" {
			q["tree"] = tree
		}
		if depth := mustI(cmd, "depth"); depth > 0 {
			q["depth"] = strconv.Itoa(depth)
		}
		return getJSON(o, cmd, jenkins.BuildPath(args[0], args[1])+"/api/json", q)
	}}
	get.Flags().String("tree", "", "Jenkins tree selector.")
	get.Flags().Int("depth", 0, "Jenkins API depth.")
	c.AddCommand(get)
	c.AddCommand(&cobra.Command{Use: "status <job> <build>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		return buildStatus(o, cmd, args[0], args[1])
	}})
	logCmd := &cobra.Command{Use: "log <job> <build>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		start := mustI(cmd, "start")
		if start >= 0 {
			return progressiveLog(o, cmd, args[0], args[1], start, 1, 0)
		}
		return getText(o, cmd, jenkins.BuildPath(args[0], args[1])+"/consoleText", "text")
	}}
	logCmd.Flags().Int("start", -1, "Progressive log byte offset; -1 reads consoleText.")
	c.AddCommand(logCmd)
	follow := &cobra.Command{Use: "log-follow <job> <build>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		return progressiveLog(o, cmd, args[0], args[1], mustI(cmd, "start"), mustI(cmd, "max-rounds"), mustI(cmd, "wait-ms"))
	}}
	follow.Flags().Int("start", 0, "Progressive log byte offset.")
	follow.Flags().Int("max-rounds", 30, "Maximum progressive log polling rounds.")
	follow.Flags().Int("wait-ms", 1000, "Milliseconds to wait between progressive log polling rounds.")
	c.AddCommand(follow)
	c.AddCommand(&cobra.Command{Use: "stop <job> <build>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "Pass --yes after confirming the build stop.", 400))
		}
		return doBody(o, cmd, http.MethodPost, jenkins.BuildPath(args[0], args[1])+"/stop", nil, "", "")
	}})
	c.AddCommand(&cobra.Command{Use: "artifacts <job> <build>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		return getJSON(o, cmd, jenkins.BuildPath(args[0], args[1])+"/api/json", map[string]string{"tree": "artifacts[fileName,relativePath],number,url,result,building"})
	}})
	return c
}

func artifactCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "artifact"}
	download := &cobra.Command{Use: "download <job> <build> <path>", Args: cobra.ExactArgs(3), RunE: func(cmd *cobra.Command, args []string) error {
		cx, err := loadCtx(o, args[0])
		if err != nil {
			return print(cmd, o, envelopeError(err, "config_error"))
		}
		target := jenkins.DownloadPath(mustS(cmd, "output"), args[2])
		if o.DryRun {
			return print(cmd, o, output.Success(cx.inst.Name, dryRunData(http.MethodGet, jenkins.ArtifactPath(args[0], args[1], args[2]), nil, map[string]any{"output": target})))
		}
		resp, err := cx.client.Do(jenkins.Request{Method: http.MethodGet, Path: jenkins.ArtifactPath(args[0], args[1], args[2])})
		if err != nil {
			return print(cmd, o, envelopeError(err, "server_error"))
		}
		defer resp.Body.Close()
		n, saved, err := jenkins.SaveResponseBody(resp, target)
		if err != nil {
			return print(cmd, o, output.Failure("server_error", err.Error(), "", 500))
		}
		return print(cmd, o, output.Success(cx.inst.Name, map[string]any{"path": saved, "bytes": n, "content_type": resp.Header.Get("Content-Type"), "name": jenkins.ResponseName(resp, args[2])}))
	}}
	download.Flags().String("output", "", "Local output path; defaults to the artifact file name.")
	c.AddCommand(download)
	return c
}

func pipelineCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "pipeline"}
	c.AddCommand(&cobra.Command{Use: "runs <job>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return getJSONValue(o, cmd, jenkins.JobPath(args[0])+"/wfapi/runs", nil)
	}})
	c.AddCommand(&cobra.Command{Use: "run <job> <run-id>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		return getJSON(o, cmd, jenkins.BuildPath(args[0], args[1])+"/wfapi/describe", nil)
	}})
	c.AddCommand(&cobra.Command{Use: "stages <job> <run-id>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		return getJSON(o, cmd, jenkins.BuildPath(args[0], args[1])+"/wfapi/describe", nil)
	}})
	c.AddCommand(&cobra.Command{Use: "node-log <job> <run-id> <node-id>", Args: cobra.ExactArgs(3), RunE: func(cmd *cobra.Command, args []string) error {
		return getText(o, cmd, jenkins.BuildPath(args[0], args[1])+"/execution/node/"+url.PathEscape(args[2])+"/wfapi/log", "text")
	}})
	c.AddCommand(&cobra.Command{Use: "artifacts <job> <run-id>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		return getJSONValue(o, cmd, jenkins.BuildPath(args[0], args[1])+"/wfapi/artifacts", nil)
	}})
	return c
}

func viewCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "view"}
	c.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		return getJSON(o, cmd, "/api/json", map[string]string{"tree": "views[name,url]"})
	}})
	c.AddCommand(&cobra.Command{Use: "get <view>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return getJSON(o, cmd, "/view/"+url.PathEscape(args[0])+"/api/json", nil)
	}})
	create := &cobra.Command{Use: "create <view>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		body, err := readBody(cmd)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", err.Error(), "", 400))
		}
		if body == "" {
			return print(cmd, o, output.Failure("invalid_args", "missing --body, --body-file, or --body-stdin", "", 400))
		}
		return doBody(o, cmd, http.MethodPost, "/createView", map[string]string{"name": args[0]}, body, "application/xml")
	}}
	addBodyFlags(create)
	c.AddCommand(create)
	c.AddCommand(&cobra.Command{Use: "delete <view>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		return doBody(o, cmd, http.MethodPost, "/view/"+url.PathEscape(args[0])+"/doDelete", nil, "", "")
	}})
	configCmd := &cobra.Command{Use: "config"}
	configCmd.AddCommand(&cobra.Command{Use: "get <view>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return getText(o, cmd, "/view/"+url.PathEscape(args[0])+"/config.xml", "config_xml")
	}})
	update := &cobra.Command{Use: "update <view>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		body, err := readBody(cmd)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", err.Error(), "", 400))
		}
		if body == "" {
			return print(cmd, o, output.Failure("invalid_args", "missing --body, --body-file, or --body-stdin", "", 400))
		}
		return doBody(o, cmd, http.MethodPost, "/view/"+url.PathEscape(args[0])+"/config.xml", nil, body, "application/xml")
	}}
	addBodyFlags(update)
	configCmd.AddCommand(update)
	c.AddCommand(configCmd)
	return c
}

func nodeCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "node"}
	c.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		return getJSON(o, cmd, "/computer/api/json", nil)
	}})
	c.AddCommand(&cobra.Command{Use: "get <node>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		node := url.PathEscape(args[0])
		if args[0] == "built-in" || args[0] == "master" {
			node = "(built-in)"
		}
		return getJSON(o, cmd, "/computer/"+node+"/api/json", nil)
	}})
	return c
}

func pluginCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "plugin"}
	c.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		return getJSON(o, cmd, "/pluginManager/api/json", map[string]string{"depth": "1"})
	}})
	c.AddCommand(&cobra.Command{Use: "get <plugin>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cx, err := loadCtx(o, "")
		if err != nil {
			return print(cmd, o, envelopeError(err, "config_error"))
		}
		resp, err := cx.client.Do(jenkins.Request{Method: http.MethodGet, Path: "/pluginManager/api/json", Query: map[string]string{"depth": "1"}})
		if err != nil {
			return print(cmd, o, envelopeError(err, "server_error"))
		}
		defer resp.Body.Close()
		data := jenkins.JSONMap(resp.Body)
		for _, raw := range listAny(data["plugins"]) {
			m, _ := raw.(map[string]any)
			if m["shortName"] == args[0] || m["name"] == args[0] {
				return print(cmd, o, output.Success(cx.inst.Name, m))
			}
		}
		return print(cmd, o, output.Failure("not_found", "plugin not found", "", 404))
	}})
	return c
}

func systemCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "system"}
	quiet := &cobra.Command{Use: "quiet-down", RunE: func(cmd *cobra.Command, args []string) error {
		q := map[string]string{}
		if reason := mustS(cmd, "reason"); reason != "" {
			q["reason"] = reason
		}
		if block := mustB(cmd, "block"); block {
			q["block"] = "true"
		}
		return doBody(o, cmd, http.MethodPost, "/quietDown", q, "", "")
	}}
	quiet.Flags().String("reason", "", "Quiet-down reason recorded by Jenkins.")
	quiet.Flags().Bool("block", false, "Ask Jenkins to block until quieting down completes.")
	c.AddCommand(quiet)
	c.AddCommand(&cobra.Command{Use: "cancel-quiet-down", RunE: func(cmd *cobra.Command, args []string) error {
		return doBody(o, cmd, http.MethodPost, "/cancelQuietDown", nil, "", "")
	}})
	c.AddCommand(&cobra.Command{Use: "safe-restart", RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "Pass --yes after confirming the controller restart.", 400))
		}
		return doBody(o, cmd, http.MethodPost, "/safeRestart", nil, "", "")
	}})
	return c
}

func rawAPICmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "api"}
	for _, method := range []string{"get", "post", "put", "delete"} {
		m := strings.ToUpper(method)
		cmd := &cobra.Command{Use: method + " <path>", Args: cobra.ExactArgs(1)}
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			method := strings.ToUpper(cmd.Name())
			if method == http.MethodDelete && !o.Yes {
				return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
			}
			q, err := jenkins.QueryMap(mustSA(cmd, "query"))
			if err != nil {
				return print(cmd, o, output.Failure("invalid_args", err.Error(), "Pass --query key=value.", 400))
			}
			body := ""
			if method == http.MethodPost || method == http.MethodPut {
				body, err = readBody(cmd)
				if err != nil {
					return print(cmd, o, output.Failure("invalid_args", err.Error(), "", 400))
				}
			}
			contentType := mustS(cmd, "content-type")
			if contentType == "" {
				contentType = detectContentType(body)
			}
			if method == http.MethodGet {
				return getJSONValue(o, cmd, args[0], q)
			}
			return doBody(o, cmd, method, args[0], q, body, contentType)
		}
		cmd.Flags().StringArray("query", nil, "Raw query parameter in key=value form; repeat for multiple values.")
		if m == http.MethodPost || m == http.MethodPut {
			addBodyFlags(cmd)
			cmd.Flags().String("content-type", "", "Content-Type header for raw request body.")
		}
		c.AddCommand(cmd)
	}
	return c
}

func commandsCmd() *cobra.Command {
	return &cobra.Command{Use: "commands", RunE: func(cmd *cobra.Command, args []string) error {
		return output.Print(cmd.OutOrStdout(), "json", output.Success("", map[string]any{"commands": catalog.CommandsFromCobra("jenkins", cmd.Root())}))
	}}
}

func schemaCmd() *cobra.Command {
	return &cobra.Command{Use: "schema <command>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		schema, ok := catalog.SchemaFromCobra("jenkins", args[0], cmd.Root())
		if !ok {
			return output.Print(cmd.OutOrStdout(), "json", output.Failure("not_found", "command not found", "Run jenkins commands --json to list command names.", 404))
		}
		return output.Print(cmd.OutOrStdout(), "json", output.Success("", schema))
	}}
}

func helpLLMCmd() *cobra.Command {
	return &cobra.Command{Use: "help llm", RunE: func(cmd *cobra.Command, args []string) error {
		tips := []string{
			"For agents, --json is the default way to use every jenkins command and subcommand.",
			"Use --instance when multiple Jenkins instances are configured under jenkins.instances in ~/.efp/config.yaml.",
			"Jenkins jobs inside folders are written as slash paths, for example folder/app/main.",
			"Use job build or job build-with-params to trigger builds, then inspect queue get for the executable build number.",
			"Use build status for compact build state and build log or build log-follow for console output.",
			"Use artifact download with --output so binary data is written to a file rather than stdout.",
			"Use pipeline commands only when the Pipeline REST API plugin is installed.",
			"Use --dry-run before write operations and --yes only after confirming destructive or service-affecting actions.",
			"Inspect error.code and error.hint before retrying.",
		}
		return output.Print(cmd.OutOrStdout(), "json", output.Success("", map[string]any{"tips": tips, "commands": catalog.Commands("jenkins")}))
	}}
}

func versionCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "version", RunE: func(cmd *cobra.Command, args []string) error {
		return print(cmd, o, output.Success("", map[string]any{"version": version.Version, "commit": version.Commit, "date": version.Date}))
	}}
}

func getJSON(o *Opts, cmd *cobra.Command, path string, q map[string]string) error {
	return getJSONValue(o, cmd, path, q)
}

func getJSONValue(o *Opts, cmd *cobra.Command, path string, q map[string]string) error {
	cx, err := loadCtx(o, path)
	if err != nil {
		return print(cmd, o, envelopeError(err, "config_error"))
	}
	resp, err := cx.client.Do(jenkins.Request{Method: http.MethodGet, Path: path, Query: q})
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	defer resp.Body.Close()
	return print(cmd, o, output.Success(cx.inst.Name, jenkins.JSONValue(resp.Body)))
}

func getText(o *Opts, cmd *cobra.Command, path, field string) error {
	cx, err := loadCtx(o, path)
	if err != nil {
		return print(cmd, o, envelopeError(err, "config_error"))
	}
	resp, err := cx.client.Do(jenkins.Request{Method: http.MethodGet, Path: path})
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	defer resp.Body.Close()
	text, err := jenkins.Text(resp.Body)
	if err != nil {
		return print(cmd, o, output.Failure("server_error", err.Error(), "", 500))
	}
	return print(cmd, o, output.Success(cx.inst.Name, map[string]any{field: text, "content_type": resp.Header.Get("Content-Type")}))
}

func doBody(o *Opts, cmd *cobra.Command, method, path string, q map[string]string, body, contentType string) error {
	cx, err := loadCtx(o, path)
	if err != nil {
		return print(cmd, o, envelopeError(err, "config_error"))
	}
	if o.DryRun {
		return print(cmd, o, output.Success(cx.inst.Name, dryRunData(method, path, q, body)))
	}
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	resp, err := cx.client.Do(jenkins.Request{Method: method, Path: path, Query: q, Body: reader, ContentType: contentType})
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	defer resp.Body.Close()
	data := map[string]any{"ok": true, "status": resp.StatusCode}
	if ct := resp.Header.Get("Content-Type"); strings.Contains(ct, "json") {
		data = jenkins.JSONMap(resp.Body)
		data["status"] = resp.StatusCode
	} else {
		text, _ := jenkins.Text(resp.Body)
		if text != "" {
			data["text"] = text
		}
	}
	return print(cmd, o, output.Success(cx.inst.Name, data))
}

func triggerBuild(o *Opts, cmd *cobra.Command, path string, q map[string]string, params map[string]string) error {
	cx, err := loadCtx(o, path)
	if err != nil {
		return print(cmd, o, envelopeError(err, "config_error"))
	}
	if o.DryRun {
		return print(cmd, o, output.Success(cx.inst.Name, dryRunData(http.MethodPost, path, q, params)))
	}
	var body io.Reader
	contentType := ""
	if len(params) > 0 {
		body, contentType = jenkins.FormBody(params)
	}
	resp, err := cx.client.Do(jenkins.Request{Method: http.MethodPost, Path: path, Query: q, Body: body, ContentType: contentType})
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	defer resp.Body.Close()
	location := resp.Header.Get("Location")
	return print(cmd, o, output.Success(cx.inst.Name, map[string]any{"triggered": true, "status": resp.StatusCode, "queue_url": location, "queue_id": jenkins.QueueIDFromLocation(location)}))
}

func buildStatus(o *Opts, cmd *cobra.Command, job, build string) error {
	cx, err := loadCtx(o, job)
	if err != nil {
		return print(cmd, o, envelopeError(err, "config_error"))
	}
	resp, err := cx.client.Do(jenkins.Request{Method: http.MethodGet, Path: jenkins.BuildPath(job, build) + "/api/json", Query: map[string]string{"tree": "number,url,building,result,duration,estimatedDuration,timestamp,fullDisplayName,actions[causes[*]]"}})
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	defer resp.Body.Close()
	data := jenkins.JSONMap(resp.Body)
	state := "unknown"
	if building, _ := data["building"].(bool); building {
		state = "building"
	} else if result, _ := data["result"].(string); result != "" {
		state = strings.ToLower(result)
	}
	data["state"] = state
	return print(cmd, o, output.Success(cx.inst.Name, data))
}

func progressiveLog(o *Opts, cmd *cobra.Command, job, build string, start, maxRounds, waitMS int) error {
	if start < 0 {
		start = 0
	}
	if maxRounds <= 0 {
		maxRounds = 1
	}
	cx, err := loadCtx(o, job)
	if err != nil {
		return print(cmd, o, envelopeError(err, "config_error"))
	}
	var buf bytes.Buffer
	next := start
	more := false
	for round := 0; round < maxRounds; round++ {
		resp, err := cx.client.Do(jenkins.Request{Method: http.MethodGet, Path: jenkins.BuildPath(job, build) + "/logText/progressiveText", Query: map[string]string{"start": strconv.Itoa(next)}})
		if err != nil {
			return print(cmd, o, envelopeError(err, "server_error"))
		}
		text, _ := jenkins.Text(resp.Body)
		_ = resp.Body.Close()
		buf.WriteString(text)
		if size := resp.Header.Get("X-Text-Size"); size != "" {
			if parsed, err := strconv.Atoi(size); err == nil {
				next = parsed
			}
		}
		more = strings.EqualFold(resp.Header.Get("X-More-Data"), "true")
		if !more {
			break
		}
		if round+1 < maxRounds && waitMS > 0 {
			time.Sleep(time.Duration(waitMS) * time.Millisecond)
		}
	}
	return print(cmd, o, output.Success(cx.inst.Name, map[string]any{"text": buf.String(), "start": start, "next_start": next, "more": more}))
}

func readBody(cmd *cobra.Command) (string, error) {
	body, err := files.ReadBodyFromFlags(mustS(cmd, "body"), mustS(cmd, "body-file"), mustB(cmd, "body-stdin"))
	if err != nil {
		return "", err
	}
	return body, nil
}

func addBodyFlags(cmd *cobra.Command) {
	cmd.Flags().String("body", "", "Inline request body.")
	cmd.Flags().String("body-file", "", "Path to a file containing the request body.")
	cmd.Flags().Bool("body-stdin", false, "Read the request body from stdin.")
}

func dryRunData(method, path string, q map[string]string, body any) map[string]any {
	return map[string]any{"dry_run": true, "method": method, "path": path, "query": q, "body": redactBody(body)}
}

func redactBody(v any) any {
	switch x := v.(type) {
	case map[string]string:
		out := map[string]string{}
		for k, v := range x {
			if secretKey(k) {
				out[k] = "***REDACTED***"
			} else {
				out[k] = v
			}
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for k, v := range x {
			if secretKey(k) {
				out[k] = "***REDACTED***"
			} else {
				out[k] = redactBody(v)
			}
		}
		return out
	default:
		return v
	}
}

func secretKey(k string) bool {
	k = strings.ToLower(k)
	return strings.Contains(k, "password") || strings.Contains(k, "token") || strings.Contains(k, "secret") || strings.Contains(k, "api_key") || strings.Contains(k, "apikey")
}

func detectContentType(body string) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return "application/json"
	}
	if strings.HasPrefix(trimmed, "<") {
		return "application/xml"
	}
	return "text/plain"
}

func lastJobSegment(job string) string {
	parts := strings.Split(strings.Trim(job, "/"), "/")
	if len(parts) == 0 {
		return job
	}
	return parts[len(parts)-1]
}

func listAny(v any) []any {
	switch x := v.(type) {
	case []any:
		return x
	default:
		return nil
	}
}

func mustS(cmd *cobra.Command, name string) string {
	v, _ := cmd.Flags().GetString(name)
	return v
}

func mustB(cmd *cobra.Command, name string) bool {
	v, _ := cmd.Flags().GetBool(name)
	return v
}

func mustI(cmd *cobra.Command, name string) int {
	v, _ := cmd.Flags().GetInt(name)
	return v
}

func mustSA(cmd *cobra.Command, name string) []string {
	v, _ := cmd.Flags().GetStringArray(name)
	return v
}
