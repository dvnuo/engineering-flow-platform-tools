package commands

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"engineering-flow-platform-tools/internal/catalog"
	"engineering-flow-platform-tools/internal/clihelp"
	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/output"
	"engineering-flow-platform-tools/internal/version"
	"github.com/spf13/cobra"
)

const adfsAssumeCommand = "adfs-assume"
const envAdapterStateDir = "EFP_ADAPTER_STATE_DIR"

var redactedSecretPlaceholders = map[string]struct{}{
	"***redacted***": {},
	"[redacted]":     {},
	"redacted":       {},
}

type Opts struct {
	Config, Format   string
	JSON, Verbose    bool
	DryRun           bool
	adfsAssumeRunner commandRunner
}

type commandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type commandRunner interface {
	Run(ctx context.Context, command string, args []string, env []string) (commandResult, error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, command string, args []string, env []string) (commandResult, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = env
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := commandResult{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: 0}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
			return result, nil
		}
		result.ExitCode = 1
		return result, err
	}
	return result, nil
}

func NewRoot() *cobra.Command {
	return NewRootWithRunner(execRunner{})
}

func NewRootWithRunner(r commandRunner) *cobra.Command {
	cobra.EnableCommandSorting = false
	o := &Opts{Format: "table", adfsAssumeRunner: r}
	c := &cobra.Command{Use: "aws-auth", SilenceErrors: true, SilenceUsage: true}
	c.PersistentFlags().StringVar(&o.Config, "config", "", "Path to EFP config file.")
	c.PersistentFlags().BoolVar(&o.JSON, "json", false, "Print JSON envelope.")
	c.PersistentFlags().StringVar(&o.Format, "format", "table", "Output format: table|json|yaml.")
	c.PersistentFlags().BoolVar(&o.Verbose, "verbose", false, "Print non-secret diagnostics when available.")
	c.PersistentFlags().BoolVar(&o.DryRun, "dry-run", false, "Preview authorization without running the provider command.")
	c.AddCommand(loginCmd(o), authCmd(o), commandsCmd(o), schemaCmd(o), helpLLMCmd(o), versionCmd(o))
	clihelp.ApplyCatalogHelp(c, clihelp.ProductHelp{
		Product: "aws-auth",
		Binary:  "aws-auth",
		Short:   "Authorize AWS credentials from the shared EFP config",
		Long: strings.TrimSpace(`aws-auth is a terminal-invoked CLI for agents and runtimes that need AWS authorization from the shared EFP config file.

Configuration uses the shared EFP config file, normally ~/.efp/config.yaml, under the aws node. The auth login command stores the configured domain, username, and password. The login command reads that config and invokes the installed authorization provider with the account and role supplied for that login.`),
		Examples: []string{
			`printf '%s\n' "$AWS_AD_PASSWORD" | aws-auth auth login --domain HBEU --username GB-SVC-XXX-XXX --password-stdin --json`,
			`aws-auth login --account 123456 --role ADFS-ReadOnly --profile default --json`,
			`aws-auth login`,
			`aws-auth --config ~/.efp/config.yaml login --account 123456 --role ADFS-ReadOnly --profile default --json`,
			`aws-auth commands --json`,
			`aws-auth schema login --json`,
			`aws-auth help llm --json`,
		},
		Instructions: "copy cmd/aws-auth/aws-auth-cli.instructions.md to ~/.copilot/instructions/aws-auth-cli.instructions.md.",
		Groups: map[string]string{
			"login": "Authorize AWS credentials.",
			"auth":  "Manage AWS authorization config.",
		},
	})
	return c
}

