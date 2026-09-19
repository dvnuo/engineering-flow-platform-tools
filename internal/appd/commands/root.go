package commands

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"engineering-flow-platform-tools/internal/appd"
	"engineering-flow-platform-tools/internal/catalog"
	"engineering-flow-platform-tools/internal/clihelp"
	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/httpclient"
	"engineering-flow-platform-tools/internal/instance"
	"engineering-flow-platform-tools/internal/output"
	"engineering-flow-platform-tools/internal/version"
	"github.com/spf13/cobra"
)

const product = "appd"

// defaultSnapshotGetMins is the window snapshot get searches when no time
// flag is given: the Controller's default snapshot retention of 14 days.
const defaultSnapshotGetMins = 14 * 24 * 60

type Opts struct {
	Instance, Config, Format   string
	JSON, Verbose, DryRun, Yes bool
}

type ctx struct {
	cfg    config.RootConfig
	inst   config.InstanceConfig
	client *appd.Client
}

func NewRoot() *cobra.Command {
	cobra.EnableCommandSorting = false
	o := &Opts{Format: "table"}
	c := &cobra.Command{Use: product, SilenceErrors: true, SilenceUsage: true}
	c.PersistentFlags().StringVar(&o.Instance, "instance", "", "Configured appd instance name.")
	c.PersistentFlags().StringVar(&o.Config, "config", "", "Path to EFP config file.")
	c.PersistentFlags().BoolVar(&o.JSON, "json", false, "Print JSON envelope.")
	c.PersistentFlags().StringVar(&o.Format, "format", "table", "Output format: table|json|yaml.")
	c.PersistentFlags().BoolVar(&o.Verbose, "verbose", false, "Print non-secret diagnostics when available.")
	c.PersistentFlags().BoolVar(&o.DryRun, "dry-run", false, "Preview a request without sending it.")
	c.PersistentFlags().BoolVar(&o.Yes, "yes", false, "Confirm destructive or service-affecting operations.")
	c.AddCommand(instanceCmd(o), authCmd(o), appCmd(o), tierCmd(o), nodeCmd(o), btCmd(o), backendCmd(o), metricCmd(o), snapshotCmd(o), violationCmd(o), eventCmd(o), apiCmd(o), commandsCmd(o), schemaCmd(o), helpLLMCmd(o), versionCmd(o))
	clihelp.ApplyCatalogHelp(c, clihelp.ProductHelp{
		Product: product,
		Binary:  product,
		Short:   "Inspect AppDynamics applications, tiers, nodes, metrics, snapshots, and events",
		Long: strings.TrimSpace(`appd is a terminal-invoked CLI for agents that need read-only, JSON-first access to the AppDynamics Controller REST API: applications, tiers, nodes, business transactions, backends, metrics, transaction snapshots, health-rule violations, and events.

Every time-ranged command takes an explicit window (--duration-mins, --start-time/--end-time, or --before-time/--after-time) and echoes the resolved window as data.time_range. Credentials are an AppDynamics API Client (OAuth client credentials, exchanged for a short-lived bearer token kept only in memory) or a user@account basic login.

Configuration uses the shared EFP config from environment variables injected by managed runtimes (for example EFP_APPD_DEFAULT_INSTANCE, EFP_APPD_INSTANCES_0_BASE_URL, EFP_APPD_INSTANCES_0_ACCOUNT, EFP_APPD_INSTANCES_0_AUTH_TYPE=api_client) or the config file, normally ~/.efp/config.yaml (local), under the appd node.`),
		Examples: []string{
			`appd app list --json`,
			`appd bt list --app ecommerce --json`,
			`appd snapshot list --app ecommerce --errors-only --duration-mins 60 --json`,
			`appd violation list --app ecommerce --duration-mins 120 --json`,
			`appd event list --app ecommerce --event-types APPLICATION_DEPLOYMENT --duration-mins 1440 --json`,
			`appd metric preset --app ecommerce --preset bt-response-time --tier web --bt /checkout --json`,
			`appd help llm --json`,
		},
		Instructions: "copy cmd/appd/appd-cli.instructions.md to ~/.copilot/instructions/appd-cli.instructions.md.",
		Groups: map[string]string{
			"instance":  "Manage configured AppDynamics Controller instances.",
			"auth":      "Manage AppDynamics credentials stored in the EFP config.",
			"app":       "List and inspect AppDynamics applications.",
			"tier":      "Inspect the tiers of an application.",
			"node":      "Inspect the nodes of an application.",
			"bt":        "Inspect business transactions of an application.",
			"backend":   "Inspect the backends detected for an application.",
			"metric":    "Browse the metric tree and fetch metric values.",
			"snapshot":  "Inspect transaction snapshots.",
			"violation": "Inspect health-rule violations.",
			"event":     "Inspect Controller events such as errors and deployments.",
			"api":       "Call raw read-only Controller REST paths on the selected instance.",
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
	res, err := instance.Resolve(cfg.AppD, o.Instance, "", product)
	if err != nil {
		return nil, err
	}
	client, err := appd.New(res.Instance)
	if err != nil {
		return nil, err
	}
	return &ctx{cfg: cfg, inst: res.Instance, client: client}, nil
}

func envelopeError(err error, fallbackCode string) output.Envelope {
	var httpErr *httpclient.HTTPError
	if errors.As(err, &httpErr) {
		return output.Failure(httpErr.Code, httpErr.Message, httpErr.Hint, httpErr.Status)
	}
	msg := httpclient.SanitizeErrorText(err.Error())
	if isStableErrorCode(msg) {
		return output.Failure(msg, msg, hintForCode(msg), 400)
	}
	if fallbackCode == "" {
		fallbackCode = "server_error"
	}
	return output.Failure(fallbackCode, msg, "", 500)
}

func isStableErrorCode(code string) bool {
	switch code {
	case "config_missing", "config_error", "no_instance_configured", "instance_required", "ambiguous_instance", "instance_url_mismatch", "invalid_args", "not_found", "not_supported", "auth_failed", "permission_denied", "rate_limited", "network_error", "server_error":
		return true
	default:
		return false
	}
}

func hintForCode(code string) string {
	switch code {
	case "no_instance_configured":
		return "Add a Controller with appd instance add <name> --base-url ... --account ... or configure the appd node."
	case "instance_required", "ambiguous_instance":
		return "Pass --instance <name> or set a default with appd instance default <name>."
	default:
		return ""
	}
}

// withGet resolves the instance, honours --dry-run, performs one Controller
// GET, and prints the envelope built by fn from the decoded JSON value.
func withGet(o *Opts, cmd *cobra.Command, path string, q map[string]string, fn func(cx *ctx, v any) output.Envelope) error {
	cx, err := loadCtx(o)
	if err != nil {
		return print(cmd, o, envelopeError(err, "config_error"))
	}
	if o.DryRun {
		preview, err := cx.client.Preview(path, q)
		if err != nil {
			return print(cmd, o, envelopeError(err, "invalid_args"))
		}
		return print(cmd, o, output.Success(cx.inst.Name, preview))
	}
	v, err := cx.client.Get(path, q)
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	return print(cmd, o, fn(cx, v))
}

func authFromFlags(cmd *cobra.Command) (config.AuthConfig, error) {
	username := strings.TrimSpace(mustS(cmd, "username"))
	authType := strings.ToLower(strings.TrimSpace(mustS(cmd, "auth-type")))
	pw, key := mustB(cmd, "password-stdin"), mustB(cmd, "api-key-stdin")
	auth := config.AuthConfig{Type: authType, Username: username}
	if pw && key {
		return auth, errors.New("use only one of --password-stdin and --api-key-stdin")
	}
	if pw || key {
		secret, _ := io.ReadAll(cmd.InOrStdin())
		value := strings.TrimRight(string(secret), "\r\n")
		if pw {
			auth.Password = value
		} else {
			auth.APIKey = value
		}
	}
	if auth.Type == "" {
		switch {
		case auth.APIKey != "":
			auth.Type = appd.AuthAPIClient
		case auth.Password != "":
			auth.Type = appd.AuthBasic
		}
	}
	auth.NormalizeType()
	switch auth.Type {
	case appd.AuthAPIClient:
		if auth.Username == "" || auth.APIKey == "" {
			return auth, errors.New("api_client auth needs --username <api-client-name> and the client secret on stdin via --api-key-stdin")
		}
	case appd.AuthBasic:
		if auth.Username == "" || auth.Password == "" {
			return auth, errors.New("basic_password auth needs --username and the password on stdin via --password-stdin")
		}
	case "":
		return auth, errors.New("missing credentials")
	default:
		return auth, errors.New("unsupported --auth-type " + auth.Type + "; use api_client or basic_password")
	}
	return auth, nil
}

func authFlagsFailure(err error) output.Envelope {
	return output.Failure("invalid_args", err.Error(), "Use --auth-type api_client --username <api-client-name> --api-key-stdin, or --auth-type basic_password --username <user> --password-stdin.", 400)
}

func addAuthFlags(cmd *cobra.Command) {
	cmd.Flags().String("username", "", "API client name (api_client) or user name (basic_password); a bare name is qualified with the instance account as name@account.")
	cmd.Flags().String("auth-type", "", "Authentication type: api_client (OAuth client credentials) or basic_password.")
	cmd.Flags().Bool("password-stdin", false, "Read the basic_password password from stdin.")
	cmd.Flags().Bool("api-key-stdin", false, "Read the API client secret from stdin (stored as auth.api_key).")
}

func addAppFlag(cmd *cobra.Command) {
	cmd.Flags().String("app", "", "AppDynamics application name or numeric id.")
}

func addTimeFlags(cmd *cobra.Command, defaultMins int) {
	cmd.Flags().Int("duration-mins", defaultMins, "Window length in minutes: alone it means BEFORE_NOW; with --before-time or --after-time it sizes that window.")
	cmd.Flags().String("start-time", "", "Window start as epoch milliseconds or RFC3339; requires --end-time (BETWEEN_TIMES).")
	cmd.Flags().String("end-time", "", "Window end as epoch milliseconds or RFC3339; requires --start-time (BETWEEN_TIMES).")
	cmd.Flags().String("before-time", "", "Window end as epoch milliseconds or RFC3339; the window is --duration-mins before it (BEFORE_TIME).")
	cmd.Flags().String("after-time", "", "Window start as epoch milliseconds or RFC3339; the window is --duration-mins after it (AFTER_TIME).")
}

func timeRangeFromFlags(cmd *cobra.Command) (appd.TimeRange, error) {
	return appd.ResolveTimeRange(appd.TimeRangeInput{
		DurationMins: mustI(cmd, "duration-mins"),
		StartTime:    mustS(cmd, "start-time"),
		EndTime:      mustS(cmd, "end-time"),
		BeforeTime:   mustS(cmd, "before-time"),
		AfterTime:    mustS(cmd, "after-time"),
	})
}

func timeRangeFailure(err error) output.Envelope {
	return output.Failure("invalid_args", err.Error(), "Use --duration-mins N (before now), --start-time plus --end-time, or --before-time/--after-time plus --duration-mins; timestamps are epoch milliseconds or RFC3339.", 400)
}

func requireApp(cmd *cobra.Command) (string, *output.Envelope) {
	app := strings.TrimSpace(mustS(cmd, "app"))
	if app == "" {
		e := output.Failure("invalid_args", "--app is required", "Pass the application name or numeric id; run appd app list --json to discover it.", 400)
		return "", &e
	}
	return app, nil
}

func appPath(app string) string {
	return "/controller/rest/applications/" + url.PathEscape(app)
}

func invalidBaseURL() output.Envelope {
	return output.Failure("invalid_args", "invalid --base-url", "Use an absolute http(s) Controller URL such as https://appd.example.test:8090.", 400)
}

func instanceCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "instance"}
	c.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		instances := make([]config.InstanceConfig, 0, len(cfg.AppD.Instances))
		for _, in := range cfg.AppD.Instances {
			in.Auth = config.RedactAuth(in.Auth)
			instances = append(instances, in)
		}
		return print(cmd, o, output.Success("", map[string]any{"instances": instances, "default_instance": cfg.AppD.DefaultInstance}))
	}})
	c.AddCommand(&cobra.Command{Use: "get <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		for _, in := range cfg.AppD.Instances {
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
			return print(cmd, o, output.Failure("invalid_args", "--base-url is required", "Pass the Controller base URL, for example https://appd.example.test:8090.", 400))
		}
		if _, err := appd.NormalizeBaseURL(baseURL); err != nil {
			return print(cmd, o, invalidBaseURL())
		}
		for _, in := range cfg.AppD.Instances {
			if in.Name == args[0] {
				return print(cmd, o, output.Failure("invalid_args", "instance already exists", "Use appd instance update or appd auth login --instance "+args[0]+" to change it.", 400))
			}
		}
		account := strings.TrimSpace(mustS(cmd, "account"))
		auth, authErr := authFromFlags(cmd)
		if authErr != nil {
			return print(cmd, o, authFlagsFailure(authErr))
		}
		if account == "" && !strings.Contains(auth.Username, "@") {
			return print(cmd, o, output.Failure("invalid_args", "--account is required unless --username is already written as name@account", "Pass the Controller account name with --account.", 400))
		}
		in := config.InstanceConfig{Name: args[0], BaseURL: baseURL, RESTPath: mustS(cmd, "rest-path"), Account: account, Auth: auth}
		cfg.AppD.Instances = append(cfg.AppD.Instances, in)
		if mustB(cmd, "default") {
			cfg.AppD.DefaultInstance = args[0]
		}
		if err := saveCfg(o, cfg); err != nil {
			return print(cmd, o, output.Failure("config_error", err.Error(), "", 500))
		}
		return print(cmd, o, output.Success(args[0], map[string]any{"added": true, "account": account, "auth_type": auth.Type}))
	}}
	add.Flags().String("base-url", "", "AppDynamics Controller base URL, with or without the /controller suffix.")
	add.Flags().String("account", "", "Controller account name used to qualify the API client name or user as name@account.")
	add.Flags().String("rest-path", "", "Reserved REST path override; normally empty because appd builds /controller/rest paths itself.")
	addAuthFlags(add)
	add.Flags().Bool("default", false, "Make the added AppDynamics instance the default instance.")
	c.AddCommand(add)
	update := &cobra.Command{Use: "update <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		for i := range cfg.AppD.Instances {
			if cfg.AppD.Instances[i].Name != args[0] {
				continue
			}
			changed := false
			if v := strings.TrimSpace(mustS(cmd, "base-url")); v != "" {
				if _, err := appd.NormalizeBaseURL(v); err != nil {
					return print(cmd, o, invalidBaseURL())
				}
				cfg.AppD.Instances[i].BaseURL = v
				changed = true
			}
			if v := mustS(cmd, "rest-path"); v != "" {
				cfg.AppD.Instances[i].RESTPath = v
				changed = true
			}
			if v := strings.TrimSpace(mustS(cmd, "account")); v != "" {
				cfg.AppD.Instances[i].Account = v
				changed = true
			}
			if !changed {
				return print(cmd, o, output.Failure("invalid_args", "nothing to update", "Pass --base-url, --account, or --rest-path.", 400))
			}
			if err := saveCfg(o, cfg); err != nil {
				return print(cmd, o, output.Failure("config_error", err.Error(), "", 500))
			}
			return print(cmd, o, output.Success(args[0], map[string]any{"updated": true, "account": cfg.AppD.Instances[i].Account}))
		}
		return print(cmd, o, output.Failure("not_found", "instance not found", "", 404))
	}}
	update.Flags().String("base-url", "", "New AppDynamics Controller base URL.")
	update.Flags().String("account", "", "New Controller account name.")
	update.Flags().String("rest-path", "", "Reserved REST path override; normally empty.")
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
		found := false
		for _, in := range cfg.AppD.Instances {
			if in.Name != args[0] {
				out = append(out, in)
			} else {
				found = true
			}
		}
		if !found {
			return print(cmd, o, output.Failure("not_found", "instance not found", "", 404))
		}
		cfg.AppD.Instances = out
		if cfg.AppD.DefaultInstance == args[0] {
			cfg.AppD.DefaultInstance = ""
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
			return print(cmd, o, output.Success("", map[string]any{"default_instance": cfg.AppD.DefaultInstance}))
		}
		found := false
		for _, in := range cfg.AppD.Instances {
			if in.Name == args[0] {
				found = true
			}
		}
		if !found {
			return print(cmd, o, output.Failure("not_found", "instance not found", "Run appd instance list --json to see configured instances.", 404))
		}
		cfg.AppD.DefaultInstance = args[0]
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
			return print(cmd, o, authFlagsFailure(authErr))
		}
		idx, err := selectedInstanceIndex(cfg.AppD, o.Instance)
		if err != nil {
			return print(cmd, o, output.Failure(err.Error(), err.Error(), hintForCode(err.Error()), 400))
		}
		if cfg.AppD.Instances[idx].Account == "" && !strings.Contains(auth.Username, "@") {
			return print(cmd, o, output.Failure("invalid_args", "instance "+cfg.AppD.Instances[idx].Name+" has no account and --username is not written as name@account", "Run appd instance update "+cfg.AppD.Instances[idx].Name+" --account <account> first, or pass --username name@account.", 400))
		}
		cfg.AppD.Instances[idx].Auth = auth
		if err := saveCfg(o, cfg); err != nil {
			return print(cmd, o, output.Failure("config_error", err.Error(), "", 500))
		}
		return print(cmd, o, output.Success(cfg.AppD.Instances[idx].Name, map[string]any{"logged_in": true, "auth_type": auth.Type}))
	}}
	addAuthFlags(login)
	c.AddCommand(login)
	c.AddCommand(&cobra.Command{Use: "logout", RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "Pass --yes after confirming that the stored credentials should be cleared.", 400))
		}
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", err.Error(), "", 404))
		}
		idx, err := selectedInstanceIndex(cfg.AppD, o.Instance)
		if err != nil {
			return print(cmd, o, output.Failure(err.Error(), err.Error(), hintForCode(err.Error()), 400))
		}
		cfg.AppD.Instances[idx].Auth = config.AuthConfig{}
		if err := saveCfg(o, cfg); err != nil {
			return print(cmd, o, output.Failure("config_error", err.Error(), "", 500))
		}
		return print(cmd, o, output.Success(cfg.AppD.Instances[idx].Name, map[string]any{"logged_out": true}))
	}})
	c.AddCommand(&cobra.Command{Use: "test", RunE: func(cmd *cobra.Command, args []string) error {
		return withGet(o, cmd, "/controller/rest/applications", nil, func(cx *ctx, v any) output.Envelope {
			return output.Success(cx.inst.Name, map[string]any{
				"authenticated":     true,
				"application_count": len(appd.ListAny(v)),
				"auth_type":         cx.client.AuthType(),
				"account":           cx.inst.Account,
				"identity":          cx.client.Identity(),
			})
		})
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

func appCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "app"}
	c.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		return withGet(o, cmd, "/controller/rest/applications", nil, func(cx *ctx, v any) output.Envelope {
			apps := appd.ListAny(v)
			return output.Success(cx.inst.Name, map[string]any{"applications": apps, "count": len(apps)})
		})
	}})
	c.AddCommand(&cobra.Command{Use: "get <app>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return withGet(o, cmd, "/controller/rest/applications", nil, func(cx *ctx, v any) output.Envelope {
			app, ok := appd.FindByNameOrID(appd.ListAny(v), args[0])
			if !ok {
				return output.Failure("not_found", "application not found: "+args[0], "Run appd app list --json to see application names and ids.", 404)
			}
			return output.Success(cx.inst.Name, app)
		})
	}})
	return c
}

func tierCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "tier"}
	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		app, fail := requireApp(cmd)
		if fail != nil {
			return print(cmd, o, *fail)
		}
		return withGet(o, cmd, appPath(app)+"/tiers", nil, func(cx *ctx, v any) output.Envelope {
			tiers := appd.ListAny(v)
			return output.Success(cx.inst.Name, map[string]any{"application": app, "tiers": tiers, "count": len(tiers)})
		})
	}}
	addAppFlag(list)
	c.AddCommand(list)
	get := &cobra.Command{Use: "get <tier>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		app, fail := requireApp(cmd)
		if fail != nil {
			return print(cmd, o, *fail)
		}
		return withGet(o, cmd, appPath(app)+"/tiers/"+url.PathEscape(args[0]), nil, func(cx *ctx, v any) output.Envelope {
			tier, ok := appd.FirstItem(v)
			if !ok {
				return output.Failure("not_found", "tier not found: "+args[0], "Run appd tier list --app "+app+" --json to see tier names and ids.", 404)
			}
			return output.Success(cx.inst.Name, tier)
		})
	}}
	addAppFlag(get)
	c.AddCommand(get)
	return c
}

func nodeCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "node"}
	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		app, fail := requireApp(cmd)
		if fail != nil {
			return print(cmd, o, *fail)
		}
		tier := strings.TrimSpace(mustS(cmd, "tier"))
		path := appPath(app) + "/nodes"
		if tier != "" {
			path = appPath(app) + "/tiers/" + url.PathEscape(tier) + "/nodes"
		}
		return withGet(o, cmd, path, nil, func(cx *ctx, v any) output.Envelope {
			nodes := appd.ListAny(v)
			return output.Success(cx.inst.Name, map[string]any{"application": app, "tier": tier, "nodes": nodes, "count": len(nodes)})
		})
	}}
	addAppFlag(list)
	list.Flags().String("tier", "", "Restrict the listing to one tier (name or id).")
	c.AddCommand(list)
	get := &cobra.Command{Use: "get <node>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		app, fail := requireApp(cmd)
		if fail != nil {
			return print(cmd, o, *fail)
		}
		return withGet(o, cmd, appPath(app)+"/nodes/"+url.PathEscape(args[0]), nil, func(cx *ctx, v any) output.Envelope {
			node, ok := appd.FirstItem(v)
			if !ok {
				return output.Failure("not_found", "node not found: "+args[0], "Run appd node list --app "+app+" --json to see node names and ids.", 404)
			}
			return output.Success(cx.inst.Name, node)
		})
	}}
	addAppFlag(get)
	c.AddCommand(get)
	return c
}

func btCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "bt"}
	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		app, fail := requireApp(cmd)
		if fail != nil {
			return print(cmd, o, *fail)
		}
		tier := strings.TrimSpace(mustS(cmd, "tier"))
		return withGet(o, cmd, appPath(app)+"/business-transactions", nil, func(cx *ctx, v any) output.Envelope {
			bts := appd.FilterByTier(appd.ListAny(v), tier)
			return output.Success(cx.inst.Name, map[string]any{"application": app, "tier_filter": tier, "business_transactions": bts, "count": len(bts)})
		})
	}}
	addAppFlag(list)
	list.Flags().String("tier", "", "Keep only business transactions of this tier (tierName or tierId), filtered client-side.")
	c.AddCommand(list)
	return c
}

func backendCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "backend"}
	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		app, fail := requireApp(cmd)
		if fail != nil {
			return print(cmd, o, *fail)
		}
		return withGet(o, cmd, appPath(app)+"/backends", nil, func(cx *ctx, v any) output.Envelope {
			backends := appd.ListAny(v)
			return output.Success(cx.inst.Name, map[string]any{"application": app, "backends": backends, "count": len(backends)})
		})
	}}
	addAppFlag(list)
	c.AddCommand(list)
	return c
}

func metricCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "metric"}
	browse := &cobra.Command{Use: "browse", RunE: func(cmd *cobra.Command, args []string) error {
		app, fail := requireApp(cmd)
		if fail != nil {
			return print(cmd, o, *fail)
		}
		metricPath := strings.TrimSpace(mustS(cmd, "path"))
		q := map[string]string{}
		if metricPath != "" {
			q["metric-path"] = metricPath
		}
		return withGet(o, cmd, appPath(app)+"/metrics", q, func(cx *ctx, v any) output.Envelope {
			children := appd.ListAny(v)
			return output.Success(cx.inst.Name, map[string]any{"application": app, "metric_path": metricPath, "children": children, "count": len(children)})
		})
	}}
	addAppFlag(browse)
	browse.Flags().String("path", "", "Metric path to expand, for example \"Overall Application Performance\"; empty lists the root folders.")
	c.AddCommand(browse)

	get := &cobra.Command{Use: "get", RunE: func(cmd *cobra.Command, args []string) error {
		app, fail := requireApp(cmd)
		if fail != nil {
			return print(cmd, o, *fail)
		}
		metricPath := strings.TrimSpace(mustS(cmd, "path"))
		if metricPath == "" {
			return print(cmd, o, output.Failure("invalid_args", "--path is required", "Pass a metric path such as \"Overall Application Performance|Average Response Time (ms)\"; discover paths with appd metric browse.", 400))
		}
		return metricData(o, cmd, app, metricPath, "")
	}}
	addAppFlag(get)
	get.Flags().String("path", "", "Full metric path, segments separated by | (wildcards such as * are passed through).")
	addMetricDataFlags(get)
	c.AddCommand(get)

	preset := &cobra.Command{Use: "preset", RunE: func(cmd *cobra.Command, args []string) error {
		app, fail := requireApp(cmd)
		if fail != nil {
			return print(cmd, o, *fail)
		}
		name := strings.ToLower(strings.TrimSpace(mustS(cmd, "preset")))
		metricPath, err := appd.ExpandPreset(name, mustS(cmd, "tier"), mustS(cmd, "bt"), mustS(cmd, "node"))
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", err.Error(), "Presets: bt-response-time, bt-calls, bt-errors (need --tier and --bt), tier-cpu (needs --tier), node-heap (needs --tier and --node).", 400))
		}
		return metricData(o, cmd, app, metricPath, name)
	}}
	addAppFlag(preset)
	preset.Flags().String("preset", "", "Preset metric: "+strings.Join(appd.PresetNames, ", ")+".")
	preset.Flags().String("tier", "", "Tier name for the preset.")
	preset.Flags().String("bt", "", "Business transaction name for the bt-* presets.")
	preset.Flags().String("node", "", "Node name for the node-heap preset.")
	addMetricDataFlags(preset)
	c.AddCommand(preset)
	return c
}

