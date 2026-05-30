package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/catalog"
	iauth "engineering-flow-platform-tools/internal/inspectimage/auth"
	"engineering-flow-platform-tools/internal/inspectimage/config"
	"engineering-flow-platform-tools/internal/inspectimage/copilot"
	"engineering-flow-platform-tools/internal/inspectimage/imagecheck"
	"engineering-flow-platform-tools/internal/inspectimage/inspect"
	"engineering-flow-platform-tools/internal/output"
	"engineering-flow-platform-tools/internal/version"
	"github.com/spf13/cobra"
)

type Opts struct {
	Format        string
	ConfigPath    string
	JSON, Verbose bool
}

func NewRoot() *cobra.Command {
	return NewRootWithClient(nil)
}

func NewRootWithClient(client inspect.ResponsesClient) *cobra.Command {
	cobra.EnableCommandSorting = false
	o := &Opts{Format: "table"}
	c := &cobra.Command{
		Use:   "inspect-image",
		Short: "Inspect one local image with a GitHub Copilot backed vision model",
		Long: strings.TrimSpace(`inspect-image is a local CLI for text-only agents that need to understand a screenshot, UI state, diagram, chart, or visible text in exactly one image.

It is invoked from Bash or a terminal. It is not a Portal tool, runtime built-in tool, or MCP server.

The inspect command validates the local image first, then sends the image bytes to the configured GitHub Copilot plugin /responses endpoint. Use --json for agent workflows so callers can read ok, data.result.answer, data.result.visible_text, error.code, and error.hint.

Configuration is stored in ~/.copilot/inspect-image.json by default. Set INSPECT_IMAGE_CONFIG or pass --config to use a different file.`),
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
	c.PersistentFlags().StringVar(&o.ConfigPath, "config", "", "Path to inspect-image config. Overrides INSPECT_IMAGE_CONFIG and COPILOT_HOME.")
	c.AddCommand(inspectCmd(o, client), authCmd(o), doctorCmd(o), modelsCmd(o), commandsCmd(o), schemaCmd(o), helpLLMCmd(o), versionCmd(o))
	return c
}

func inspectCmd(o *Opts, client inspect.ResponsesClient) *cobra.Command {
	opts := inspect.Options{}
	c := &cobra.Command{Use: "inspect --image <path> --prompt <text>",
		Short: "Inspect exactly one local image",
		Long: strings.TrimSpace(`Validate one local JPEG, PNG, WEBP, or GIF image and send it to the GitHub Copilot plugin /responses endpoint for visual inspection.

Use this for screenshots, UI states, diagrams, charts, visible errors, and OCR-like extraction where plain OCR is too narrow. Remote image URLs, PDFs, video, audio, and multiple images are not supported.

The prompt is appended as the task; it does not replace the built-in safety and structured-output instructions. Use --preset to bias the prompt toward OCR, UI, diagram, chart, or error analysis. In --json mode, failures are returned as ok=false with error.code and error.hint.`),
		Example: strings.TrimSpace(`inspect-image inspect --image ./screenshot.png --prompt "Read the visible error and explain what is happening." --json
inspect-image inspect --image ./diagram.webp --preset diagram --prompt "Explain this architecture diagram." --json
inspect-image inspect --image ./chart.png --preset chart --prompt-file ./task.txt --json`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && opts.ImagePath == "" {
				opts.ImagePath = args[0]
			}
			cfgPath, err := config.ResolvePath(o.ConfigPath)
			if err != nil {
				return print(cmd, o, fail("config_error", messageWithDetail("Config path could not be resolved.", err), "Set INSPECT_IMAGE_CONFIG or pass --config.", 500))
			}
			cfg, err := config.LoadOrDefault(cfgPath)
			if err != nil {
				return print(cmd, o, fail("config_error", messageWithDetail("Config file could not be loaded.", err), "Fix inspect-image.json or pass --config.", 400))
			}
			timeout := opts.TimeoutSecond
			if timeout <= 0 {
				timeout = cfg.API.TimeoutSeconds
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(timeout)*time.Second)
			defer cancel()
			if client == nil {
				if _, _, err := inspectLocalOnly(cfg, opts); err != nil {
					return printErr(cmd, o, err)
				}
				var refreshErr error
				cfg, refreshErr = ensureCopilotToken(cmd.Context(), cfg, cfgPath)
				if refreshErr != nil {
					return printErr(cmd, o, refreshErr)
				}
			}
			if client == nil {
				client = &copilot.Client{BaseURL: cfg.API.BaseURL, Token: cfg.Auth.CopilotToken, HTTPClient: copilot.NewHTTPClient(time.Duration(timeout) * time.Second)}
			}
			result, err := inspect.Run(ctx, cfg, client, opts)
			if err != nil {
				return printErr(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		}}
	c.Flags().StringVar(&opts.ImagePath, "image", "", "Local image path. Exactly one regular file; remote URLs are rejected.")
	c.Flags().StringVar(&opts.Prompt, "prompt", "", "Task for the model, such as reading an error, explaining a UI state, or summarizing a diagram.")
	c.Flags().StringVar(&opts.PromptFile, "prompt-file", "", "Read the task prompt from a local text file instead of --prompt.")
	c.Flags().StringVar(&opts.Model, "model", config.DefaultModel, "Model to use. Allowed: gpt-5.4, gpt-5-mini, gpt-5.4-mini.")
	c.Flags().StringVar(&opts.Reasoning, "reasoning", config.DefaultReasoning, "Reasoning effort. Allowed: low, medium, high, xhigh.")
	c.Flags().StringVar(&opts.Preset, "preset", "general", "Prompt preset: general, ocr, ui, diagram, chart, or error.")
	c.Flags().IntVar(&opts.TimeoutSecond, "timeout", 0, "Request timeout in seconds. Defaults to config api.timeout_seconds.")
	return c
}

func inspectLocalOnly(cfg config.Config, opts inspect.Options) (imagecheck.ImageInfo, []string, error) {
	if _, err := inspect.ReadPrompt(opts.Prompt, opts.PromptFile); err != nil {
		return imagecheck.ImageInfo{}, nil, err
	}
	model := opts.Model
	if model == "" {
		model = cfg.Defaults.Model
	}
	if !config.StringAllowed(model, config.AllowedModels) {
		return imagecheck.ImageInfo{}, nil, &copilot.APIError{Code: "model_not_allowed", Message: "Model is not allowed for inspect-image.", Hint: "Run inspect-image models --json and choose an allowed model.", Status: 400}
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

func ensureCopilotToken(ctx context.Context, cfg config.Config, path string) (config.Config, error) {
	if iauth.TokenValid(cfg, time.Now()) && !iauth.NeedsExchange(cfg) {
		return cfg, nil
	}
	if cfg.Auth.CopilotToken == "" && cfg.Auth.GitHubAccessToken == "" {
		return cfg, &copilot.APIError{Code: "auth_required", Message: "GitHub Copilot authentication is required.", Hint: "Run inspect-image auth login.", Status: 401}
	}
	if cfg.Auth.GitHubAccessToken == "" {
		return cfg, &copilot.APIError{Code: "auth_expired", Message: "GitHub Copilot authentication expired.", Hint: "Run inspect-image auth login.", Status: 401}
	}
	client := &iauth.DeviceClient{HTTPClient: copilot.NewHTTPClient(time.Duration(cfg.API.TimeoutSeconds) * time.Second)}
	token, expires, apiBaseURL, err := client.ExchangeCopilotToken(ctx, cfg.Auth.GitHubAccessToken)
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
		return cfg, &copilot.APIError{Code: "config_error", Message: messageWithDetail("Config file could not be saved.", err), Hint: "Check permissions for ~/.copilot/inspect-image.json.", Status: 500}
	}
	return cfg, nil
}

func authCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{
		Use:   "auth",
		Short: "Manage GitHub Copilot authentication",
		Long: strings.TrimSpace(`Manage the GitHub device-flow login used to exchange a GitHub access token for a GitHub Copilot plugin token.

Tokens are stored in the same inspect-image config file as defaults and limits. Token values are never printed by auth status, doctor, errors, or verbose diagnostics.`),
		Example: strings.TrimSpace(`inspect-image auth status --json
inspect-image auth login
inspect-image auth logout --yes --json`),
	}
	c.AddCommand(authLoginCmd(o), authStatusCmd(o), authLogoutCmd(o))
	return c
}

func authLoginCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "login",
		Short: "Sign in with GitHub device flow",
		Long: strings.TrimSpace(`Create the config file if needed, start GitHub device authentication, print the verification URL and user code in human mode, then exchange the GitHub token for a Copilot plugin token.

JSON mode prints only non-secret status fields such as auth_configured, github_host, github_user, and copilot_token_expires_at.`),
		Example: strings.TrimSpace(`inspect-image auth login
inspect-image auth login --json`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.ResolvePath(o.ConfigPath)
			if err != nil {
				return print(cmd, o, fail("config_error", messageWithDetail("Config path could not be resolved.", err), "Set INSPECT_IMAGE_CONFIG or pass --config.", 500))
			}
			cfg, err := config.LoadOrDefault(path)
			if err != nil {
				return print(cmd, o, fail("config_error", messageWithDetail("Config file could not be loaded.", err), "Fix inspect-image.json or pass --config.", 400))
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
				return print(cmd, o, fail("config_error", messageWithDetail("Config file could not be saved.", err), "Check permissions for ~/.copilot/inspect-image.json.", 500))
			}
			return print(cmd, o, output.Success("", result))
		}}
}

func authStatusCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "status",
		Short: "Show non-secret authentication status",
		Long: strings.TrimSpace(`Read the inspect-image config and report whether Copilot authentication is configured and currently valid.

This command never prints github_access_token, copilot_token, Authorization headers, or token-derived secrets. In --json mode, missing or expired auth returns ok=false with error.code=auth_required.`),
		Example: "inspect-image auth status --json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.ResolvePath(o.ConfigPath)
			if err != nil {
				return print(cmd, o, fail("config_error", messageWithDetail("Config path could not be resolved.", err), "Set INSPECT_IMAGE_CONFIG or pass --config.", 500))
			}
			cfg, err := config.Load(path)
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return print(cmd, o, fail("config_error", messageWithDetail("Config file could not be loaded.", err), "Fix inspect-image.json or pass --config.", 400))
				}
				return print(cmd, o, fail("auth_required", "GitHub Copilot authentication is required.", "Run inspect-image auth login.", 401))
			}
			status := iauth.Summarize(cfg, time.Now())
			if !status.CopilotTokenValid {
				return print(cmd, o, fail("auth_required", "GitHub Copilot authentication is required.", "Run inspect-image auth login.", 401))
			}
			return print(cmd, o, output.Success("", status))
		}}
}

