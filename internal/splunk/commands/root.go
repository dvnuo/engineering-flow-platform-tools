package commands

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"

	"engineering-flow-platform-tools/internal/catalog"
	"engineering-flow-platform-tools/internal/clihelp"
	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/httpclient"
	"engineering-flow-platform-tools/internal/output"
	"engineering-flow-platform-tools/internal/splunk"
	"engineering-flow-platform-tools/internal/version"
	"github.com/spf13/cobra"
)

const product = "splunk"

type Opts struct {
	Instance, Config, Format   string
	JSON, Verbose, DryRun, Yes bool
}

type ctx struct {
	cfg    config.RootConfig
	inst   config.InstanceConfig
	client *splunk.Client
}

func NewRoot() *cobra.Command {
	cobra.EnableCommandSorting = false
	o := &Opts{Format: "table"}
	c := &cobra.Command{Use: product, SilenceErrors: true, SilenceUsage: true}
	c.PersistentFlags().StringVar(&o.Instance, "instance", "", "Configured splunk instance name.")
	c.PersistentFlags().StringVar(&o.Config, "config", "", "Path to EFP config file.")
	c.PersistentFlags().BoolVar(&o.JSON, "json", false, "Print JSON envelope.")
	c.PersistentFlags().StringVar(&o.Format, "format", "table", "Output format: table|json|yaml.")
	c.PersistentFlags().BoolVar(&o.Verbose, "verbose", false, "Print non-secret diagnostics when available.")
	c.PersistentFlags().BoolVar(&o.DryRun, "dry-run", false, "Preview a request without sending it.")
	c.PersistentFlags().BoolVar(&o.Yes, "yes", false, "Confirm destructive or service-affecting operations.")
	c.AddCommand(instanceCmd(o), authCmd(o), searchCmd(o), savedCmd(o), indexCmd(o), apiCmd(o), commandsCmd(o), schemaCmd(o), helpLLMCmd(o), versionCmd(o))
	clihelp.ApplyCatalogHelp(c, clihelp.ProductHelp{
		Product: product,
		Binary:  product,
		Short:   "Run bounded Splunk searches and inspect indexes and saved searches",
		Long: strings.TrimSpace(`splunk is a terminal-invoked CLI for agents that need read-only, JSON-first access to Splunk Enterprise through the management REST API: search jobs with explicit time ranges and result caps, saved searches, and index metadata.

Every search is guarded: SPL that writes data or triggers actions (delete, outputlookup, collect, sendemail, script, ...) is refused before a job is created, results are capped by the instance max_results, and long field values are truncated unless --output writes them to a file.

Configuration uses the shared EFP config from environment variables injected by managed runtimes (for example EFP_SPLUNK_DEFAULT_INSTANCE, EFP_SPLUNK_INSTANCES_0_BASE_URL) or the config file, normally ~/.efp/config.yaml (local), under the splunk node.`),
		Examples: []string{
			`splunk auth test --json`,
			`splunk search run --query "index=main error | head 100" --earliest -1h --json`,
			`splunk search oneshot --query "index=main | stats count by host" --earliest -15m --json`,
			`splunk saved run "Errors last hour" --json`,
			`splunk index list --json`,
			`splunk help llm --json`,
		},
		Instructions: "copy cmd/splunk/splunk-cli.instructions.md to ~/.copilot/instructions/splunk-cli.instructions.md.",
		Groups: map[string]string{
			"instance":   "Manage configured Splunk instances.",
			"auth":       "Manage Splunk credentials stored in the EFP config.",
			"search":     "Run bounded, read-only SPL searches and inspect search jobs.",
			"search.job": "Inspect, page, and cancel existing search jobs by sid.",
			"saved":      "List and dispatch saved searches.",
			"index":      "Inspect index metadata.",
			"api":        "Call raw read-only Splunk REST paths on the selected instance.",
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
	cfg, _, err := config.LoadShared(o.Config)
	return cfg, err
}

func saveCfg(o *Opts, cfg config.RootConfig) error {
	return config.SaveShared(o.Config, cfg)
}

func loadCtx(o *Opts) (*ctx, error) {
	cfg, err := loadCfg(o)
	if err != nil {
		return nil, err
	}
	inst, err := splunk.ResolveInstance(cfg, o.Instance)
	if err != nil {
		return nil, err
	}
	client, err := splunk.New(inst)
	if err != nil {
		return nil, err
	}
	return &ctx{cfg: cfg, inst: inst, client: client}, nil
}

func envelopeError(err error, fallbackCode string) output.Envelope {
	var httpErr *httpclient.HTTPError
	if errors.As(err, &httpErr) {
		return output.Failure(httpErr.Code, httpErr.Message, httpErr.Hint, httpErr.Status)
	}
	msg := httpclient.SanitizeErrorText(err.Error())
	if isStableErrorCode(msg) {
		return output.Failure(msg, msg, stableHint(msg), 400)
	}
	if fallbackCode == "" {
		fallbackCode = "server_error"
	}
	return output.Failure(fallbackCode, msg, "", 500)
}

func isStableErrorCode(code string) bool {
	switch code {
	case "config_missing", "config_error", "no_instance_configured", "instance_required", "ambiguous_instance", "instance_url_mismatch", "invalid_args", "not_found", "not_supported", "auth_failed", "permission_denied", "network_error", "server_error":
		return true
	default:
		return false
	}
}

func stableHint(code string) string {
	switch code {
	case "no_instance_configured":
		return "Add a Splunk instance with splunk instance add <name> --base-url https://splunk-api.example.test:8089 --token-stdin --default --json."
	case "instance_required":
		return "Pass --instance <name> or set a default with splunk instance default <name> --json."
	default:
		return ""
	}
}

func authFromFlags(cmd *cobra.Command) (config.AuthConfig, error) {
	auth := config.AuthConfig{Type: mustS(cmd, "auth-type"), Username: mustS(cmd, "username")}
	if mustB(cmd, "password-stdin") {
		auth.Password = readSecret(cmd)
	}
	if mustB(cmd, "token-stdin") {
		auth.Token = readSecret(cmd)
	}
	auth.NormalizeType()
	switch auth.Type {
	case "basic_password":
		if auth.Username == "" || auth.Password == "" {
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

func readSecret(cmd *cobra.Command) string {
	secret, _ := io.ReadAll(cmd.InOrStdin())
	return strings.TrimRight(string(secret), "\r\n")
}

func addAuthFlags(cmd *cobra.Command) {
	cmd.Flags().String("username", "", "Username for session login (auth.type basic_password).")
	cmd.Flags().String("auth-type", "", "Authentication type: bearer_token (Splunk authentication token) or basic_password (session login).")
	cmd.Flags().Bool("password-stdin", false, "Read the session-login password from stdin.")
	cmd.Flags().Bool("token-stdin", false, "Read the Splunk authentication token from stdin.")
}

func addInstanceFieldFlags(cmd *cobra.Command, update bool) {
	prefix := ""
	if update {
		prefix = "New "
	}
	cmd.Flags().String("base-url", "", prefix+"Splunk management REST base URL, for example https://splunk-api.example.test:8089.")
	cmd.Flags().String("default-index", "", prefix+"Index prepended as index=<name> when a query does not constrain the index.")
	cmd.Flags().String("default-earliest", "", prefix+"earliest_time used when --earliest is omitted (default -1h).")
	cmd.Flags().Int("max-results", 0, prefix+"Hard cap on results one search may return (default 1000).")
}

const missingAuthHint = "Use --token-stdin for a Splunk authentication token, or --username with --password-stdin for session login."

func instanceCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "instance"}
	c.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		for i := range cfg.Splunk.Instances {
			cfg.Splunk.Instances[i].Auth = config.RedactAuth(cfg.Splunk.Instances[i].Auth)
		}
		return print(cmd, o, output.Success("", map[string]any{"instances": cfg.Splunk.Instances, "default_instance": cfg.Splunk.DefaultInstance}))
	}})
	c.AddCommand(&cobra.Command{Use: "get <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		for _, in := range cfg.Splunk.Instances {
			if in.Name == args[0] {
				in.Auth = config.RedactAuth(in.Auth)
				return print(cmd, o, output.Success(in.Name, in))
			}
		}
		return print(cmd, o, output.Failure("not_found", "instance not found", "", 404))
	}})
	add := &cobra.Command{Use: "add <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := loadCfg(o)
		baseURL := strings.TrimSpace(mustS(cmd, "base-url"))
		if baseURL == "" {
			return print(cmd, o, output.Failure("invalid_args", "--base-url is required", "Pass the Splunk management REST URL, usually https://<host>:8089.", 400))
		}
		if mustI(cmd, "max-results") < 0 {
			return print(cmd, o, output.Failure("invalid_args", "--max-results must not be negative", "", 400))
		}
		auth, authErr := authFromFlags(cmd)
		if authErr != nil {
			return print(cmd, o, output.Failure("invalid_args", "missing required auth secret", missingAuthHint, 400))
		}
		for _, in := range cfg.Splunk.Instances {
			if in.Name == args[0] {
				return print(cmd, o, output.Failure("invalid_args", "instance already exists", "Use splunk instance update or choose another name.", 400))
			}
		}
		in := config.InstanceConfig{Name: args[0], BaseURL: baseURL, Auth: auth, DefaultIndex: strings.TrimSpace(mustS(cmd, "default-index")), DefaultEarliest: strings.TrimSpace(mustS(cmd, "default-earliest")), MaxResults: mustI(cmd, "max-results")}
		cfg.Splunk.Instances = append(cfg.Splunk.Instances, in)
		if mustB(cmd, "default") {
			cfg.Splunk.DefaultInstance = args[0]
		}
		if err := saveCfg(o, cfg); err != nil {
			return print(cmd, o, output.Failure("config_error", err.Error(), "", 500))
		}
		return print(cmd, o, output.Success(args[0], map[string]any{"added": true}))
	}}
	addInstanceFieldFlags(add, false)
	addAuthFlags(add)
	add.Flags().Bool("default", false, "Make the added Splunk instance the default instance.")
	c.AddCommand(add)
	update := &cobra.Command{Use: "update <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		for i := range cfg.Splunk.Instances {
			in := &cfg.Splunk.Instances[i]
			if in.Name != args[0] {
				continue
			}
			changed := false
			if v := strings.TrimSpace(mustS(cmd, "base-url")); v != "" {
				in.BaseURL = v
				changed = true
			}
			if cmd.Flags().Changed("default-index") {
				in.DefaultIndex = strings.TrimSpace(mustS(cmd, "default-index"))
				changed = true
			}
			if cmd.Flags().Changed("default-earliest") {
				in.DefaultEarliest = strings.TrimSpace(mustS(cmd, "default-earliest"))
				changed = true
			}
			if cmd.Flags().Changed("max-results") {
				if mustI(cmd, "max-results") < 0 {
					return print(cmd, o, output.Failure("invalid_args", "--max-results must not be negative", "", 400))
				}
				in.MaxResults = mustI(cmd, "max-results")
				changed = true
			}
			if !changed {
				return print(cmd, o, output.Failure("invalid_args", "nothing to update", "Pass --base-url, --default-index, --default-earliest, or --max-results.", 400))
			}
			if err := saveCfg(o, cfg); err != nil {
				return print(cmd, o, output.Failure("config_error", err.Error(), "", 500))
			}
			return print(cmd, o, output.Success(args[0], map[string]any{"updated": true}))
		}
		return print(cmd, o, output.Failure("not_found", "instance not found", "", 404))
	}}
	addInstanceFieldFlags(update, true)
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
		for _, in := range cfg.Splunk.Instances {
			if in.Name != args[0] {
				out = append(out, in)
			}
		}
		cfg.Splunk.Instances = out
		if cfg.Splunk.DefaultInstance == args[0] {
			cfg.Splunk.DefaultInstance = ""
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
			return print(cmd, o, output.Success("", map[string]any{"default_instance": cfg.Splunk.DefaultInstance}))
		}
		found := false
		for _, in := range cfg.Splunk.Instances {
			if in.Name == args[0] {
				found = true
			}
		}
		if !found {
			return print(cmd, o, output.Failure("not_found", "instance not found", "Run splunk instance list --json.", 404))
		}
		cfg.Splunk.DefaultInstance = args[0]
		if err := saveCfg(o, cfg); err != nil {
			return print(cmd, o, output.Failure("config_error", err.Error(), "", 500))
		}
		return print(cmd, o, output.Success(args[0], map[string]any{"default_instance": args[0]}))
	}})
	return c
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
			return print(cmd, o, output.Failure("invalid_args", "missing required auth secret", missingAuthHint, 400))
		}
		idx, err := selectedInstanceIndex(cfg.Splunk, o.Instance)
		if err != nil {
			return print(cmd, o, output.Failure(err.Error(), err.Error(), stableHint(err.Error()), 400))
		}
		cfg.Splunk.Instances[idx].Auth = auth
		if err := saveCfg(o, cfg); err != nil {
			return print(cmd, o, output.Failure("config_error", err.Error(), "", 500))
		}
		return print(cmd, o, output.Success(cfg.Splunk.Instances[idx].Name, map[string]any{"logged_in": true, "auth_type": auth.Type}))
	}}
	addAuthFlags(login)
	c.AddCommand(login)
	c.AddCommand(&cobra.Command{Use: "logout", RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "Pass --yes after confirming that the stored credentials should be removed.", 400))
		}
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		idx, err := selectedInstanceIndex(cfg.Splunk, o.Instance)
		if err != nil {
			return print(cmd, o, output.Failure(err.Error(), err.Error(), stableHint(err.Error()), 400))
		}
		cfg.Splunk.Instances[idx].Auth = config.AuthConfig{}
		if err := saveCfg(o, cfg); err != nil {
			return print(cmd, o, output.Failure("config_error", err.Error(), "", 500))
		}
		return print(cmd, o, output.Success(cfg.Splunk.Instances[idx].Name, map[string]any{"logged_out": true}))
	}})
	c.AddCommand(&cobra.Command{Use: "test", RunE: func(cmd *cobra.Command, args []string) error {
		cx, err := loadCtx(o)
		if err != nil {
			return print(cmd, o, envelopeError(err, "config_error"))
		}
		content, err := cx.client.CurrentContext()
		if err != nil {
			return print(cmd, o, envelopeError(err, "server_error"))
		}
		return print(cmd, o, output.Success(cx.inst.Name, map[string]any{"authenticated": true, "username": content["username"], "roles": stringList(content["roles"]), "auth_type": cx.inst.Auth.Type}))
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

func commandsCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "commands", RunE: func(cmd *cobra.Command, args []string) error {
		return print(cmd, o, output.Success("", map[string]any{"commands": catalog.CommandsFromCobra(product, cmd.Root())}))
	}}
}

func schemaCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "schema <command>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		schema, ok := catalog.SchemaFromCobra(product, args[0], cmd.Root())
		if !ok {
			return print(cmd, o, output.Failure("not_found", "command not found", "Run "+product+" commands --json to list command names.", 404))
		}
		return print(cmd, o, output.Success("", schema))
	}}
}

func helpLLMCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "help llm", RunE: func(cmd *cobra.Command, args []string) error {
		tips := []string{
			"For agents, --json is the default way to use every " + product + " command and subcommand.",
			"Run " + product + " commands --json, then " + product + " schema search.run --json before constructing a complex call.",
			"Use --instance when several Splunk instances are configured; without it the default instance is used.",
			"Always give an explicit time range: pass --earliest (for example -15m, -1h, -24h@h) and --latest (default now). Without --earliest the instance default_earliest or -1h applies; never search all time.",
			"Start narrow: end the SPL with | head 100 or aggregate with | stats count by host (or another field) before asking for raw events, and add fields with --fields _time,host,message to keep results small.",
			"Never dump raw events beyond the cap: --count is limited by the instance max_results (default 1000) and each field value is cut at --max-field-chars (default 2000). data.results_truncated and data.fields_truncated tell you when more exists; page with --offset or aggregate instead of raising the cap.",
			"Re-run with --output results.json when a result set is large or field values are truncated: the untruncated results JSON is written to that file and only path, bytes, and counts are printed.",
			"Results are read-only: SPL that writes data or triggers actions (" + strings.Join(splunk.BlockedCommands(), ", ") + ") is refused with spl_blocked before any job is created, saved searches are checked the same way, and search job cancel requires --yes.",
			"When the query does not name an index and the instance sets default_index, index=<default_index> is prepended automatically; use " + product + " index list --json to discover indexes and their event counts.",
			"Use search run for normal searches (creates a job, polls until done, then reads results), search oneshot for quick aggregate queries, and search job results <sid> to page a finished job. wait_timeout means the job was cancelled after --timeout-sec; narrow the search or raise the timeout.",
			"Use --dry-run to see the exact SPL and job parameters (after index injection and the guard) without contacting Splunk.",
			"Inspect error.code and error.hint before retrying: auth_failed means the token expired or the session is invalid, search_failed carries the Splunk messages in data.messages.",
		}
		return print(cmd, o, output.Success("", map[string]any{"tips": tips, "commands": catalog.CommandsFromCobra(product, cmd.Root())}))
	}}
}

func versionCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "version", RunE: func(cmd *cobra.Command, args []string) error {
		return print(cmd, o, output.Success("", map[string]any{"version": version.Version, "commit": version.Commit, "date": version.Date}))
	}}
}

func dryRunData(method, path string, form url.Values, extra map[string]any) map[string]any {
	data := map[string]any{"dry_run": true, "method": method, "path": path}
	if len(form) > 0 {
		values := map[string]any{}
		for key := range form {
			if vs := form[key]; len(vs) == 1 {
				values[key] = vs[0]
			} else {
				values[key] = vs
			}
		}
		data["form"] = values
	}
	for k, v := range extra {
		data[k] = v
	}
	return data
}

func stringList(v any) []string {
	out := []string{}
	switch x := v.(type) {
	case []any:
		for _, item := range x {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
	case []string:
		out = append(out, x...)
	case string:
		if x != "" {
			out = append(out, x)
		}
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
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

func mustSS(cmd *cobra.Command, name string) []string {
	v, _ := cmd.Flags().GetStringSlice(name)
	return v
}

func mustSA(cmd *cobra.Command, name string) []string {
	v, _ := cmd.Flags().GetStringArray(name)
	return v
}
