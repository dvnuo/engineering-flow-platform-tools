package copilot

import (
	"context"
	"encoding/base64"

	"engineering-flow-platform-tools/internal/inspectimage/config"
	"engineering-flow-platform-tools/internal/inspectimage/imagecheck"
)

type ResponsesRequest struct {
	Model           string              `json:"model"`
	Instructions    string              `json:"instructions,omitempty"`
	Reasoning       ResponsesReasoning  `json:"reasoning"`
	Input           []ResponsesInput    `json:"input"`
	MaxOutputTokens int                 `json:"max_output_tokens,omitempty"`
	Text            ResponsesTextFormat `json:"text,omitempty"`
}

type ResponsesReasoning struct {
	Effort string `json:"effort"`
}

type ResponsesTextFormat struct {
	Format map[string]string `json:"format,omitempty"`
}

type ResponsesInput struct {
	Role    string             `json:"role"`
	Content []ResponsesContent `json:"content"`
}

type ResponsesContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

func BuildRequest(model, reasoning, prompt string, img imagecheck.ImageInfo) (ResponsesRequest, error) {
	if !config.StringAllowed(model, config.AllowedModels) {
		return ResponsesRequest{}, &APIError{Code: "model_not_allowed", Message: "Model is not allowed for inspect-image.", Hint: "Run inspect-image models --json and choose an allowed model.", Status: 400}
	}
	if !config.StringAllowed(reasoning, config.AllowedReasoning) {
		return ResponsesRequest{}, &APIError{Code: "reasoning_not_allowed", Message: "Reasoning effort is not allowed for inspect-image.", Hint: "Use one of: low, medium, high, xhigh.", Status: 400}
	}
	dataURL := "data:" + img.MIMEType + ";base64," + base64.StdEncoding.EncodeToString(img.Data)
	return ResponsesRequest{
		Model:           model,
		Instructions:    "Inspect exactly one local image. Return concise structured JSON. Do not invent details. Mark unreadable text as uncertain.",
		Reasoning:       ResponsesReasoning{Effort: reasoning},
		MaxOutputTokens: 4096,
		Text:            ResponsesTextFormat{Format: map[string]string{"type": "text"}},
		Input: []ResponsesInput{{
			Role: "user",
			Content: []ResponsesContent{
				{Type: "input_text", Text: prompt},
				{Type: "input_image", ImageURL: dataURL},
			},
		}},
	}, nil
}

func (c *Client) Responses(ctx context.Context, req ResponsesRequest) (map[string]any, error) {
	var raw map[string]any
	if err := c.postJSON(ctx, "/responses", req, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}
