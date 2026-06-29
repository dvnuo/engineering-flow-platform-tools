package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/catalog"
	"engineering-flow-platform-tools/internal/clihelp"
	"engineering-flow-platform-tools/internal/inspectimage/aiplatform"
	iauth "engineering-flow-platform-tools/internal/inspectimage/auth"
	"engineering-flow-platform-tools/internal/inspectimage/config"
	"engineering-flow-platform-tools/internal/inspectimage/copilot"
	"engineering-flow-platform-tools/internal/inspectimage/imagecheck"
	"engineering-flow-platform-tools/internal/inspectimage/inspect"
	"engineering-flow-platform-tools/internal/inspectimage/vision"
	"engineering-flow-platform-tools/internal/output"
	"engineering-flow-platform-tools/internal/version"
	"github.com/spf13/cobra"
)

type Opts struct {
	Format        string
	ConfigPath    string
	JSON, Verbose bool
}

var exchangeCopilotToken = func(ctx context.Context, cfg config.Config) (string, time.Time, string, error) {
	client := &iauth.DeviceClient{HTTPClient: copilot.NewHTTPClient(time.Duration(cfg.API.TimeoutSeconds) * time.Second)}
	return client.ExchangeCopilotToken(ctx, cfg.Auth.GitHubAccessToken)
}

var exchangeAIPlatformToken = func(ctx context.Context, cfg config.Config, timeout time.Duration) (string, time.Time, error) {
	client := aiplatform.NewClient(cfg, timeout)
	result, err := client.ExchangeToken(ctx, cfg.AIPlatform.Auth.Username, cfg.AIPlatform.Auth.Password)
	if err != nil {
		return "", time.Time{}, err
	}
	return result.Token, result.ExpiresAt, nil
}

func NewRoot() *cobra.Command {
	return NewRootWithClient(nil)
}

func Execute(args []string, stdout, stderr io.Writer) int {
	return clihelp.Execute(NewRoot(), "inspect-image", args, stdout, stderr)
}