func loginCmd(o *Opts) *cobra.Command {
	var account string
	var role string
	var profile string
	c := &cobra.Command{
		Use:   "login",
		Short: "Authorize AWS credentials from saved auth config.",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, cfg, err := loadAWSConfigForRead(o.Config)
			if err != nil {
				return print(cmd, o, output.Failure("config_error", output.RedactString(err.Error()), "Check EFP_CONFIG or pass --config.", 400))
			}
			login, failure := buildLogin(cmd, cfg.AWS, loginOptions{Account: account, Role: role, Profile: profile, Prompt: !o.JSON})
			if failure != nil {
				return print(cmd, o, *failure)
			}
			if o.DryRun {
				return print(cmd, o, output.Success("", map[string]any{
					"authenticated": false,
					"dry_run":       true,
					"command":       formatCommand(login.command, login.args),
				}))
			}
			result, err := o.adfsAssumeRunner.Run(cmd.Context(), login.command, login.args, login.env)
			if err != nil {
				return print(cmd, o, output.Failure(
					"execution_failed",
					redactWithSecrets(err.Error(), login.password),
					"Ensure adfs-assume is installed and available on PATH.",
					500,
				))
			}
			if result.ExitCode != 0 {
				message := strings.TrimSpace(result.Stderr)
				if message == "" {
					message = strings.TrimSpace(result.Stdout)
				}
				if message == "" {
					message = fmt.Sprintf("authorization provider exited with %d", result.ExitCode)
				}
				return print(cmd, o, output.Failure(
					"auth_failed",
					redactWithSecrets(message, login.password),
					"Verify the configured AWS domain, username, password, and the supplied account and role.",
					401,
				))
			}
			data := map[string]any{
				"authenticated": true,
				"command":       formatCommand(login.command, login.args),
			}
			if o.Verbose {
				data["provider"] = adfsAssumeCommand
				data["config_path"] = path
			}
			return print(cmd, o, output.Success("", data))
		},
	}
	c.Flags().StringVar(&account, "account", "", "AWS account id to pass to adfs-assume.")
	c.Flags().StringVar(&role, "role", "", "AWS role name to pass to adfs-assume.")
	c.Flags().StringVar(&profile, "profile", "default", "AWS credentials profile name to create with adfs-assume.")
	return c
}

func authCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "auth"}
	login := &cobra.Command{Use: "login", RunE: func(cmd *cobra.Command, args []string) error {
		path, err := resolveAWSConfigPath(o.Config)
		if err != nil {
			return print(cmd, o, output.Failure("config_error", output.RedactString(err.Error()), "Check EFP_CONFIG or pass --config.", 400))
		}
		cfg, err := loadConfigForWrite(path)
		if err != nil {
			return print(cmd, o, output.Failure("config_error", output.RedactString(err.Error()), "Check EFP_CONFIG or pass --config.", 400))
		}
		domain := singleLine(mustS(cmd, "domain"))
		username := singleLine(mustS(cmd, "username"))
		password, err := passwordFromFlags(cmd)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", output.RedactString(err.Error()), "Pipe the AWS password to --password-stdin.", 400))
		}
		if domain == "" || username == "" || password == "" {
			return print(cmd, o, output.Failure("invalid_args", "domain, username, and password are required", "Use aws-auth auth login --domain HBEU --username GB-SVC-XXX-XXX --password-stdin --json.", 400))
		}
		if o.DryRun {
			return print(cmd, o, output.Success("", map[string]any{
				"configured":       false,
				"dry_run":          true,
				"config_path":      path,
				"domain":           domain,
				"username":         username,
				"password_present": true,
			}))
		}
		enabled := true
		cfg.AWS = config.AWSConfig{
			Enabled:  &enabled,
			Domain:   domain,
			Username: username,
			Password: password,
		}
		if err := config.Save(path, cfg); err != nil {
			return print(cmd, o, output.Failure("config_error", output.RedactString(err.Error()), "", 500))
		}
		return print(cmd, o, output.Success("", map[string]any{
			"configured":       true,
			"config_path":      path,
			"domain":           domain,
			"username":         username,
			"password_present": true,
		}))
	}}
	login.Flags().String("domain", "", "ADFS domain, for example HBEU.")
	login.Flags().String("username", "", "ADFS username.")
	login.Flags().Bool("password-stdin", false, "Read the ADFS password from stdin.")
	c.AddCommand(login)

	status := &cobra.Command{Use: "status", RunE: func(cmd *cobra.Command, args []string) error {
		path, cfg, err := loadAWSConfigForRead(o.Config)
		if err != nil {
			return print(cmd, o, output.Failure("config_missing", output.RedactString(err.Error()), "Run aws-auth auth login --json first.", 404))
		}
		aws := config.RedactAWS(cfg.AWS)
		return print(cmd, o, output.Success("", map[string]any{
			"configured":  bool(aws.Enabled == nil || *aws.Enabled) && singleLine(cfg.AWS.Domain) != "" && singleLine(cfg.AWS.Username) != "" && cleanSecret(cfg.AWS.Password) != "",
			"config_path": path,
			"aws":         aws,
		}))
	}}
	c.AddCommand(status)
	return c
}

