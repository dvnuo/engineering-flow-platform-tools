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
	c := &cobra.Command{Use: "inspect-image", SilenceErrors: true, SilenceUsage: true}
	c.PersistentFlags().BoolVar(&o.JSON, "json", false, "")
	c.PersistentFlags().StringVar(&o.Format, "format", "table", "")
	c.PersistentFlags().BoolVar(&o.Verbose, "verbose", false, "")
	c.PersistentFlags().StringVar(&o.ConfigPath, "config", "", "")
	c.AddCommand(inspectCmd(o, client), authCmd(o), doctorCmd(o), modelsCmd(o), commandsCmd(o), schemaCmd(o), helpLLMCmd(o), versionCmd(o))
	return c
}

func inspectCmd(o *Opts, client inspect.ResponsesClient) *cobra.Command {
	opts := inspect.Options{}
	c := &cobra.Command{Use: "inspect", RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 && opts.ImagePath == "" {
			opts.ImagePath = args[0]
		}
		cfgPath, err := config.ResolvePath(o.ConfigPath)
		if err != nil {
			return print(cmd, o, fail("config_error", "Config path could not be resolved.", "Set INSPECT_IMAGE_CONFIG or pass --config.", 500))
		}
		cfg, err := config.LoadOrDefault(cfgPath)
		if err != nil {
			return print(cmd, o, fail("config_error", "Config file could not be loaded.", "Fix inspect-image.json or pass --config.", 400))
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
	c.Flags().StringVar(&opts.ImagePath, "image", "", "")
	c.Flags().StringVar(&opts.Prompt, "prompt", "", "")
	c.Flags().StringVar(&opts.PromptFile, "prompt-file", "", "")
	c.Flags().StringVar(&opts.Model, "model", config.DefaultModel, "")
	c.Flags().StringVar(&opts.Reasoning, "reasoning", config.DefaultReasoning, "")
	c.Flags().StringVar(&opts.Preset, "preset", "general", "")
	c.Flags().IntVar(&opts.TimeoutSecond, "timeout", 0, "")
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
	if iauth.TokenValid(cfg, time.Now()) {
		return cfg, nil
	}
	if cfg.Auth.CopilotToken == "" && cfg.Auth.GitHubAccessToken == "" {
		return cfg, &copilot.APIError{Code: "auth_required", Message: "GitHub Copilot authentication is required.", Hint: "Run inspect-image auth login.", Status: 401}
	}
	if cfg.Auth.GitHubAccessToken == "" {
		return cfg, &copilot.APIError{Code: "auth_expired", Message: "GitHub Copilot authentication expired.", Hint: "Run inspect-image auth login.", Status: 401}
	}
	client := &iauth.DeviceClient{HTTPClient: copilot.NewHTTPClient(time.Duration(cfg.API.TimeoutSeconds) * time.Second)}
	token, expires, err := client.ExchangeCopilotToken(ctx, cfg.Auth.GitHubAccessToken)
	if err != nil {
		return cfg, err
	}
	cfg.Auth.CopilotToken = token
	cfg.Auth.CopilotTokenExpiresAt = expires.UTC().Format(time.RFC3339)
	cfg.Auth.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := config.Save(path, cfg); err != nil {
		return cfg, &copilot.APIError{Code: "config_error", Message: "Config file could not be saved.", Hint: "Check permissions for ~/.copilot/inspect-image.json.", Status: 500}
	}
	return cfg, nil
}

func authCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "auth"}
	c.AddCommand(authLoginCmd(o), authStatusCmd(o), authLogoutCmd(o))
	return c
}

func authLoginCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "login", RunE: func(cmd *cobra.Command, args []string) error {
		path, err := config.ResolvePath(o.ConfigPath)
		if err != nil {
			return print(cmd, o, fail("config_error", "Config path could not be resolved.", "Set INSPECT_IMAGE_CONFIG or pass --config.", 500))
		}
		cfg, err := config.LoadOrDefault(path)
		if err != nil {
			return print(cmd, o, fail("config_error", "Config file could not be loaded.", "Fix inspect-image.json or pass --config.", 400))
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
			return print(cmd, o, fail("config_error", "Config file could not be saved.", "Check permissions for ~/.copilot/inspect-image.json.", 500))
		}
		return print(cmd, o, output.Success("", result))
	}}
}

func authStatusCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "status", RunE: func(cmd *cobra.Command, args []string) error {
		path, err := config.ResolvePath(o.ConfigPath)
		if err != nil {
			return print(cmd, o, fail("config_error", "Config path could not be resolved.", "Set INSPECT_IMAGE_CONFIG or pass --config.", 500))
		}
		cfg, err := config.Load(path)
		if err != nil {
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
	c := &cobra.Command{Use: "logout", RunE: func(cmd *cobra.Command, args []string) error {
		if !yes {
			return print(cmd, o, fail("invalid_args", "auth logout requires --yes.", "Re-run inspect-image auth logout --yes.", 400))
		}
		path, err := config.ResolvePath(o.ConfigPath)
		if err != nil {
			return print(cmd, o, fail("config_error", "Config path could not be resolved.", "Set INSPECT_IMAGE_CONFIG or pass --config.", 500))
		}
		cfg, err := config.LoadOrDefault(path)
		if err != nil {
			return print(cmd, o, fail("config_error", "Config file could not be loaded.", "Fix inspect-image.json or pass --config.", 400))
		}
		cfg = iauth.Logout(cfg)
		if err := config.Save(path, cfg); err != nil {
			return print(cmd, o, fail("config_error", "Config file could not be saved.", "Check permissions for ~/.copilot/inspect-image.json.", 500))
		}
		return print(cmd, o, output.Success("", map[string]any{"auth_configured": false, "github_host": cfg.Auth.GitHubHost}))
	}}
	c.Flags().BoolVar(&yes, "yes", false, "")
	return c
}

func doctorCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "doctor", RunE: func(cmd *cobra.Command, args []string) error {
		path, err := config.ResolvePath(o.ConfigPath)
		if err != nil {
			return print(cmd, o, fail("config_error", "Config path could not be resolved.", "Set INSPECT_IMAGE_CONFIG or pass --config.", 500))
		}
		cfg, err := config.Load(path)
		if err != nil {
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
	return &cobra.Command{Use: "models", RunE: func(cmd *cobra.Command, args []string) error {
		return print(cmd, o, output.Success("", map[string]any{"default_model": config.DefaultModel, "allowed_models": config.AllowedModels, "default_reasoning": config.DefaultReasoning, "allowed_reasoning": config.AllowedReasoning}))
	}}
}

func commandsCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "commands", RunE: func(cmd *cobra.Command, args []string) error {
		return print(cmd, o, output.Success("", map[string]any{"commands": catalog.Commands("inspect-image")}))
	}}
}

func schemaCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "schema <command>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if args[0] == "inspect" {
			return print(cmd, o, output.Success("", inspectSchema()))
		}
		return print(cmd, o, output.Success("", catalog.Schema("inspect-image", args[0])))
	}}
}

func versionCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "version", RunE: func(cmd *cobra.Command, args []string) error {
		return print(cmd, o, output.Success("", map[string]any{"version": version.Version, "commit": version.Commit, "date": version.Date}))
	}}
}

func helpLLMCmd(o *Opts) *cobra.Command {
	return &cobra.Command{Use: "help llm", RunE: func(cmd *cobra.Command, args []string) error {
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
	return print(cmd, o, fail("unknown_error", "inspect-image failed.", "Retry with --json and inspect error.code.", 500))
}

func fail(code, message, hint string, status int) output.Envelope {
	return output.Failure(code, message, hint, status)
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