func NewRootWithClient(client inspect.ResponsesClient) *cobra.Command {
	cobra.EnableCommandSorting = false
	o := &Opts{Format: "table"}
	c := &cobra.Command{
		Use:   "inspect-image",
		Short: "Inspect one local image with a configured vision provider",
		Long: strings.TrimSpace(`inspect-image is a local CLI for text-only agents that need to understand a screenshot, UI state, diagram, chart, or visible text in exactly one image.

It is invoked from Bash or a terminal. It is not a Portal tool, runtime built-in tool, or MCP server.

The inspect command validates the local image first, then sends the image bytes to the configured provider endpoint. AI Platform /chat/completions is the default provider; GitHub Copilot /responses remains available when explicitly configured. For agent workflows, default every command and subcommand to --json so callers can read ok, data.result.answer, data.result.visible_text, error.code, and error.hint.

Configuration is stored in ~/.efp/config.yaml by default. Set EFP_CONFIG or pass --config to use a different file.`),
		Example: strings.TrimSpace(`inspect-image auth status --json
inspect-image auth login
inspect-image inspect --image ./screenshot.png --prompt "Read the visible error and explain what is happening." --json
inspect-image schema inspect --json
inspect-image help llm`),
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	c.PersistentFlags().BoolVar(&o.JSON, "json", false, "Print a JSON envelope to stdout. Equivalent to --format json.")
	c.PersistentFlags().StringVar(&o.Format, "format", "table", "Output format: table, json, or yaml.")
	c.PersistentFlags().BoolVar(&o.Verbose, "verbose", false, "Enable non-secret diagnostics.")
	c.PersistentFlags().StringVar(&o.ConfigPath, "config", "", "Path to EFP config. Overrides EFP_CONFIG.")
	c.AddCommand(inspectCmd(o, client), authCmd(o), doctorCmd(o), modelsCmd(o), commandsCmd(o), schemaCmd(o), helpLLMCmd(o), versionCmd(o))
	return c
}

func inspectCmd(o *Opts, client inspect.ResponsesClient) *cobra.Command {
	opts := inspect.Options{}
	var outPath string
	c := &cobra.Command{Use: "inspect --image <path> --prompt <text>",
		Short: "Inspect exactly one local image",
		Long: strings.TrimSpace(`Validate one local JPEG, PNG, WEBP, or GIF image and send it to the configured provider endpoint for visual inspection.

Use this for screenshots, UI states, diagrams, charts, visible errors, and OCR-like extraction where plain OCR is too narrow. Remote image URLs, PDFs, video, audio, and multiple images are not supported.

The prompt is appended as the task; it does not replace the built-in safety and structured-output instructions. Use --preset to bias the prompt toward OCR, UI, diagram, chart, or error analysis. In --json mode, failures are returned as ok=false with error.code and error.hint.

Stdout is the primary output path. Use --out <file> only when you want a second JSON envelope copy, such as in Windows cmd or terminal bridges where stdout capture is unreliable. Use --verbose for non-secret stage diagnostics on stderr.`),
		Example: strings.TrimSpace(`inspect-image inspect --image ./screenshot.png --prompt "Read the visible error and explain what is happening." --json
inspect-image inspect --image ./diagram.webp --preset diagram --prompt "Explain this architecture diagram." --json
inspect-image inspect --image ./chart.png --preset chart --prompt-file ./task.txt --out ./inspect-result.json --json
inspect-image.exe inspect --image "%CD%\screenshot.png" --prompt "Read the visible error" --out "%CD%\inspect-image-result.json" --json`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			externalClient := client != nil
			if len(args) > 0 && opts.ImagePath == "" {
				opts.ImagePath = args[0]
			}
			cfgPath, err := config.ResolvePath(o.ConfigPath)
			if err != nil {
				return printWithOut(cmd, o, fail("config_error", messageWithDetail("Config path could not be resolved.", err), "Set EFP_CONFIG or pass --config.", 500), outPath)
			}
			debugf(cmd, o, "config path resolved path=%s", cfgPath)
			cfg, err := config.LoadOrDefault(cfgPath)
			if err != nil {
				return printWithOut(cmd, o, fail("config_error", messageWithDetail("Config file could not be loaded.", err), "Fix ~/.efp/config.yaml or pass --config.", 400), outPath)
			}
			timeout := opts.TimeoutSecond
			if timeout <= 0 {
				timeout = cfg.API.TimeoutSeconds
			}
			debugf(cmd, o, "config loaded provider=%s base_url=%s timeout_seconds=%d", cfg.Provider, cfg.API.BaseURL, timeout)
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(timeout)*time.Second)
			defer cancel()
			if client == nil {
				img, warnings, err := inspectLocalOnly(cfg, opts)
				if err != nil {
					return printErrWithOut(cmd, o, err, outPath)
				}
				debugf(cmd, o, "image validated path=%s mime=%s size_bytes=%d sha256=%s width=%d height=%d animated=%t warnings=%d", img.Path, img.MIMEType, img.SizeBytes, img.SHA256, img.Width, img.Height, img.Animated, len(warnings))
				var refreshErr error
				debugf(cmd, o, "checking provider auth token provider=%s", cfg.Provider)
				cfg, refreshErr = ensureProviderToken(cmd.Context(), cfg, cfgPath, time.Duration(timeout)*time.Second)
				if refreshErr != nil {
					return printErrWithOut(cmd, o, refreshErr, outPath)
				}
				debugf(cmd, o, "provider auth token ready provider=%s", cfg.Provider)
			}
			if client == nil {
				client = newProviderClient(cfg, time.Duration(timeout)*time.Second)
			}
			client = wrapVerboseClient(cmd, o, client)
			result, err := inspect.Run(ctx, cfg, client, opts)
			if err != nil && !externalClient && shouldRefreshAfterProviderAuthError(err, cfg) {
				debugf(cmd, o, "provider request auth failed; refreshing token provider=%s", cfg.Provider)
				var refreshErr error
				cfg, refreshErr = refreshProviderToken(cmd.Context(), cfg, cfgPath, time.Duration(timeout)*time.Second)
				if refreshErr != nil {
					return printErrWithOut(cmd, o, refreshErr, outPath)
				}
				client = wrapVerboseClient(cmd, o, newProviderClient(cfg, time.Duration(timeout)*time.Second))
				result, err = inspect.Run(ctx, cfg, client, opts)
			}
			if err != nil {
				return printErrWithOut(cmd, o, err, outPath)
			}
			debugf(cmd, o, "response parsed successfully")
			return printWithOut(cmd, o, output.Success("", result), outPath)
		}}
	c.Flags().StringVar(&opts.ImagePath, "image", "", "Local image path. Exactly one regular file; remote URLs are rejected.")
	c.Flags().StringVar(&opts.Prompt, "prompt", "", "Task for the model, such as reading an error, explaining a UI state, or summarizing a diagram.")
	c.Flags().StringVar(&opts.PromptFile, "prompt-file", "", "Read the task prompt from a local text file instead of --prompt.")
	c.Flags().StringVar(&opts.Model, "model", config.DefaultModel, "Model to use. Defaults to gpt-5.4-mini; model names are passed through to the configured provider.")
	c.Flags().StringVar(&opts.Reasoning, "reasoning", config.DefaultReasoning, "Reasoning effort. Allowed: low, medium, high, xhigh.")
	c.Flags().StringVar(&opts.Preset, "preset", "general", "Prompt preset: general, ocr, ui, diagram, chart, or error.")
	c.Flags().IntVar(&opts.TimeoutSecond, "timeout", 0, "Request timeout in seconds. Defaults to config api.timeout_seconds.")
	c.Flags().StringVar(&outPath, "out", "", "Write the full JSON envelope to this file in addition to stdout.")
	return c
}

func shouldRefreshAfterProviderAuthError(err error, cfg config.Config) bool {
	var apiErr *copilot.APIError
	if errors.As(err, &apiErr) && apiErr.Code == "auth_required" && cfg.Provider == config.ProviderGitHubCopilot {
		return iauth.GitHubTokenValid(cfg, time.Now())
	}
	var aiErr *aiplatform.APIError
	if errors.As(err, &aiErr) && aiErr.Code == "auth_required" && cfg.Provider == config.ProviderAIPlatform {
		return aiplatform.CredentialsConfigured(cfg) && aiplatform.EndpointsConfigured(cfg)
	}
	return false
}