type loginSpec struct {
	command  string
	args     []string
	env      []string
	password string
}

type loginOptions struct {
	Account string
	Role    string
	Profile string
	Prompt  bool
}

func buildLogin(cmd *cobra.Command, aws config.AWSConfig, opts loginOptions) (loginSpec, *output.Envelope) {
	if aws.Enabled != nil && !*aws.Enabled {
		failure := output.Failure("config_missing", "AWS authorization is not configured.", "Set AWS domain, username, and password with aws-auth auth login.", 400)
		return loginSpec{}, &failure
	}
	domain := singleLine(aws.Domain)
	username := singleLine(aws.Username)
	password := cleanSecret(aws.Password)
	if domain == "" || username == "" || password == "" {
		failure := output.Failure("config_missing", "AWS authorization is not configured.", "Set AWS domain, username, and password with aws-auth auth login.", 400)
		return loginSpec{}, &failure
	}
	account := singleLine(opts.Account)
	role := singleLine(opts.Role)
	if opts.Prompt {
		var err error
		account, role, err = promptAccountRole(cmd, account, role)
		if err != nil {
			failure := output.Failure("invalid_args", output.RedactString(err.Error()), "Pass --account and --role when running aws-auth login.", 400)
			return loginSpec{}, &failure
		}
	}
	if account == "" || role == "" {
		failure := output.Failure("invalid_args", "account and role are required for AWS login.", "Pass --account and --role when running aws-auth login.", 400)
		return loginSpec{}, &failure
	}
	profile := singleLine(opts.Profile)
	if profile == "" {
		profile = "default"
	}
	loginArgs := []string{"--domain", domain, "--username", username, "--role", role, "--account", account, "--profile", profile, "--no-warning", "--display-token", "--jenkins"}
	return loginSpec{
		command:  adfsAssumeCommand,
		args:     loginArgs,
		env:      withADPass(os.Environ(), password),
		password: password,
	}, nil
}

func resolveAWSConfigPath(flagPath string) (string, error) {
	if flagPath != "" {
		return flagPath, nil
	}
	if p := strings.TrimSpace(os.Getenv(config.EnvConfigPath)); p != "" {
		return p, nil
	}
	return config.DefaultPath()
}

func loadAWSConfigForRead(flagPath string) (string, config.RootConfig, error) {
	path, err := resolveAWSConfigPath(flagPath)
	if err != nil {
		return "", config.RootConfig{}, err
	}
	candidates := []string{path}
	if flagPath == "" && strings.TrimSpace(os.Getenv(config.EnvConfigPath)) == "" {
		if fallback := adapterStateEFPConfigPath(); fallback != "" && fallback != path {
			candidates = append(candidates, fallback)
		}
	}

	var firstErr error
	var firstLoadedPath string
	var firstLoadedConfig config.RootConfig
	for _, candidate := range candidates {
		cfg, err := config.Load(candidate)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if firstLoadedPath == "" {
			firstLoadedPath = candidate
			firstLoadedConfig = cfg
		}
		if awsAuthConfigured(cfg.AWS) {
			return candidate, cfg, nil
		}
	}
	if firstLoadedPath != "" {
		return firstLoadedPath, firstLoadedConfig, nil
	}
	if firstErr == nil {
		firstErr = os.ErrNotExist
	}
	return path, config.RootConfig{}, firstErr
}

func adapterStateEFPConfigPath() string {
	stateDir := strings.TrimSpace(os.Getenv(envAdapterStateDir))
	if stateDir == "" {
		return ""
	}
	return filepath.Join(stateDir, "efp", "config.yaml")
}

func awsAuthConfigured(aws config.AWSConfig) bool {
	return (aws.Enabled == nil || *aws.Enabled) && singleLine(aws.Domain) != "" && singleLine(aws.Username) != "" && cleanSecret(aws.Password) != ""
}

func loadConfigForWrite(path string) (config.RootConfig, error) {
	cfg, err := config.Load(path)
	if err == nil {
		return cfg, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return config.RootConfig{Version: 1}, nil
	}
	return cfg, err
}