func authLogoutCmd(o *Opts) *cobra.Command {
	var yes bool
	c := &cobra.Command{Use: "logout",
		Short: "Clear stored authentication tokens",
		Long: strings.TrimSpace(`Clear GitHub and Copilot token fields from the inspect-image config while preserving api, defaults, limits, privacy, and other non-secret settings.

This command requires --yes so agents do not accidentally remove credentials during discovery.`),
		Example: "inspect-image auth logout --yes --json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return print(cmd, o, fail("invalid_args", "auth logout requires --yes.", "Re-run inspect-image auth logout --yes.", 400))
			}
			path, err := config.ResolvePath(o.ConfigPath)
			if err != nil {
				return print(cmd, o, fail("config_error", messageWithDetail("Config path could not be resolved.", err), "Set INSPECT_IMAGE_CONFIG or pass --config.", 500))
			}
			cfg, err := config.LoadOrDefault(path)
			if err != nil {
				return print(cmd, o, fail("config_error", messageWithDetail("Config file could not be loaded.", err), "Fix inspect-image.json or pass --config.", 400))
			}
			cfg = iauth.Logout(cfg)
			if err := config.Save(path, cfg); err != nil {
				return print(cmd, o, fail("config_error", messageWithDetail("Config file could not be saved.", err), "Check permissions for ~/.copilot/inspect-image.json.", 500))
			}
			return print(cmd, o, output.Success("", map[string]any{"auth_configured": false, "github_host": cfg.Auth.GitHubHost}))
		}}
	c.Flags().BoolVar(&yes, "yes", false, "Confirm that stored auth tokens should be cleared.")
	return c
}

func doctorCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "doctor",
		Short: "Check inspect-image configuration and Copilot readiness",
		Long: strings.TrimSpace(`Run local readiness checks for the config file, permissions, authentication, system proxy mode, /responses endpoint configuration, and allowed model defaults.

Doctor does not print tokens. If authentication is missing or expired, --json returns ok=false with error.code=auth_required and a hint to run auth login.`),
		Example: "inspect-image doctor --json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.ResolvePath(o.ConfigPath)
			if err != nil {
				return print(cmd, o, fail("config_error", messageWithDetail("Config path could not be resolved.", err), "Set INSPECT_IMAGE_CONFIG or pass --config.", 500))
			}
			cfg, err := config.Load(path)
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return print(cmd, o, fail("config_error", messageWithDetail("Config file could not be loaded.", err), "Fix inspect-image.json or pass --config.", 400))
				}
				return print(cmd, o, fail("auth_required", "GitHub Copilot authentication is required.", "Run inspect-image auth login.", 401))
			}
			if !iauth.TokenValid(cfg, time.Now()) {
				return print(cmd, o, fail("auth_required", "GitHub Copilot authentication is required.", "Run inspect-image auth login.", 401))
			}
			return print(cmd, o, output.Success("", map[string]any{"checks": map[string]any{
				"config":             "ok",
				"permissions":        permStatus(path),
				"auth":               "ok",
				"proxy":              "system",
				"responses_endpoint": "ok",
				"default_model":      cfg.Defaults.Model,
			}}))
		}}
}

func modelsCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "models",
		Short: "List allowed models and reasoning efforts",
		Long: strings.TrimSpace(`Print the hard-coded model and reasoning allowlists used by inspect-image.

Use this before constructing commands dynamically. Requests using other model names or reasoning efforts fail locally before any image bytes are sent.`),
		Example: "inspect-image models --json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return print(cmd, o, output.Success("", map[string]any{"default_model": config.DefaultModel, "allowed_models": config.AllowedModels, "default_reasoning": config.DefaultReasoning, "allowed_reasoning": config.AllowedReasoning}))
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
		Long: strings.TrimSpace(`Show the schema for a command. For inspect, the schema includes required parameters, model and reasoning enums, and image size and MIME type limits.

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
		Args: cobra.NoArgs,
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
			"model":     map[string]any{"type": "string", "enum": config.AllowedModels, "default": config.DefaultModel},
			"reasoning": map[string]any{"type": "string", "enum": config.AllowedReasoning, "default": config.DefaultReasoning},
		},
		"limits": map[string]any{"max_image_bytes": config.MaxImageBytes, "allowed_mime_types": config.AllowedMIMETypes},
	}
}

func inspectImageLLMTips() []string {
	return []string{
		"Always use --json.",
		"Use inspect-image inspect --image <path> --prompt <task> --json.",
		"Accepts exactly one JPEG, PNG, WEBP, or GIF image up to 3145728 bytes.",
		"Prefer this over OCR for screenshots, UI states, diagrams, charts, and visual errors.",
		"Read data.result.answer first.",
		"For OCR tasks, read data.result.visible_text.",
		"If ok=false, inspect error.code and error.hint before retrying.",
		"If auth_required, run inspect-image auth login.",
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
	var ie *inspect.InspectError
	if errors.As(err, &ie) {
		return print(cmd, o, fail(ie.Code, ie.Message, ie.Hint, ie.Status))
	}
	var ve *imagecheck.ValidationError
	if errors.As(err, &ve) {
		return print(cmd, o, fail(ve.Code, ve.Message, ve.Hint, ve.Status))
	}
	var ce *copilot.APIError
	if errors.As(err, &ce) {
		return print(cmd, o, fail(ce.Code, ce.Message, ce.Hint, ce.Status))
	}
	var ae *iauth.APIError
	if errors.As(err, &ae) {
		return print(cmd, o, fail(ae.Code, ae.Message, ae.Hint, ae.Status))
	}
	detail := config.RedactString(strings.TrimSpace(err.Error()))
	message := "inspect-image failed."
	if detail != "" {
		message += " " + detail
	}
	return print(cmd, o, fail("unknown_error", message, "Retry with --json and inspect error.code.", 500))
}

func fail(code, message, hint string, status int) output.Envelope {
	return output.Failure(code, message, hint, status)
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