func inspectLocalOnly(cfg config.Config, opts inspect.Options) (imagecheck.ImageInfo, []string, error) {
	if _, err := inspect.ReadPrompt(opts.Prompt, opts.PromptFile); err != nil {
		return imagecheck.ImageInfo{}, nil, err
	}
	reasoning := opts.Reasoning
	if reasoning == "" {
		reasoning = cfg.Defaults.Reasoning
	}
	if !config.StringAllowed(reasoning, config.AllowedReasoning) {
		return imagecheck.ImageInfo{}, nil, &copilot.APIError{Code: "reasoning_not_allowed", Message: "Reasoning effort is not allowed for inspect-image.", Hint: "Use one of: low, medium, high, xhigh.", Status: 400}
	}
	return imagecheck.Validate(opts.ImagePath, cfg.Limits.MaxImageBytes, cfg.Limits.AllowedMIMETypes)
}

func newProviderClient(cfg config.Config, timeout time.Duration) inspect.ResponsesClient {
	switch cfg.Provider {
	case config.ProviderAIPlatform:
		return aiplatform.NewClient(cfg, timeout)
	default:
		return &copilot.Client{BaseURL: cfg.API.BaseURL, Token: cfg.Auth.CopilotToken, HTTPClient: copilot.NewHTTPClient(timeout)}
	}
}

func ensureProviderToken(ctx context.Context, cfg config.Config, path string, timeout time.Duration) (config.Config, error) {
	switch cfg.Provider {
	case config.ProviderAIPlatform:
		return ensureAIPlatformToken(ctx, cfg, path, timeout)
	case config.ProviderGitHubCopilot:
		return ensureCopilotToken(ctx, cfg, path)
	default:
		return cfg, &copilot.APIError{Code: "config_error", Message: "Unsupported inspect-image provider: " + cfg.Provider, Hint: "Use provider github_copilot_plugin or ai_platform.", Status: 400}
	}
}

func refreshProviderToken(ctx context.Context, cfg config.Config, path string, timeout time.Duration) (config.Config, error) {
	switch cfg.Provider {
	case config.ProviderAIPlatform:
		return refreshAIPlatformToken(ctx, cfg, path, timeout)
	case config.ProviderGitHubCopilot:
		return refreshCopilotToken(ctx, cfg, path)
	default:
		return cfg, &copilot.APIError{Code: "config_error", Message: "Unsupported inspect-image provider: " + cfg.Provider, Hint: "Use provider github_copilot_plugin or ai_platform.", Status: 400}
	}
}

func ensureCopilotToken(ctx context.Context, cfg config.Config, path string) (config.Config, error) {
	if iauth.TokenValid(cfg, time.Now()) && !iauth.NeedsExchange(cfg) {
		return cfg, nil
	}
	if cfg.Auth.CopilotToken == "" && cfg.Auth.GitHubAccessToken == "" {
		return cfg, &copilot.APIError{Code: "auth_required", Message: "GitHub Copilot authentication is required.", Hint: "Run inspect-image auth login.", Status: 401}
	}
	return refreshCopilotToken(ctx, cfg, path)
}

func refreshCopilotToken(ctx context.Context, cfg config.Config, path string) (config.Config, error) {
	if !iauth.GitHubTokenValid(cfg, time.Now()) {
		return cfg, &copilot.APIError{Code: "auth_expired", Message: "GitHub Copilot authentication expired.", Hint: "Run inspect-image auth login.", Status: 401}
	}
	token, expires, apiBaseURL, err := exchangeCopilotToken(ctx, cfg)
	if err != nil {
		return cfg, err
	}
	cfg.Auth.CopilotToken = token
	cfg.Auth.CopilotTokenExpiresAt = expires.UTC().Format(time.RFC3339)
	if apiBaseURL != "" {
		cfg.API.BaseURL = apiBaseURL
	}
	cfg.Auth.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := config.Save(path, cfg); err != nil {
		return cfg, &copilot.APIError{Code: "config_error", Message: messageWithDetail("Config file could not be saved.", err), Hint: "Check permissions for ~/.efp/config.yaml and ~/.efp/tmp/copilot_token.", Status: 500}
	}
	return cfg, nil
}

func ensureAIPlatformToken(ctx context.Context, cfg config.Config, path string, timeout time.Duration) (config.Config, error) {
	if aiplatform.TokenValid(cfg, time.Now()) {
		return cfg, nil
	}
	if !aiplatform.CredentialsConfigured(cfg) || !aiplatform.EndpointsConfigured(cfg) {
		return cfg, &aiplatform.APIError{Code: "auth_required", Message: "AI Platform authentication is required.", Hint: "Configure ai_platform.auth.username, password, usercase, chat host/uri, and ib2b host/uri in ~/.efp/config.yaml, then run inspect-image auth test --json.", Status: 401}
	}
	return refreshAIPlatformToken(ctx, cfg, path, timeout)
}

func refreshAIPlatformToken(ctx context.Context, cfg config.Config, path string, timeout time.Duration) (config.Config, error) {
	if !aiplatform.CredentialsConfigured(cfg) || !aiplatform.EndpointsConfigured(cfg) {
		return cfg, &aiplatform.APIError{Code: "auth_required", Message: "AI Platform authentication is required.", Hint: "Configure ai_platform.auth.username, password, usercase, chat host/uri, and ib2b host/uri in ~/.efp/config.yaml.", Status: 401}
	}
	token, expires, err := exchangeAIPlatformToken(ctx, cfg, timeout)
	if err != nil {
		return cfg, err
	}
	cfg.AIPlatform.Auth.Token = token
	cfg.AIPlatform.Auth.TokenExpiresAt = expires.UTC().Format(time.RFC3339)
	cfg.AIPlatform.Auth.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := config.Save(path, cfg); err != nil {
		return cfg, &aiplatform.APIError{Code: "config_error", Message: messageWithDetail("Config file could not be saved.", err), Hint: "Check permissions for ~/.efp/config.yaml and ~/.efp/tmp/ai_platform_token.", Status: 500}
	}
	return cfg, nil
}

func authCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{
		Use:   "auth",
		Short: "Manage inspect-image provider authentication",
		Long: strings.TrimSpace(`Manage authentication for the configured inspect-image provider.

GitHub Copilot uses GitHub device-flow login. AI Platform uses configured username, password, and usercase credentials to exchange a short-lived iB2B token.

Token values are never printed by auth status, doctor, errors, or verbose diagnostics.`),
		Example: strings.TrimSpace(`inspect-image auth status --json
inspect-image auth login
inspect-image auth test --json
inspect-image auth logout --yes --json`),
	}
	c.AddCommand(authLoginCmd(o), authStatusCmd(o), authTestCmd(o), authLogoutCmd(o))
	return c
}

type authTestResult struct {
	iauth.Status
	Refreshed bool `json:"refreshed"`
}

type aiAuthTestResult struct {
	aiplatform.Status
	Refreshed bool `json:"refreshed"`
}

func authLoginCmd(o *Opts) *cobra.Command {
	var providerFlag, username, password, usercase string
	var passwordStdin bool
	c := &cobra.Command{Use: "login",
		Short: "Sign in with the configured provider",
		Long: strings.TrimSpace(`Create the config file if needed.

For GitHub Copilot, start GitHub device authentication, print the verification URL and user code in human mode, then exchange the GitHub token for a Copilot plugin token.

For AI Platform, save username, password, and usercase credentials from flags or stdin, then validate them by exchanging an iB2B token.

JSON mode prints only non-secret status fields such as auth_configured, provider, token_state, github_user, and token expiry timestamps.`),
		Example: strings.TrimSpace(`inspect-image auth login --provider github_copilot_plugin
inspect-image auth login --provider ai_platform --username <user> --usercase <case> --password-stdin --json`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.ResolvePath(o.ConfigPath)
			if err != nil {
				return print(cmd, o, fail("config_error", messageWithDetail("Config path could not be resolved.", err), "Set EFP_CONFIG or pass --config.", 500))
			}
			cfg, err := config.LoadOrDefault(path)
			if err != nil {
				return print(cmd, o, fail("config_error", messageWithDetail("Config file could not be loaded.", err), "Fix ~/.efp/config.yaml or pass --config.", 400))
			}
			if strings.TrimSpace(providerFlag) != "" {
				cfg.Provider = config.NormalizeProvider(providerFlag)
			}
			if cfg.Provider != config.ProviderGitHubCopilot && cfg.Provider != config.ProviderAIPlatform {
				return print(cmd, o, fail("config_error", "Unsupported inspect-image provider: "+cfg.Provider, "Use provider github_copilot_plugin or ai_platform.", 400))
			}
			if cfg.Provider == config.ProviderAIPlatform {
				if strings.TrimSpace(username) != "" {
					cfg.AIPlatform.Auth.Username = strings.TrimSpace(username)
				}
				if passwordStdin {
					b, err := io.ReadAll(cmd.InOrStdin())
					if err != nil {
						return print(cmd, o, fail("invalid_args", messageWithDetail("Password could not be read from stdin.", err), "Pipe the AI Platform password to --password-stdin.", 400))
					}
					password = strings.TrimSpace(string(b))
				}
				if strings.TrimSpace(password) != "" {
					cfg.AIPlatform.Auth.Password = strings.TrimSpace(password)
				}
				if strings.TrimSpace(usercase) != "" {
					cfg.AIPlatform.Auth.Usercase = strings.TrimSpace(usercase)
				}
				if !aiplatform.CredentialsConfigured(cfg) {
					return print(cmd, o, fail("invalid_args", "AI Platform username, password, and usercase are required.", "Pass --username, --usercase, and --password-stdin, or edit ~/.efp/config.yaml.", 400))
				}
				if !aiplatform.EndpointsConfigured(cfg) {
					return print(cmd, o, fail("config_error", "AI Platform chat and iB2B endpoints are required.", "Set ai_platform.chat.host/uri and ai_platform.ib2b.host/uri in ~/.efp/config.yaml.", 400))
				}
				updated, err := refreshAIPlatformToken(cmd.Context(), cfg, path, time.Duration(cfg.API.TimeoutSeconds)*time.Second)
				if err != nil {
					return printErr(cmd, o, err)
				}
				return print(cmd, o, output.Success("", aiplatform.Summarize(updated, time.Now())))
			}
			var humanOut io.Writer
			if fmtOut(o) != "json" {
				humanOut = cmd.OutOrStdout()
			}
			client := &iauth.DeviceClient{HTTPClient: copilot.NewHTTPClient(time.Duration(cfg.API.TimeoutSeconds) * time.Second)}
			updated, result, err := client.Login(cmd.Context(), cfg, humanOut)
			if err != nil {
				return printErr(cmd, o, err)
			}
			if err := config.Save(path, updated); err != nil {
				return print(cmd, o, fail("config_error", messageWithDetail("Config file could not be saved.", err), "Check permissions for ~/.efp/config.yaml and ~/.efp/tmp/copilot_token.", 500))
			}
			return print(cmd, o, output.Success("", result))
		}}
	c.Flags().StringVar(&providerFlag, "provider", "", "Provider to authenticate: github_copilot_plugin or ai_platform. Defaults to configured provider.")
	c.Flags().StringVar(&username, "username", "", "AI Platform username. Used only with --provider ai_platform.")
	c.Flags().StringVar(&password, "password", "", "AI Platform password. Prefer --password-stdin to avoid shell history.")
	c.Flags().BoolVar(&passwordStdin, "password-stdin", false, "Read the AI Platform password from stdin.")
	c.Flags().StringVar(&usercase, "usercase", "", "AI Platform usercase value, sent as the chat/completions user field.")
	return c
}

func authStatusCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "status",
		Short: "Show non-secret authentication status",
		Long: strings.TrimSpace(`Read the inspect-image config and report whether provider authentication is configured, currently valid, or refreshable.

This command never prints github_access_token, copilot_token, AI Platform passwords, iB2B tokens, Authorization headers, or token-derived secrets. In --json mode, an expired short-lived token still returns ok=true when stored credentials can refresh it.`),
		Example: "inspect-image auth status --json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.ResolvePath(o.ConfigPath)
			if err != nil {
				return print(cmd, o, fail("config_error", messageWithDetail("Config path could not be resolved.", err), "Set EFP_CONFIG or pass --config.", 500))
			}
			cfg, err := config.Load(path)
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return print(cmd, o, fail("config_error", messageWithDetail("Config file could not be loaded.", err), "Fix ~/.efp/config.yaml or pass --config.", 400))
				}
				return print(cmd, o, authRequiredFailure(config.Provider))
			}
			now := time.Now()
			if cfg.Provider == config.ProviderAIPlatform {
				status := aiplatform.Summarize(cfg, now)
				if !aiplatform.AuthUsable(cfg, now) {
					return print(cmd, o, fail("auth_required", "AI Platform authentication is required.", "Configure ai_platform auth and endpoints, then run inspect-image auth test --json.", 401))
				}
				return print(cmd, o, output.Success("", status))
			}
			status := iauth.Summarize(cfg, now)
			if !iauth.AuthUsable(cfg, now) {
				return print(cmd, o, fail("auth_required", "GitHub Copilot authentication is required.", "Run inspect-image auth login.", 401))
			}
			return print(cmd, o, output.Success("", status))
		}}
}

func authTestCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "test",
		Short: "Refresh and validate provider authentication",
		Long: strings.TrimSpace(`Validate inspect-image authentication without printing secrets.

If the short-lived provider token is expired but stored credentials are still usable, this command exchanges them for a fresh token and saves the updated config. Only run auth login when this command returns auth_required or auth_expired.`),
		Example: "inspect-image auth test --json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.ResolvePath(o.ConfigPath)
			if err != nil {
				return print(cmd, o, fail("config_error", messageWithDetail("Config path could not be resolved.", err), "Set EFP_CONFIG or pass --config.", 500))
			}
			cfg, err := config.Load(path)
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return print(cmd, o, fail("config_error", messageWithDetail("Config file could not be loaded.", err), "Fix ~/.efp/config.yaml or pass --config.", 400))
				}
				return print(cmd, o, authRequiredFailure(config.Provider))
			}
			refreshed := false
			if cfg.Provider == config.ProviderAIPlatform {
				if !aiplatform.AuthUsable(cfg, time.Now()) {
					return print(cmd, o, fail("auth_required", "AI Platform authentication is required.", "Configure ai_platform auth and endpoints, then run inspect-image auth login --provider ai_platform or auth test.", 401))
				}
				if !aiplatform.TokenValid(cfg, time.Now()) {
					var refreshErr error
					cfg, refreshErr = refreshAIPlatformToken(cmd.Context(), cfg, path, time.Duration(cfg.API.TimeoutSeconds)*time.Second)
					if refreshErr != nil {
						return printErr(cmd, o, refreshErr)
					}
					refreshed = true
				}
				return print(cmd, o, output.Success("", aiAuthTestResult{Status: aiplatform.Summarize(cfg, time.Now()), Refreshed: refreshed}))
			}
			if !iauth.TokenValid(cfg, time.Now()) || iauth.NeedsExchange(cfg) {
				var refreshErr error
				cfg, refreshErr = refreshCopilotToken(cmd.Context(), cfg, path)
				if refreshErr != nil {
					return printErr(cmd, o, refreshErr)
				}
				refreshed = true
			}
			return print(cmd, o, output.Success("", authTestResult{Status: iauth.Summarize(cfg, time.Now()), Refreshed: refreshed}))
		}}
}

