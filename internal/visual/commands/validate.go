package commands

import (
	"io"
	"os"
	"strings"

	"engineering-flow-platform-tools/internal/output"
	"engineering-flow-platform-tools/internal/visual/config"
	"engineering-flow-platform-tools/internal/visual/manifest"
	"engineering-flow-platform-tools/internal/visual/mermaid"
	"engineering-flow-platform-tools/internal/visual/metadata"
	visualschema "engineering-flow-platform-tools/internal/visual/schema"
	"github.com/spf13/cobra"
)

func validateCmd(o *Opts) *cobra.Command {
	var templateID, inputPath string
	c := &cobra.Command{
		Use:   "validate",
		Short: "Validate Mermaid input against a visual template contract",
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputPath == "" {
				return invalidArgs(cmd, o, "--input is required", "Pass --input <file> or --input - to read Mermaid from stdin.")
			}
			templateDir, err := config.ResolveTemplateDir(o.TemplateDir, o.Config)
			if err != nil {
				return print(cmd, o, failureFromError(err, "template_dir_missing"))
			}
			raw, err := readValidationInput(cmd, inputPath)
			if err != nil {
				return print(cmd, o, failureFromError(err, "input_read_failed"))
			}
			if strings.TrimSpace(templateID) == "" {
				inferred, ok := mermaid.InferTemplateID(raw)
				if !ok {
					return print(cmd, o, failureFromError(metadata.NewError("mermaid_input_required", "visual validate accepts Mermaid input.", "Pass a valid Mermaid .mmd file.", 400), "mermaid_input_required"))
				}
				templateID = inferred
			}
			registry, err := manifest.LoadRegistry(templateDir)
			if err != nil {
				return print(cmd, o, failureFromError(err, "template_registry_missing"))
			}
			entry, ok := registry.Find(templateID)
			if !ok {
				return print(cmd, o, output.Failure("template_not_found", "visual template was not found: "+templateID, "Run visual template list --json and choose one of the returned ids.", 404))
			}
			tpl, err := manifest.LoadTemplateManifest(templateDir, entry)
			if err != nil {
				return print(cmd, o, failureFromError(err, "template_manifest_invalid"))
			}
			if err := manifest.ValidateTemplateManifest(templateDir, entry, &tpl); err != nil {
				return print(cmd, o, failureFromError(err, "template_manifest_invalid"))
			}
			if !mermaid.IsMermaid(raw) {
				return print(cmd, o, failureFromError(metadata.NewError("mermaid_input_required", "visual validate accepts Mermaid input for public templates.", "Pass a valid Mermaid .mmd file or omit --template so the CLI can infer the Mermaid template.", 400), "mermaid_input_required"))
			}
			raw, err = mermaid.CompileIfNeeded(tpl.InputSchemaKind, raw)
			if err != nil {
				return print(cmd, o, failureFromError(err, "template_input_invalid"))
			}
			parsed, err := visualschema.ValidateInput(tpl.InputSchemaKind, raw, tpl.Limits)
			if err != nil {
				return print(cmd, o, failureFromError(err, "template_input_invalid"))
			}
			return print(cmd, o, output.Success("", map[string]any{
				"template_id":   tpl.ID,
				"template_dir":  templateDir,
				"input_summary": parsed.Summary,
			}))
		},
	}
	c.Flags().StringVar(&templateID, "template", "", "Template id from visual template list; optional for Mermaid input")
	c.Flags().StringVar(&inputPath, "input", "", "Input Mermaid file path, or - for stdin")
	return c
}

func readValidationInput(cmd *cobra.Command, inputPath string) ([]byte, error) {
	if strings.TrimSpace(inputPath) == "-" {
		b, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return nil, metadata.NewError("input_read_failed", "failed to read visual input from stdin: "+err.Error(), "Pipe valid Mermaid to visual validate --input -.", 400)
		}
		return b, nil
	}
	b, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, metadata.NewError("input_read_failed", "failed to read visual input: "+err.Error(), "Pass a readable Mermaid file path to --input.", 400)
	}
	return b, nil
}
