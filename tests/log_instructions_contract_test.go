package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogAgentInstructionsAreDocumented(t *testing.T) {
	root := filepath.Join("..")
	instructionPath := filepath.Join(root, "cmd", "log", "log-cli.instructions.md")
	instructions, err := os.ReadFile(instructionPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(instructions)
	for _, want := range []string{
		"# Log CLI Instructions for Agents",
		"Always use `--json`",
		"Do not use MCP",
		"Do not use `cat`",
		"log analyze --source <path> --run <run-dir> --json",
		"log window --file <path> --line <line>` only for files already present in that run's `manifest.json`",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing instruction %q in %s", want, instructionPath)
		}
	}

	llmUsagePath := filepath.Join(root, "docs", "LLM_USAGE.md")
	llmUsage, err := os.ReadFile(llmUsagePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(llmUsage), "cmd/log/log-cli.instructions.md") {
		t.Fatalf("LLM usage docs do not reference log instructions")
	}
}
