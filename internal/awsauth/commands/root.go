package commands

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
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
	c.AddCommand(loginCmd(o), commandsCmd(o), schemaCmd(o), helpLLMCmd(o), versionCmd(o))
	clihelp.ApplyCatalogHelp(c, clihelp.ProductHelp{
		Product: "aws-auth",
		Binary:  "aws-auth",
		Short:   "Authorize AWS credentials from the shared EFP config",
		Long: strings.TrimSpace(`aws-auth is a terminal-invoked CLI for agents and runtimes that need AWS authorization from the shared EFP config file.

Configuration uses the shared EFP config file, normally ~/.efp/config.yaml, under the aws node. The login command reads the configured domain, username, and password and invokes the installed authorization provider without printing secrets.`),
		Examples: []string{
			`aws-auth login --json`,
			`aws-auth --config ~/.efp/config.yaml login --json`,
			`aws-auth commands --json`,
			`aws-auth schema login --json`,
			`aws-auth help llm --json`,
		},
		Groups: map[string]string{
			"login": "Authorize AWS credentials.",
		},
	})
	return c
}

func loginCmd(o *Opts) *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Authorize AWS credentials from configured account credentials.",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, _ := config.ResolvePath(o.Config)
			cfg, err := config.Load(path)
			if err != nil {
				return print(cmd, o, output.Failure("config_error", output.RedactString(err.Error()), "Check EFP_CONFIG or pass --config.", 400))
			}
			login, failure := buildLogin(cfg.AWS)
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
					"Verify the configured AWS domain, username, and password.",
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
}

type loginSpec struct {
	command  string
	args     []string
	env      []string
	password string
}

func buildLogin(aws config.AWSConfig) (loginSpec, *output.Envelope) {
	if aws.Enabled != nil && !*aws.Enabled {
		failure := output.Failure("config_missing", "AWS authorization is not configured.", "Set AWS domain, username, and password in the EFP config.", 400)
		return loginSpec{}, &failure
	}
	domain := singleLine(aws.Domain)
	username := singleLine(aws.Username)
	password := cleanSecret(aws.Password)
	if domain == "" || username == "" || password == "" {
		failure := output.Failure("config_missing", "AWS authorization is not configured.", "Set AWS domain, username, and password in the EFP config.", 400)
		return loginSpec{}, &failure
	}
	return loginSpec{
		command:  adfsAssumeCommand,
		args:     []string{"--jenkins", "-n", "-d", domain, "-u", username},
		env:      withADPass(os.Environ(), password),
		password: password,
	}, nil
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
			"Use aws-auth login --json to authorize AWS credentials from the shared EFP config.",
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