func authLogoutCmd(o *Opts) *cobra.Command {
	var yes bool
	c := &cobra.Command{Use: "logout",
		Short: "Clear stored authentication tokens",
		Long: strings.TrimSpace(`Clear active provider authentication fields from the inspect-image config while preserving api, defaults, limits, privacy, and other non-secret settings.

This command requires --yes so agents do not accidentally remove credentials during discovery.`),
		Example: "inspect-image auth logout --yes --json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return print(cmd, o, fail("invalid_args", "auth logout requires --yes.", "Re-run inspect-image auth logout --yes.", 400))
			}
			path, err := config.ResolvePath(o.ConfigPath)
			if err != nil {
				return print(cmd, o, fail("config_error", messageWithDetail("Config path could not be resolved.", err), "Set EFP_CONFIG or pass --config.", 500))
			}
			cfg, err := config.LoadOrDefault(path)
			if err != nil {
				return print(cmd, o, fail("config_error", messageWithDetail("Config file could not be loaded.", err), "Fix ~/.efp/config.yaml or pass --config.", 400))
			}
			if cfg.Provider == config.ProviderAIPlatform {
				cfg = aiplatform.Logout(cfg)
			} else {
				cfg = iauth.Logout(cfg)
			}
			if err := config.Save(path, cfg); err != nil {
				return print(cmd, o, fail("config_error", messageWithDetail("Config file could not be saved.", err), tokenSaveHint(cfg.Provider), 500))
			}
			return print(cmd, o, output.Success("", map[string]any{"auth_configured": false, "provider": cfg.Provider, "github_host": cfg.Auth.GitHubHost}))
		}}
	c.Flags().BoolVar(&yes, "yes", false, "Confirm that stored auth tokens should be cleared.")
	return c
}

func doctorCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "doctor",
		Short: "Check inspect-image configuration and provider readiness",
		Long: strings.TrimSpace(`Run local readiness checks for the config file, permissions, authentication, system proxy mode, provider endpoint configuration, and model defaults.

Doctor does not print tokens. An expired short-lived token is ok when it is refreshable from stored provider credentials; missing or unusable auth returns ok=false with error.code=auth_required.`),
		Example: "inspect-image doctor --json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.ResolvePath(o.ConfigPath)
			if err != nil {
				return print(cmd, o, fail("config_error", messageWithDetail("Config path could not be resolved.", err), "Set EFP_CONFIG or pass --config.", 500))
			}
			cfg, err := config.Load(path)
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return print(cmd, o, fail("config_error", messageWithDetail("Config file could not be loaded.", err), "Fix ~/.efp/config.yaml or pass --config.", 400))
				}
				return print(cmd, o, authRequiredFailure(config.Provider))
			}
			now := time.Now()
			if cfg.Provider == config.ProviderAIPlatform {
				status := aiplatform.Summarize(cfg, now)
				if !aiplatform.AuthUsable(cfg, now) {
					return print(cmd, o, fail("auth_required", "AI Platform authentication is required.", "Configure ai_platform auth and endpoints, then run inspect-image auth test --json.", 401))
				}
				authCheck := "ok"
				if !status.TokenValid && status.TokenRefreshable {
					authCheck = "refreshable"
				}
				return print(cmd, o, output.Success("", map[string]any{"checks": map[string]any{
					"config":                    "ok",
					"permissions":               permStatus(path),
					"provider":                  cfg.Provider,
					"auth":                      authCheck,
					"token_valid":               status.TokenValid,
					"token_refreshable":         status.TokenRefreshable,
					"chat_endpoint_configured":  status.ChatEndpointConfigured,
					"ib2b_endpoint_configured":  status.IB2BEndpointConfigured,
					"proxy":                     "system",
					"chat_completions_endpoint": "ok",
					"default_model":             cfg.Defaults.Model,
				}}))
			}
			status := iauth.Summarize(cfg, now)
			if !iauth.AuthUsable(cfg, now) {
				return print(cmd, o, fail("auth_required", "GitHub Copilot authentication is required.", "Run inspect-image auth login.", 401))
			}
			authCheck := "ok"
			if !status.CopilotTokenValid && status.CopilotTokenRefreshable {
				authCheck = "refreshable"
			}
			return print(cmd, o, output.Success("", map[string]any{"checks": map[string]any{
				"config":                    "ok",
				"permissions":               permStatus(path),
				"provider":                  cfg.Provider,
				"auth":                      authCheck,
				"copilot_token_valid":       status.CopilotTokenValid,
				"copilot_token_refreshable": status.CopilotTokenRefreshable,
				"proxy":                     "system",
				"responses_endpoint":        "ok",
				"default_model":             cfg.Defaults.Model,
			}}))
		}}
}

func modelsCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "models",
		Short: "Show model defaults and reasoning efforts",
		Long: strings.TrimSpace(`Print the default model and reasoning allowlist used by inspect-image.

Model names are passed through to the configured provider. Reasoning efforts are still validated locally before any image bytes are sent.`),
		Example: "inspect-image models --json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return print(cmd, o, output.Success("", map[string]any{"default_model": config.DefaultModel, "model_restrictions": "none", "default_reasoning": config.DefaultReasoning, "allowed_reasoning": config.AllowedReasoning}))
		}}
}

func commandsCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "commands",
		Short: "List agent-facing inspect-image commands",
		Long: strings.TrimSpace(`Print the stable command catalog for agents, including usage, risk, descriptions, examples, flags, and required arguments.

Agents should call this with --json when discovering how to use inspect-image.`),
		Example: "inspect-image commands --json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return print(cmd, o, output.Success("", map[string]any{"commands": catalog.Commands("inspect-image")}))
		}}
}

func schemaCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "schema <command>",
		Short: "Show a machine-readable command schema",
		Long: strings.TrimSpace(`Show the schema for a command. For inspect, the schema includes required parameters, the reasoning enum, model default, and image size and MIME type limits.

Agents should call inspect-image schema inspect --json before building complex inspect commands.`),
		Example: "inspect-image schema inspect --json",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] == "inspect" {
				return print(cmd, o, output.Success("", inspectSchema()))
			}
			return print(cmd, o, output.Success("", catalog.Schema("inspect-image", args[0])))
		}}
}

func versionCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "version",
		Short:   "Print build version information",
		Long:    "Print the inspect-image build version, commit, and build date using the standard ok/data JSON envelope when --json is set.",
		Example: "inspect-image version --json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return print(cmd, o, output.Success("", map[string]any{"version": version.Version, "commit": version.Commit, "date": version.Date}))
		}}
}

func helpLLMCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "help llm",
		Short: "Print usage guidance for LLM agents",
		Long: strings.TrimSpace(`Print concise guidance for LLM agents that invoke inspect-image from Bash or a terminal.

The JSON form includes tips and the command catalog. The text form is suitable for humans or for copying into an agent instruction file.`),
		Example: strings.TrimSpace(`inspect-image help llm
inspect-image help llm --json`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tips := inspectImageLLMTips()
			if fmtOut(o) == "json" {
				return print(cmd, o, output.Success("", map[string]any{"tips": tips, "commands": catalog.Commands("inspect-image")}))
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), inspectImageLLMMarkdown(tips))
			return err
		}}
}

func inspectSchema() map[string]any {
	return map[string]any{
		"name":     "inspect",
		"required": []string{"image", "prompt"},
		"properties": map[string]any{
			"image":     map[string]any{"type": "string", "description": "Local image path. Exactly one image."},
			"prompt":    map[string]any{"type": "string"},
			"out":       map[string]any{"type": "string", "description": "Optional file path for a JSON envelope copy. Useful when terminal stdout capture is unreliable."},
			"model":     map[string]any{"type": "string", "default": config.DefaultModel, "description": "Passed through to the configured provider; no local allowlist is enforced."},
			"reasoning": map[string]any{"type": "string", "enum": config.AllowedReasoning, "default": config.DefaultReasoning},
		},
		"limits": map[string]any{"max_image_bytes": config.MaxImageBytes, "allowed_mime_types": config.AllowedMIMETypes},
	}
}

func inspectImageLLMTips() []string {
	return []string{
		"For agents, --json is the default way to use every inspect-image command and subcommand.",
		"Always add --json to inspect, auth status, auth test, doctor, models, commands, schema, version, and help llm so results and failures use the stable envelope.",
		"Only omit --json for human-facing interactive output such as asking the user to run inspect-image auth login and read the device-code prompt.",
		"Use inspect-image inspect --image <path> --prompt <task> --json.",
		"For image analysis, recognition, screenshot understanding, UI inspection, diagrams, charts, visual errors, or visible text in an image, use inspect-image as the only image-analysis path.",
		"Do not use OCR tools as the primary path, and do not write Python/OpenCV/Tesseract/image-parser scripts to recognize or interpret image content.",
		"Accepts exactly one JPEG, PNG, WEBP, or GIF image up to 3145728 bytes.",
		"Read data.result.answer first.",
		"For OCR tasks, read data.result.visible_text.",
		"If ok=false, inspect error.code and error.hint before retrying.",
		"If inspect-image auth status returns token_state=refreshable, run inspect-image auth test --json or retry inspect-image inspect --json; do not ask for auth login.",
		"If auth_required or auth_expired and auth status is not refreshable, ask the user to run inspect-image auth login, wait for completion, then retry inspect-image inspect --json.",
		"Do not fall back to OCR, Python image recognition, manual guessing, or another image-analysis approach when auth is missing.",
		"Stdout is the primary output path; --out writes an additional JSON envelope copy and should be used only for terminal-capture issues or diagnostics.",
		"On Windows cmd, use double quotes and cmd-native commands such as where/dir/cd/type; do not use Bash-only commands such as pwd, command -v, cat, ls, cd \"$PWD\", or single quotes.",
		"Example for Windows cmd fallback: inspect-image.exe inspect --image \"%CD%\\screenshot.png\" --prompt \"Read the visible error\" --out \"%CD%\\inspect-image-result.json\" --json",
		"If inspect appears to produce no stdout, rerun with --verbose --out <workspace-file> --json, then read the file with the file-read tool; use type <file> only when no file-read tool is available.",
	}
}

