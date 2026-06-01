package inspect

import (
	"fmt"
	"os"
	"strings"
)

func ComposePrompt(preset, task string) string {
	var b strings.Builder
	b.WriteString("Inspect exactly one image.\n")
	b.WriteString("Do not invent details.\n")
	b.WriteString("Mark uncertain text as uncertain.\n")
	b.WriteString("Return structured JSON with answer, summary, visible_text, objects, ui_elements, observations, uncertainties.\n")
	if p := presetInstruction(preset); p != "" {
		b.WriteString(p)
		b.WriteString("\n")
	}
	b.WriteString("\nTask: ")
	b.WriteString(strings.TrimSpace(task))
	return b.String()
}

func ReadPrompt(prompt, promptFile string) (string, error) {
	if strings.TrimSpace(prompt) != "" {
		return strings.TrimSpace(prompt), nil
	}
	if strings.TrimSpace(promptFile) == "" {
		return "", &InspectError{Code: "prompt_required", Message: "--prompt or --prompt-file is required.", Hint: "Pass --prompt <task> or --prompt-file <path>.", Status: 400}
	}
	b, err := os.ReadFile(promptFile)
	if err != nil {
		return "", &InspectError{Code: "invalid_args", Message: fmt.Sprintf("Prompt file could not be read: %s", promptFile), Hint: "Pass a readable text file for --prompt-file.", Status: 400}
	}
	if strings.TrimSpace(string(b)) == "" {
		return "", &InspectError{Code: "prompt_required", Message: "Prompt file is empty.", Hint: "Provide a non-empty task prompt.", Status: 400}
	}
	return strings.TrimSpace(string(b)), nil
}

func presetInstruction(preset string) string {
	switch strings.ToLower(strings.TrimSpace(preset)) {
	case "", "general":
		return ""
	case "ocr":
		return "Preset: emphasize text extraction, preserving visible line breaks and uncertainty."
	case "ui":
		return "Preset: emphasize UI elements, current state, errors, and likely next action."
	case "diagram":
		return "Preset: emphasize components, relationships, and flow."
	case "chart":
		return "Preset: emphasize labels, axes, values, trends, and caveats."
	case "error":
		return "Preset: emphasize visible error, likely cause, and suggested next action."
	default:
		return "Preset: " + preset
	}
}

type InspectError struct {
	Code    string
	Message string
	Hint    string
	Status  int
}

func (e *InspectError) Error() string { return e.Code + ": " + e.Message }