func passwordFromFlags(cmd *cobra.Command) (string, error) {
	if !mustB(cmd, "password-stdin") {
		return "", errors.New("missing --password-stdin")
	}
	secret, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return "", fmt.Errorf("failed to read --password-stdin: %w", err)
	}
	password := cleanSecret(string(secret))
	if password == "" {
		return "", errors.New("password is empty")
	}
	return password, nil
}

func promptAccountRole(cmd *cobra.Command, account, role string) (string, string, error) {
	reader := bufio.NewReader(cmd.InOrStdin())
	var err error
	if account == "" {
		account, err = promptLine(cmd, reader, "AWS account")
		if err != nil {
			return "", "", err
		}
	}
	if role == "" {
		role, err = promptLine(cmd, reader, "AWS role")
		if err != nil {
			return "", "", err
		}
	}
	return account, role, nil
}

func promptLine(cmd *cobra.Command, reader *bufio.Reader, label string) (string, error) {
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s: ", label)
	value, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	value = singleLine(value)
	if value == "" {
		return "", fmt.Errorf("%s is required", strings.ToLower(label))
	}
	return value, nil
}

func withADPass(env []string, password string) []string {
	out := make([]string, 0, len(env)+1)
	for _, item := range env {
		key, _, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		switch key {
		case "AD_PASS", "password":
			continue
		default:
			out = append(out, item)
		}
	}
	return append(out, "AD_PASS="+password)
}

func cleanSecret(value string) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return ""
	}
	if _, ok := redactedSecretPlaceholders[strings.ToLower(text)]; ok {
		return ""
	}
	return text
}

func singleLine(value string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(strings.ReplaceAll(value, "\x00", ""), "\r", " ")), " ")
}

func mustS(cmd *cobra.Command, name string) string {
	v, _ := cmd.Flags().GetString(name)
	return v
}

func mustB(cmd *cobra.Command, name string) bool {
	v, _ := cmd.Flags().GetBool(name)
	return v
}

func formatCommand(command string, args []string) string {
	parts := append([]string{command}, args...)
	if runtime.GOOS == "windows" {
		return strings.Join(parts, " ")
	}
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || strings.ContainsAny(part, " \t\n'\"\\$") {
			out = append(out, "'"+strings.ReplaceAll(part, "'", `'\''`)+"'")
			continue
		}
		out = append(out, part)
	}
	return strings.Join(out, " ")
}

func redactWithSecrets(value string, secrets ...string) string {
	text := value
	for _, secret := range secrets {
		if secret != "" {
			text = strings.ReplaceAll(text, secret, output.Redacted)
		}
	}
	return output.RedactString(text)
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

func commandsCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "commands", RunE: func(cmd *cobra.Command, args []string) error {
		return print(cmd, o, output.Success("", map[string]any{"commands": catalog.CommandsFromCobra("aws-auth", cmd.Root())}))
	}}
}

func schemaCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "schema <command>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		schema, ok := catalog.SchemaFromCobra("aws-auth", args[0], cmd.Root())
		if !ok {
			return print(cmd, o, output.Failure("not_found", "command not found", "Run aws-auth commands --json to list command names.", 404))
		}
		return print(cmd, o, output.Success("", schema))
	}}
}

func helpLLMCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "help llm", RunE: func(cmd *cobra.Command, args []string) error {
		tips := []string{
			"For agents, --json is the default way to use every aws-auth command and subcommand.",
			"Use aws-auth auth login --password-stdin --json to store AWS auth config without putting the password in shell history.",
			"Use aws-auth login --account <account-id> --role <role-name> --profile default --json to authorize default AWS credentials from the shared EFP config.",
			"Use --config or EFP_CONFIG when the caller manages an isolated config file.",
			"Inspect error.code and error.hint before retrying.",
		}
		return print(cmd, o, output.Success("", map[string]any{"tips": tips, "commands": catalog.CommandsFromCobra("aws-auth", cmd.Root())}))
	}}
}

func versionCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "version", RunE: func(cmd *cobra.Command, args []string) error {
		return print(cmd, o, output.Success("", map[string]any{"version": version.Version, "commit": version.Commit, "date": version.Date}))
	}}
}
