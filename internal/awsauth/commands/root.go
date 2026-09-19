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

const (
	adfsAssumeCommand     = "adfs-assume"
	saml2awsCommand       = "saml2aws"
	awsCLICommand         = "aws"
	kubectlCommand        = "kubectl"
	envAdapterStateDir    = "EFP_ADAPTER_STATE_DIR"
	defaultAWSAuthProfile = "saml"
)

// secretEnvKeys are never inherited by provider or AWS CLI child processes;
// the selected provider receives the password through exactly one of them.
var secretEnvKeys = []string{"AD_PASS", "SAML2AWS_PASSWORD", "password"}

var redactedSecretPlaceholders = map[string]struct{}{
	"***redacted***": {},
	"[redacted]":     {},
	"redacted":       {},
}

type Opts struct {
	Config, Format string
	JSON, Verbose  bool
	DryRun         bool
	runner         commandRunner
}

type commandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// commandRunner executes the external binaries aws-auth orchestrates
// (adfs-assume, saml2aws, aws, kubectl). Tests inject a fake through
// NewRootWithRunner so no real binary is needed.
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
	o := &Opts{Format: "table", runner: r}
	c := &cobra.Command{Use: "aws-auth", SilenceErrors: true, SilenceUsage: true}
	c.PersistentFlags().StringVar(&o.Config, "config", "", "Path to EFP config file.")
	c.PersistentFlags().BoolVar(&o.JSON, "json", false, "Print JSON envelope.")
	c.PersistentFlags().StringVar(&o.Format, "format", "table", "Output format: table|json|yaml.")
	c.PersistentFlags().BoolVar(&o.Verbose, "verbose", false, "Print non-secret diagnostics when available.")
	c.PersistentFlags().BoolVar(&o.DryRun, "dry-run", false, "Preview the provider or AWS CLI commands without running them.")
	c.AddCommand(loginCmd(o), accountCmd(o), statusCmd(o), eksCmd(o), authCmd(o), commandsCmd(o), schemaCmd(o), helpLLMCmd(o), versionCmd(o))
	clihelp.ApplyCatalogHelp(c, clihelp.ProductHelp{
		Product: "aws-auth",
		Binary:  "aws-auth",
		Short:   "Authorize AWS credentials for configured accounts and prepare EKS access",
		Long: strings.TrimSpace(`aws-auth is a terminal-invoked CLI for agents and runtimes that need AWS credentials from the shared EFP config.

Configuration uses the shared EFP config from environment variables injected by managed runtimes (for example EFP_AWS_DOMAIN, EFP_AWS_USERNAME, EFP_AWS_PROVIDER, EFP_AWS_ACCOUNTS_0_NAME) or the config file, normally ~/.efp/config.yaml (local), under the aws node. The node holds the directory credentials, the provider that exchanges them for AWS credentials (adfs-assume, saml2aws, or assume-role), and an account matrix: name, account id, role, regions.

login writes each account's credentials to its own AWS CLI profile (the account name by default) so agents can query several accounts side by side with aws --profile <name>. eks kubeconfig turns an account and cluster into a kubectl context named <account>/<cluster>. status reports which profiles exist and whether their session expired.`),
		Examples: []string{
			`aws-auth account list --json`,
			`aws-auth login --account cps-dev --json`,
			`aws-auth login --all --json`,
			`aws-auth login --account 123456 --role ADFS-ReadOnly --profile saml --json`,
			`aws-auth status --verify --json`,
			`aws-auth eks kubeconfig --account cps-dev --cluster cps-dev-eks --json`,
			`printf '%s\n' "$AWS_AD_PASSWORD" | aws-auth auth login --domain HBEU --username GB-SVC-XXX-XXX --password-stdin --json`,
			`aws-auth help llm --json`,
		},
		Instructions: "copy cmd/aws-auth/aws-auth-cli.instructions.md to ~/.copilot/instructions/aws-auth-cli.instructions.md.",
		Groups: map[string]string{
			"login":   "Authorize AWS credentials.",
			"account": "Inspect the configured account matrix.",
			"status":  "Inspect AWS CLI profiles and session expiry.",
			"eks":     "Discover EKS clusters and write kubectl contexts.",
			"auth":    "Manage AWS authorization config.",
		},
	})
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
		// Only the directory credentials are replaced; provider, account matrix
		// and defaults already stored under the aws node are preserved.
		enabled := true
		cfg.AWS.Enabled = &enabled
		cfg.AWS.Domain = domain
		cfg.AWS.Username = username
		cfg.AWS.Password = password
		if config.EnvManaged(o.Config) {
			return print(cmd, o, output.Failure("config_env_managed", "config comes from environment variables; pass --config to write a file", "", 400))
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
			"configured":    awsAuthConfigured(cfg.AWS),
			"provider":      cfg.AWS.EffectiveProvider(),
			"account_count": len(cfg.AWS.EnabledAccounts()),
			"config_path":   path,
			"aws":           aws,
		}))
	}}
	c.AddCommand(status)
	return c
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
	if config.EnvManaged(flagPath) {
		cfg, source, err := config.LoadShared("")
		if err != nil {
			return source, config.RootConfig{}, err
		}
		return source, cfg, nil
	}
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

// awsAuthConfigured reports whether the aws node can authorize something: the
// ADFS providers need directory credentials, assume-role only needs accounts.
func awsAuthConfigured(aws config.AWSConfig) bool {
	if aws.Enabled != nil && !*aws.Enabled {
		return false
	}
	if aws.EffectiveProvider() == config.AWSProviderAssumeRole {
		return len(aws.EnabledAccounts()) > 0
	}
	return singleLine(aws.Domain) != "" && singleLine(aws.Username) != "" && cleanSecret(aws.Password) != ""
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

// stripSecretEnv removes every secret-carrying variable so child processes
// only ever see the one the caller adds back through withSecretEnv.
func stripSecretEnv(env []string) []string {
	out := make([]string, 0, len(env)+1)
	for _, item := range env {
		key, _, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		secret := false
		for _, candidate := range secretEnvKeys {
			if key == candidate {
				secret = true
				break
			}
		}
		if secret {
			continue
		}
		out = append(out, item)
	}
	return out
}

func withSecretEnv(env []string, key, value string) []string {
	return append(stripSecretEnv(env), key+"="+value)
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func truncateText(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
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
			"Run aws-auth account list --json first to see the configured account matrix (name, account id, role, regions).",
			"Use aws-auth login --account <name> --json to authorize one configured account; its credentials land in the AWS CLI profile named after the account, so pass --profile <name> (or AWS_PROFILE=<name>) to aws and kubectl afterwards.",
			"Use aws-auth login --all --json to authorize every configured account; partial=true means some accounts failed and results[] says which.",
			"Use aws-auth login --account <account-id> --role <role-name> --json for an account outside the matrix; it writes the saml profile.",
			"Use aws-auth status --json to see which profiles hold credentials and whether their session expired; re-run login for that account when aws reports ExpiredToken.",
			"Use aws-auth eks kubeconfig --account <name> --cluster <cluster> --json, then kubectl --context <name>/<cluster> for read-only cluster inspection.",
			"Use aws-auth auth login --password-stdin --json to store directory credentials without putting the password in shell history.",
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