func addMetricDataFlags(cmd *cobra.Command) {
	addTimeFlags(cmd, 60)
	cmd.Flags().Bool("rollup", false, "Return one value aggregated over the window instead of one value per minute.")
	cmd.Flags().String("api", "v2", "Controller metric endpoint: v2 (metric-data-v2, default) or v1 (metric-data).")
}

func metricData(o *Opts, cmd *cobra.Command, app, metricPath, preset string) error {
	tr, err := timeRangeFromFlags(cmd)
	if err != nil {
		return print(cmd, o, timeRangeFailure(err))
	}
	api := strings.ToLower(strings.TrimSpace(mustS(cmd, "api")))
	endpoint := ""
	switch api {
	case "", "v2":
		api, endpoint = "v2", "/metric-data-v2"
	case "v1":
		endpoint = "/metric-data"
	default:
		return print(cmd, o, output.Failure("invalid_args", "--api must be v2 or v1", "", 400))
	}
	rollup := mustB(cmd, "rollup")
	q := tr.Query()
	q["metric-path"] = metricPath
	q["rollup"] = strconv.FormatBool(rollup)
	return withGet(o, cmd, appPath(app)+endpoint, q, func(cx *ctx, v any) output.Envelope {
		metrics := appd.ListAny(v)
		data := map[string]any{"application": app, "metric_path": metricPath, "api": api, "rollup": rollup, "time_range": tr, "metrics": metrics, "count": len(metrics)}
		if preset != "" {
			data["preset"] = preset
		}
		return output.Success(cx.inst.Name, data)
	})
}

func snapshotCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "snapshot"}
	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		app, fail := requireApp(cmd)
		if fail != nil {
			return print(cmd, o, *fail)
		}
		tr, err := timeRangeFromFlags(cmd)
		if err != nil {
			return print(cmd, o, timeRangeFailure(err))
		}
		maxResults := mustI(cmd, "max-results")
		if maxResults <= 0 {
			return print(cmd, o, output.Failure("invalid_args", "--max-results must be a positive number", "", 400))
		}
		q := tr.Query()
		q["maximum-results"] = strconv.Itoa(maxResults)
		filters := map[string]any{}
		setCSV := func(flag, param string, upper bool) {
			v := strings.TrimSpace(mustS(cmd, flag))
			if v == "" {
				return
			}
			if upper {
				v = appd.NormalizeEnumCSV(v)
			} else {
				v = strings.Join(appd.SplitCSV(v), ",")
			}
			q[param] = v
			filters[strings.ReplaceAll(flag, "-", "_")] = v
		}
		setCSV("bt-ids", "business-transaction-ids", false)
		setCSV("tier-ids", "application-component-ids", false)
		setCSV("node-ids", "application-component-node-ids", false)
		setCSV("user-experience", "user-experience", true)
		for _, b := range []struct{ flag, param string }{
			{"errors-only", "error-occurred"},
			{"first-in-chain", "first-in-chain"},
			{"need-exit-calls", "need-exit-calls"},
			{"need-props", "need-props"},
		} {
			if mustB(cmd, b.flag) {
				q[b.param] = "true"
				filters[strings.ReplaceAll(b.flag, "-", "_")] = true
			}
		}
		return withGet(o, cmd, appPath(app)+"/request-snapshots", q, func(cx *ctx, v any) output.Envelope {
			snapshots := appd.TrimSnapshots(appd.ListAny(v))
			return output.Success(cx.inst.Name, map[string]any{
				"application":    app,
				"time_range":     tr,
				"filters":        filters,
				"max_results":    maxResults,
				"snapshots":      snapshots,
				"count_returned": len(snapshots),
				"truncated":      len(snapshots) >= maxResults,
			})
		})
	}}
	addAppFlag(list)
	addTimeFlags(list, 60)
	list.Flags().String("bt-ids", "", "Comma-separated business transaction ids (business-transaction-ids).")
	list.Flags().String("tier-ids", "", "Comma-separated tier ids (application-component-ids).")
	list.Flags().String("node-ids", "", "Comma-separated node ids (application-component-node-ids).")
	list.Flags().String("user-experience", "", "Comma-separated user experiences: NORMAL, SLOW, VERY_SLOW, STALL, ERROR.")
	list.Flags().Bool("errors-only", false, "Only snapshots in which an error occurred (error-occurred=true).")
	list.Flags().Bool("first-in-chain", false, "Only snapshots that start a call chain (first-in-chain=true).")
	list.Flags().Bool("need-exit-calls", false, "Include exit calls in each snapshot (need-exit-calls=true).")
	list.Flags().Bool("need-props", false, "Include snapshot properties such as HTTP parameters (need-props=true).")
	list.Flags().Int("max-results", 50, "Maximum snapshots to return (maximum-results).")
	c.AddCommand(list)

	get := &cobra.Command{Use: "get", RunE: func(cmd *cobra.Command, args []string) error {
		app, fail := requireApp(cmd)
		if fail != nil {
			return print(cmd, o, *fail)
		}
		guid := strings.TrimSpace(mustS(cmd, "guid"))
		if guid == "" {
			return print(cmd, o, output.Failure("invalid_args", "--guid is required", "Pass the requestGUID from appd snapshot list.", 400))
		}
		tr, err := timeRangeFromFlags(cmd)
		if err != nil {
			return print(cmd, o, timeRangeFailure(err))
		}
		q := tr.Query()
		q["guids"] = guid
		q["need-exit-calls"] = "true"
		q["need-props"] = "true"
		return withGet(o, cmd, appPath(app)+"/request-snapshots", q, func(cx *ctx, v any) output.Envelope {
			snapshot, ok := appd.FirstItem(v)
			if !ok {
				return output.Failure("not_found", "snapshot not found: "+guid, "Check the GUID and widen the time range (the default window is the last 14 days).", 404)
			}
			return output.Success(cx.inst.Name, map[string]any{
				"application":          app,
				"time_range":           tr,
				"snapshot":             snapshot,
				"call_graph_available": false,
				"note":                 "The call graph (drill-down) is not exposed by the public Controller REST API; open the snapshot in the Controller UI for it.",
			})
		})
	}}
	addAppFlag(get)
	get.Flags().String("guid", "", "Snapshot requestGUID.")
	addTimeFlags(get, defaultSnapshotGetMins)
	c.AddCommand(get)
	return c
}

func violationCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "violation"}
	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		app, fail := requireApp(cmd)
		if fail != nil {
			return print(cmd, o, *fail)
		}
		tr, err := timeRangeFromFlags(cmd)
		if err != nil {
			return print(cmd, o, timeRangeFailure(err))
		}
		return withGet(o, cmd, appPath(app)+"/problems/healthrule-violations", tr.Query(), func(cx *ctx, v any) output.Envelope {
			violations := appd.ListAny(v)
			return output.Success(cx.inst.Name, map[string]any{"application": app, "time_range": tr, "violations": violations, "count": len(violations)})
		})
	}}
	addAppFlag(list)
	addTimeFlags(list, 60)
	c.AddCommand(list)
	return c
}

func eventCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "event"}
	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		app, fail := requireApp(cmd)
		if fail != nil {
			return print(cmd, o, *fail)
		}
		eventTypes := appd.NormalizeEnumCSV(mustS(cmd, "event-types"))
		if eventTypes == "" {
			return print(cmd, o, output.Failure("invalid_args", "--event-types is required", "Pass a comma-separated list such as APPLICATION_ERROR,APPLICATION_DEPLOYMENT,DIAGNOSTIC_SESSION,CUSTOM,POLICY_OPEN_WARNING,POLICY_OPEN_CRITICAL.", 400))
		}
		severities := appd.NormalizeEnumCSV(mustS(cmd, "severities"))
		if severities == "" {
			severities = "ERROR,WARN,INFO"
		}
		tr, err := timeRangeFromFlags(cmd)
		if err != nil {
			return print(cmd, o, timeRangeFailure(err))
		}
		q := tr.Query()
		q["event-types"] = eventTypes
		q["severities"] = severities
		return withGet(o, cmd, appPath(app)+"/events", q, func(cx *ctx, v any) output.Envelope {
			events := appd.ListAny(v)
			return output.Success(cx.inst.Name, map[string]any{"application": app, "time_range": tr, "event_types": eventTypes, "severities": severities, "events": events, "count": len(events)})
		})
	}}
	addAppFlag(list)
	addTimeFlags(list, 60)
	list.Flags().String("event-types", "", "Comma-separated event types, for example APPLICATION_ERROR,APPLICATION_DEPLOYMENT,DIAGNOSTIC_SESSION,CUSTOM.")
	list.Flags().String("severities", "ERROR,WARN,INFO", "Comma-separated severities: ERROR, WARN, INFO.")
	c.AddCommand(list)
	return c
}

func apiCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "api"}
	get := &cobra.Command{Use: "get <path>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cx, err := loadCtx(o)
		if err != nil {
			return print(cmd, o, envelopeError(err, "config_error"))
		}
		path, q, err := cx.client.RESTPath(args[0])
		if err != nil {
			return print(cmd, o, envelopeError(err, "invalid_args"))
		}
		extra, err := parseKeyValue(mustSA(cmd, "query"))
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", err.Error(), "Pass --query key=value.", 400))
		}
		for k, v := range extra {
			q[k] = v
		}
		if o.DryRun {
			preview, err := cx.client.Preview(path, q)
			if err != nil {
				return print(cmd, o, envelopeError(err, "invalid_args"))
			}
			return print(cmd, o, output.Success(cx.inst.Name, preview))
		}
		v, err := cx.client.Get(path, q)
		if err != nil {
			return print(cmd, o, envelopeError(err, "server_error"))
		}
		return print(cmd, o, output.Success(cx.inst.Name, v))
	}}
	get.Flags().StringArray("query", nil, "Raw query parameter in key=value form; repeat for multiple values. output=JSON is added automatically.")
	c.AddCommand(get)
	return c
}

func parseKeyValue(items []string) (map[string]string, error) {
	out := map[string]string{}
	for _, item := range items {
		k, v, ok := strings.Cut(item, "=")
		if !ok || strings.TrimSpace(k) == "" {
			return nil, fmt.Errorf("invalid key=value parameter %q", item)
		}
		out[strings.TrimSpace(k)] = v
	}
	return out, nil
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
			"Run " + product + " commands --json, then " + product + " schema <command> --json before constructing a complex call.",
			"Use --instance when several AppDynamics controllers are configured; without it the default instance is used.",
			"Start with appd app list --json to learn application names and ids; every other command takes --app <name-or-id>.",
			"Triage order: app list, then bt list --app <app>, then snapshot list --app <app> --errors-only --duration-mins 60, then violation list --app <app>, then event list --app <app> --event-types APPLICATION_DEPLOYMENT to align incidents with deployments (compare eventTime with the Jenkins build timestamps).",
			"Every time-ranged command has an explicit window: --duration-mins N (default 60, before now), --start-time plus --end-time (epoch milliseconds or RFC3339), or --before-time/--after-time plus --duration-mins; the resolved window is echoed as data.time_range.",
			"Use metric preset for common metric paths (bt-response-time, bt-calls, bt-errors need --tier and --bt; tier-cpu needs --tier; node-heap needs --tier and --node); use metric browse --path to discover other paths and metric get --path for them.",
			"snapshot list output is trimmed to summary fields and capped by --max-results; focus it with --errors-only, --user-experience ERROR,VERY_SLOW,STALL, --bt-ids, --tier-ids, or --node-ids.",
			"snapshot get --guid returns the snapshot with exit calls and properties; the call graph (drill-down) is not available through the public Controller REST API, so point the user to the Controller UI for it.",
			"All appd commands are read-only; --dry-run previews the request path and query without contacting the Controller.",
			"Inspect error.code and error.hint before retrying; auth_failed usually means a wrong API client secret, a disabled API client, or a missing account.",
		}
		return print(cmd, o, output.Success("", map[string]any{"tips": tips, "commands": catalog.CommandsFromCobra(product, cmd.Root())}))
	}}
}

func versionCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "version", RunE: func(cmd *cobra.Command, args []string) error {
		return print(cmd, o, output.Success("", map[string]any{"version": version.Version, "commit": version.Commit, "date": version.Date}))
	}}
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
