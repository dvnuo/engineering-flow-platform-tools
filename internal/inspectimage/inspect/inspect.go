package inspect

import (
	"context"
	"time"

	"engineering-flow-platform-tools/internal/inspectimage/aiplatform"
	"engineering-flow-platform-tools/internal/inspectimage/config"
	"engineering-flow-platform-tools/internal/inspectimage/copilot"
	"engineering-flow-platform-tools/internal/inspectimage/imagecheck"
	"engineering-flow-platform-tools/internal/inspectimage/vision"
)

type ResponsesClient interface {
	Responses(ctx context.Context, req vision.Request) (map[string]any, error)
}

func Run(ctx context.Context, cfg config.Config, client ResponsesClient, opts Options) (Result, error) {
	cfg.FillDefaults()
	model := opts.Model
	if model == "" {
		model = cfg.Defaults.Model
	}
	reasoning := opts.Reasoning
	if reasoning == "" {
		reasoning = cfg.Defaults.Reasoning
	}
	task, err := ReadPrompt(opts.Prompt, opts.PromptFile)
	if err != nil {
		return Result{}, err
	}
	img, warnings, err := imagecheck.Validate(opts.ImagePath, cfg.Limits.MaxImageBytes, cfg.Limits.AllowedMIMETypes)
	if err != nil {
		return Result{}, err
	}
	req, err := vision.BuildRequest(model, reasoning, ComposePrompt(opts.Preset, task), img)
	if err != nil {
		return Result{}, err
	}
	if client == nil {
		timeout := time.Duration(cfg.API.TimeoutSeconds) * time.Second
		switch cfg.Provider {
		case config.ProviderAIPlatform:
			client = aiplatform.NewClient(cfg, timeout)
		case config.ProviderGitHubCopilot:
			client = &copilot.Client{BaseURL: cfg.API.BaseURL, Token: cfg.Auth.CopilotToken, HTTPClient: copilot.NewHTTPClient(timeout)}
		default:
			return Result{}, &copilot.APIError{Code: "config_error", Message: "Unsupported inspect-image provider: " + cfg.Provider, Hint: "Use provider ai_platform or github_copilot_plugin.", Status: 400}
		}
	}
	raw, err := client.Responses(ctx, req)
	if err != nil {
		return Result{}, err
	}
	parsed, err := vision.ParseResponse(raw)
	if err != nil {
		return Result{}, err
	}
	warnings = append(warnings, parsed.Warnings...)
	img.Data = nil
	return Result{
		Tool:      "inspect_image",
		Provider:  cfg.Provider,
		Model:     model,
		Reasoning: reasoning,
		Image:     img,
		Result:    parsed.Result,
		Warnings:  warnings,
	}, nil
}
