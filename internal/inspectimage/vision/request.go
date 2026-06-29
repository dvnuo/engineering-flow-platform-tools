package vision

import (
	"encoding/base64"

	"engineering-flow-platform-tools/internal/inspectimage/imagecheck"
)

type Request struct {
	Model           string     `json:"model"`
	Instructions    string     `json:"instructions,omitempty"`
	Reasoning       Reasoning  `json:"reasoning"`
	Input           []Input    `json:"input"`
	MaxOutputTokens int        `json:"max_output_tokens,omitempty"`
	Text            TextFormat `json:"text,omitempty"`
}

type Reasoning struct {
	Effort string `json:"effort"`
}

type TextFormat struct {
	Format map[string]string `json:"format,omitempty"`
}

type Input struct {
	Role    string    `json:"role"`
	Content []Content `json:"content"`
}

type Content struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

type Error struct {
	Code    string
	Message string
	Hint    string
	Status  int
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func BuildRequest(model, reasoning, prompt string, img imagecheck.ImageInfo) (Request, error) {
	if reasoning == "" {
		reasoning = "medium"
	}
	if !stringAllowed(reasoning, []string{"low", "medium", "high", "xhigh"}) {
		return Request{}, &Error{Code: "reasoning_not_allowed", Message: "Reasoning effort is not allowed for inspect-image.", Hint: "Use one of: low, medium, high, xhigh.", Status: 400}
	}
	dataURL := "data:" + img.MIMEType + ";base64," + base64.StdEncoding.EncodeToString(img.Data)
	return Request{
		Model:           model,
		Instructions:    "Inspect exactly one local image. Return concise structured JSON. Do not invent details. Mark unreadable text as uncertain.",
		Reasoning:       Reasoning{Effort: reasoning},
		MaxOutputTokens: 4096,
		Text:            TextFormat{Format: map[string]string{"type": "text"}},
		Input: []Input{{
			Role: "user",
			Content: []Content{
				{Type: "input_text", Text: prompt},
				{Type: "input_image", ImageURL: dataURL},
			},
		}},
	}, nil
}

func stringAllowed(v string, allowed []string) bool {
	for _, item := range allowed {
		if v == item {
			return true
		}
	}
	return false
}
