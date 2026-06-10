package commands

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"engineering-flow-platform-tools/internal/output"
	"engineering-flow-platform-tools/internal/visual/mermaid"
	"engineering-flow-platform-tools/internal/visual/metadata"

	"github.com/spf13/cobra"
)

func routePlanCmd(o *Opts) *cobra.Command {
	var inputPath, outPath string
	c := &cobra.Command{
		Use:   "route-plan",
		Short: "Compile Mermaid architecture input into the internal RoutePlan",
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputPath == "" {
				return invalidArgs(cmd, o, "--input is required", "Pass a Mermaid architecture file.")
			}
			raw, err := readVisualCommandInput(inputPath, cmd.InOrStdin())
			if err != nil {
				return print(cmd, o, failureFromError(err, "input_read_failed"))
			}
			plan, err := mermaid.GenerateRoutePlan(context.Background(), raw)
			if err != nil {
				return print(cmd, o, failureFromError(err, "route_plan_failed"))
			}
			if strings.TrimSpace(outPath) != "" {
				if err := writeJSONOutput(outPath, plan); err != nil {
					return print(cmd, o, failureFromError(err, "output_write_failed"))
				}
			}
			return print(cmd, o, output.Success("", map[string]any{
				"engine":     plan["backend"],
				"route_plan": plan,
				"out":        strings.TrimSpace(outPath),
			}))
		},
	}
	c.Flags().StringVar(&inputPath, "input", "", "Input Mermaid architecture file path, or - for stdin")
	c.Flags().StringVar(&outPath, "out", "", "Optional output JSON path for the route plan")
	return c
}

func readVisualCommandInput(path string, stdin io.Reader) ([]byte, error) {
	if strings.TrimSpace(path) == "-" {
		if stdin == nil {
			stdin = os.Stdin
		}
		b, err := io.ReadAll(stdin)
		if err != nil {
			return nil, metadata.NewError("input_read_failed", "failed to read input from stdin: "+err.Error(), "Pipe a Mermaid architecture file.", 400)
		}
		return b, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, metadata.NewError("input_read_failed", "failed to read input: "+err.Error(), "Pass a readable Mermaid architecture file.", 400)
	}
	return b, nil
}

func writeJSONOutput(path string, value any) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return metadata.NewError("output_write_failed", "failed to create output directory: "+err.Error(), "Check --out permissions.", 500)
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return metadata.NewError("output_write_failed", "failed to encode JSON output: "+err.Error(), "Inspect route plan data.", 500)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return metadata.NewError("output_write_failed", "failed to write output: "+err.Error(), "Check --out permissions.", 500)
	}
	return nil
}
