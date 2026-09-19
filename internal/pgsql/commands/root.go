package commands

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/catalog"
	"engineering-flow-platform-tools/internal/clihelp"
	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/output"
	"engineering-flow-platform-tools/internal/pgsql"
	"engineering-flow-platform-tools/internal/version"
	"github.com/spf13/cobra"
)

const product = "pgsql"

type Opts struct {
	Instance, Config, Format   string
	JSON, Verbose, DryRun, Yes bool

	exec pgsql.Executor
}

// NewRoot builds the pgsql command tree backed by the real pgx executor.
func NewRoot() *cobra.Command {
	return NewRootWithExecutor(pgsql.NewExecutor())
}

// NewRootWithExecutor builds the command tree with a custom Executor so tests
// can run every command against pgsql.FakeExecutor.
func NewRootWithExecutor(exec pgsql.Executor) *cobra.Command {
	cobra.EnableCommandSorting = false
	o := &Opts{Format: "table", exec: exec}
	c := &cobra.Command{Use: product, SilenceErrors: true, SilenceUsage: true}
	c.PersistentFlags().StringVar(&o.Instance, "instance", "", "Configured pgsql instance name.")
	c.PersistentFlags().StringVar(&o.Config, "config", "", "Path to EFP config file.")
	c.PersistentFlags().BoolVar(&o.JSON, "json", false, "Print JSON envelope.")
	c.PersistentFlags().StringVar(&o.Format, "format", "table", "Output format: table|json|yaml.")
	c.PersistentFlags().BoolVar(&o.Verbose, "verbose", false, "Print non-secret diagnostics when available.")
	c.PersistentFlags().BoolVar(&o.DryRun, "dry-run", false, "Preview the guarded statement and connection target without connecting.")
	c.PersistentFlags().BoolVar(&o.Yes, "yes", false, "Confirm destructive or service-affecting operations.")
	c.AddCommand(instanceCmd(o), authCmd(o), queryCmd(o), explainCmd(o), schemaCmd(o), statCmd(o), dbCmd(o), commandsCmd(o), helpLLMCmd(o), versionCmd(o))
	clihelp.ApplyCatalogHelp(c, clihelp.ProductHelp{
		Product: product,
		Binary:  product,
		Short:   "Run read-only PostgreSQL queries and inspect schema and activity",
		Long: strings.TrimSpace(`pgsql is a terminal-invoked CLI for agents that need read-only, JSON-first access to PostgreSQL: bounded SELECT queries inside a READ ONLY transaction, schema description, and activity/lock/statistics views.

Every statement passes a guard (single read statement, no data-modifying CTEs, no side-effecting functions), runs inside BEGIN READ ONLY with a statement timeout, and is rolled back. The real guarantee is the read-only database role configured for the instance.

Configuration uses the shared EFP config from environment variables injected by managed runtimes (for example EFP_PGSQL_DEFAULT_INSTANCE, EFP_PGSQL_INSTANCES_0_HOST) or the config file, normally ~/.efp/config.yaml (local), under the pgsql node.`),
		Examples: []string{
			`pgsql auth test --json`,
			`pgsql schema tables --json`,
			`pgsql schema describe public.orders --json`,
			`pgsql query --sql "SELECT status, count(*) FROM orders GROUP BY status" --limit 50 --json`,
			`pgsql stat activity --state active --min-duration-sec 5 --json`,
			`pgsql help llm --json`,
		},
		Instructions: "copy cmd/pgsql/pgsql-cli.instructions.md to ~/.copilot/instructions/pgsql-cli.instructions.md.",
		Groups: map[string]string{
			"instance": "Manage configured PostgreSQL instances.",
			"auth":     "Manage the PostgreSQL password stored in the EFP config and test the connection.",
			"schema":   "Describe a pgsql command, or list and describe database relations.",
			"stat":     "Inspect activity, locks, slow statements, replication, and table statistics.",
			"db":       "Inspect database-level facts such as size.",
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

// session is one resolved instance plus the connection parameters derived
// from it and the executor that will run statements against it.
type session struct {
	cfg  config.RootConfig
	inst config.PgsqlInstanceConfig
	conn pgsql.ConnParams
	exec pgsql.Executor
}

// resolveInstance mirrors internal/instance.Resolve without URL routing:
// explicit name, then default_instance, then the only instance.
func resolveInstance(p config.PgsqlConfig, explicit string) (config.PgsqlInstanceConfig, error) {
	if len(p.Instances) == 0 {
		return config.PgsqlInstanceConfig{}, &pgsql.Error{Code: "no_instance_configured", Message: "no pgsql instance is configured", Hint: "Run pgsql instance add <name> --host ... --database ... --username ... --password-stdin, or set EFP_PGSQL_INSTANCES_0_HOST and related variables.", Status: 400}
	}
	names := make([]string, 0, len(p.Instances))
	for _, in := range p.Instances {
		names = append(names, in.Name)
	}
	if explicit != "" {
		for _, in := range p.Instances {
			if in.Name == explicit {
				return in, nil
			}
		}
		return config.PgsqlInstanceConfig{}, &pgsql.Error{Code: "instance_required", Message: "pgsql instance " + explicit + " is not configured", Hint: "Configured instances: " + strings.Join(names, ", ") + ". Run pgsql instance list --json.", Status: 400}
	}
	if p.DefaultInstance != "" {
		for _, in := range p.Instances {
			if in.Name == p.DefaultInstance {
				return in, nil
			}
		}
	}
	if len(p.Instances) > 1 {
		return config.PgsqlInstanceConfig{}, &pgsql.Error{Code: "instance_required", Message: "several pgsql instances are configured and none is the default", Hint: "Pass --instance <name> (one of " + strings.Join(names, ", ") + ") or run pgsql instance default <name>.", Status: 400}
	}
	return p.Instances[0], nil
}

func connParams(inst config.PgsqlInstanceConfig) pgsql.ConnParams {
	return pgsql.ConnParams{
		Host:             inst.Host,
		Port:             inst.EffectivePort(),
		Database:         inst.Database,
		Username:         inst.Username,
		Password:         inst.Password,
		SSLMode:          inst.EffectiveSSLMode(),
		CACert:           inst.CACert,
		StatementTimeout: time.Duration(inst.EffectiveStatementTimeoutSeconds()) * time.Second,
	}
}

func loadSession(o *Opts) (*session, error) {
	cfg, err := loadCfg(o)
	if err != nil {
		return nil, err
	}
	inst, err := resolveInstance(cfg.Pgsql, o.Instance)
	if err != nil {
		return nil, err
	}
	if !inst.IsEnabled() {
		return nil, &pgsql.Error{Code: "instance_disabled", Message: "pgsql instance " + inst.Name + " is disabled", Hint: "Enable it with pgsql instance update " + inst.Name + " --enabled=true or select another instance with --instance.", Status: 400}
	}
	var missing []string
	if inst.Host == "" {
		missing = append(missing, "host")
	}
	if inst.Database == "" {
		missing = append(missing, "database")
	}
	if inst.Username == "" {
		missing = append(missing, "username")
	}
	if len(missing) > 0 {
		return nil, &pgsql.Error{Code: pgsql.CodeConfigError, Message: "pgsql instance " + inst.Name + " is missing " + strings.Join(missing, ", "), Hint: "Run pgsql instance update " + inst.Name + " --host ... --database ... --username ... --json.", Status: 400}
	}
	return &session{cfg: cfg, inst: inst, conn: connParams(inst), exec: o.exec}, nil
}

// query runs one guarded statement through the session's executor.
func (s *session) query(sql string, args []any, limit int) (*pgsql.Output, error) {
	return pgsql.Execute(context.Background(), s.exec, s.conn, sql, args, limit)
}

// target describes the connection without secrets, for dry-run and verbose output.
func (s *session) target() map[string]any {
	return map[string]any{
		"instance":                  s.inst.Name,
		"host":                      s.conn.Host,
		"port":                      s.conn.Port,
		"database":                  s.conn.Database,
		"username":                  s.conn.Username,
		"sslmode":                   s.conn.SSLMode,
		"ca_cert_configured":        s.conn.CACert != "",
		"statement_timeout_seconds": int(s.conn.EffectiveTimeout().Seconds()),
		"max_rows":                  s.inst.EffectiveMaxRows(),
	}
}

type dryStatement struct {
	SQL    string `json:"sql"`
	Params []any  `json:"params"`
}

func dryRunData(s *session, statements ...dryStatement) map[string]any {
	for i := range statements {
		if statements[i].Params == nil {
			statements[i].Params = []any{}
		}
	}
	data := map[string]any{"dry_run": true, "statements": statements, "connection": s.target()}
	if len(statements) == 1 {
		data["sql"] = statements[0].SQL
		data["params"] = statements[0].Params
	}
	return data
}

// runPreset resolves the session, honors --dry-run, executes one preset, and
// prints the shaped output plus any extra keys.
func runPreset(o *Opts, cmd *cobra.Command, sql string, args []any, limit int, extra func(*session, *pgsql.Output) map[string]any) error {
	s, err := loadSession(o)
	if err != nil {
		return print(cmd, o, envelopeError(err))
	}
	if o.DryRun {
		return print(cmd, o, output.Success(s.inst.Name, dryRunData(s, dryStatement{SQL: sql, Params: args})))
	}
	out, err := s.query(sql, args, limit)
	if err != nil {
		return print(cmd, o, envelopeError(err))
	}
	data := out.Data()
	if extra != nil {
		for k, v := range extra(s, out) {
			data[k] = v
		}
	}
	if o.Verbose {
		data["connection"] = s.target()
	}
	return print(cmd, o, output.Success(s.inst.Name, data))
}

func envelopeError(err error) output.Envelope {
	var pe *pgsql.Error
	if errors.As(err, &pe) {
		status := pe.Status
		if status == 0 {
			status = 500
		}
		return output.Failure(pe.Code, pe.Message, pe.Hint, status)
	}
	if errors.Is(err, config.ErrEnvManaged) {
		return output.Failure("config_env_managed", err.Error(), "Config comes from EFP_PGSQL_* environment variables; pass --config <path> to write a file instead.", 400)
	}
	if os.IsNotExist(err) {
		return output.Failure("config_missing", err.Error(), "Create ~/.efp/config.yaml with a pgsql node, pass --config <path>, or set EFP_PGSQL_INSTANCES_0_HOST and related variables.", 404)
	}
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "no_instance_configured", "instance_required", "invalid_args", "not_found", "not_supported", "auth_failed", "permission_denied", "network_error", "server_error":
		return output.Failure(msg, msg, "", 400)
	}
	return output.Failure("server_error", msg, "", 500)
}

func toArgs(values []string) []any {
	out := make([]any, 0, len(values))
	for _, v := range values {
		out = append(out, v)
	}
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

func mustSA(cmd *cobra.Command, name string) []string {
	v, _ := cmd.Flags().GetStringArray(name)
	return v
}

func itoa(n int) string { return strconv.Itoa(n) }

func commandsCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "commands", RunE: func(cmd *cobra.Command, args []string) error {
		return print(cmd, o, output.Success("", map[string]any{"commands": catalog.CommandsFromCobra(product, cmd.Root())}))
	}}
}

func describeCommand(o *Opts, cmd *cobra.Command, name string) error {
	schema, ok := catalog.SchemaFromCobra(product, name, cmd.Root())
	if !ok {
		return print(cmd, o, output.Failure("not_found", "command not found", "Run "+product+" commands --json to list command names.", 404))
	}
	return print(cmd, o, output.Success("", schema))
}

func helpLLMCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "help llm", RunE: func(cmd *cobra.Command, args []string) error {
		tips := []string{
			"For agents, --json is the default way to use every " + product + " command and subcommand.",
			"Run " + product + " commands --json, then " + product + " schema <command> --json before constructing a complex call.",
			"Use --instance when several instances are configured; without it the default instance is used. pgsql auth test --json proves the connection and that the session is READ ONLY.",
			"Start with pgsql schema tables --json and pgsql schema describe <table> --json; never guess table or column names.",
			"Always pass --limit; rows are capped by the instance max_rows, cells by --max-cell-chars, and pgsql query --output result.csv writes a full extract to disk instead of the envelope.",
			"Aggregate (count, sum, GROUP BY) and filter with WHERE instead of SELECT *; use stat activity for blocking chains (blocked_by), stat locks --blocked-only for lock waits, and stat slow for pg_stat_statements.",
			"Results may contain PII: summarize them, and do not paste raw rows into reports, tickets, or other systems.",
			"Every statement runs inside a READ ONLY transaction with a statement timeout and is rolled back; the guard rejects writes, multiple statements, data-modifying CTEs, SELECT INTO, and side-effecting functions such as pg_terminate_backend, pg_sleep, pg_read_file, dblink, and set_config.",
			"The real guarantee is the read-only database role configured for the instance; the guard and READ ONLY transaction are defense in depth, not a substitute.",
			"Use --dry-run on query and explain to see the guarded statement and connection target without connecting.",
			"Inspect error.code and error.hint before retrying: read_only_violation means rewrite as a SELECT, query_timeout means narrow the query, permission_denied means the role lacks SELECT on that relation, invalid_args carries the server message for bad names or syntax.",
		}
		return print(cmd, o, output.Success("", map[string]any{"tips": tips, "commands": catalog.CommandsFromCobra(product, cmd.Root())}))
	}}
}

func versionCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "version", RunE: func(cmd *cobra.Command, args []string) error {
		return print(cmd, o, output.Success("", map[string]any{"version": version.Version, "commit": version.Commit, "date": version.Date}))
	}}
}