func inspectImageLLMMarkdown(tips []string) string {
	var b strings.Builder
	b.WriteString("# inspect-image CLI usage for agents\n\n")
	for _, tip := range tips {
		b.WriteString("- ")
		b.WriteString(tip)
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func printErr(cmd *cobra.Command, o *Opts, err error) error {
	return print(cmd, o, errEnvelope(err))
}

func printErrWithOut(cmd *cobra.Command, o *Opts, err error, outPath string) error {
	return printWithOut(cmd, o, errEnvelope(err), outPath)
}

func errEnvelope(err error) output.Envelope {
	var ie *inspect.InspectError
	if errors.As(err, &ie) {
		return fail(ie.Code, ie.Message, ie.Hint, ie.Status)
	}
	var ve *imagecheck.ValidationError
	if errors.As(err, &ve) {
		return fail(ve.Code, ve.Message, ve.Hint, ve.Status)
	}
	var ce *copilot.APIError
	if errors.As(err, &ce) {
		return fail(ce.Code, ce.Message, ce.Hint, ce.Status)
	}
	var ai *aiplatform.APIError
	if errors.As(err, &ai) {
		return fail(ai.Code, ai.Message, ai.Hint, ai.Status)
	}
	var le *vision.Error
	if errors.As(err, &le) {
		return fail(le.Code, le.Message, le.Hint, le.Status)
	}
	var ae *iauth.APIError
	if errors.As(err, &ae) {
		return fail(ae.Code, ae.Message, ae.Hint, ae.Status)
	}
	detail := config.RedactString(strings.TrimSpace(err.Error()))
	message := "inspect-image failed."
	if detail != "" {
		message += " " + detail
	}
	return fail("unknown_error", message, "Retry with --json and inspect error.code.", 500)
}

func fail(code, message, hint string, status int) output.Envelope {
	return output.Failure(code, message, hint, status)
}

func authRequiredFailure(provider string) output.Envelope {
	switch config.NormalizeProvider(provider) {
	case config.ProviderAIPlatform:
		return fail("auth_required", "AI Platform authentication is required.", "Configure ai_platform auth and endpoints, then run inspect-image auth test --json.", 401)
	case config.ProviderGitHubCopilot:
		return fail("auth_required", "GitHub Copilot authentication is required.", "Run inspect-image auth login --provider github_copilot_plugin.", 401)
	default:
		return fail("auth_required", "Provider authentication is required.", "Set inspect_image.provider to ai_platform or github_copilot_plugin, then run inspect-image auth test --json.", 401)
	}
}

func tokenSaveHint(provider string) string {
	switch config.NormalizeProvider(provider) {
	case config.ProviderAIPlatform:
		return "Check permissions for ~/.efp/config.yaml and ~/.efp/tmp/ai_platform_token."
	case config.ProviderGitHubCopilot:
		return "Check permissions for ~/.efp/config.yaml and ~/.efp/tmp/copilot_token."
	default:
		return "Check permissions for ~/.efp/config.yaml and provider token files under ~/.efp/tmp."
	}
}

func messageWithDetail(message string, err error) string {
	if err == nil {
		return message
	}
	detail := config.RedactString(strings.TrimSpace(err.Error()))
	if detail == "" {
		return message
	}
	return message + " " + detail
}

func print(cmd *cobra.Command, o *Opts, env output.Envelope) error {
	return output.Print(cmd.OutOrStdout(), fmtOut(o), env)
}

func printWithOut(cmd *cobra.Command, o *Opts, env output.Envelope, outPath string) error {
	if strings.TrimSpace(outPath) != "" {
		if err := writeEnvelopeFile(outPath, env); err != nil {
			failEnv := fail("invalid_args", messageWithDetail("Output file could not be written.", err), "Choose a writable --out path or omit --out.", 400)
			debugf(cmd, o, "output file write failed path=%s error=%s json_envelope_ok=false process_exit_code=0", outPath, config.RedactString(err.Error()))
			return print(cmd, o, failEnv)
		}
		debugf(cmd, o, "wrote JSON envelope path=%s", filepath.Clean(outPath))
	}
	if env.OK {
		debugf(cmd, o, "json_envelope_ok=true process_exit_code=0")
	} else if env.Error != nil {
		debugf(cmd, o, "json_envelope_ok=false error_code=%s status=%d process_exit_code=0", env.Error.Code, env.Error.Status)
	}
	return print(cmd, o, env)
}

func writeEnvelopeFile(path string, env output.Envelope) error {
	env = output.RedactEnvelope(env)
	clean := filepath.Clean(path)
	if dir := filepath.Dir(clean); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	b, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if err := os.WriteFile(clean, b, 0o600); err != nil {
		return err
	}
	_ = os.Chmod(clean, 0o600)
	return nil
}

func debugf(cmd *cobra.Command, o *Opts, format string, args ...any) {
	if o == nil || !o.Verbose {
		return
	}
	message := config.RedactString(fmt.Sprintf(format, args...))
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "inspect-image verbose: %s\n", message)
}

type verboseResponsesClient struct {
	cmd   *cobra.Command
	opts  *Opts
	inner inspect.ResponsesClient
}

func wrapVerboseClient(cmd *cobra.Command, o *Opts, client inspect.ResponsesClient) inspect.ResponsesClient {
	if o == nil || !o.Verbose || client == nil {
		return client
	}
	return &verboseResponsesClient{cmd: cmd, opts: o, inner: client}
}

func (v *verboseResponsesClient) Responses(ctx context.Context, req vision.Request) (map[string]any, error) {
	debugf(v.cmd, v.opts, "sending provider image request model=%s reasoning=%s content_items=%d", req.Model, req.Reasoning.Effort, countInputContent(req))
	raw, err := v.inner.Responses(ctx, req)
	if err != nil {
		debugf(v.cmd, v.opts, "provider image request failed error=%s", err.Error())
		return raw, err
	}
	debugf(v.cmd, v.opts, "provider image response received")
	return raw, nil
}

func countInputContent(req vision.Request) int {
	n := 0
	for _, input := range req.Input {
		n += len(input.Content)
	}
	return n
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

func permStatus(path string) string {
	if _, err := os.Stat(path); err != nil {
		return "missing"
	}
	if config.PermissionOK(path) {
		return "ok"
	}
	return "too_open"
}
