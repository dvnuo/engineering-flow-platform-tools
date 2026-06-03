package commands

import (
	"io"
	"os"
	"strings"

	"engineering-flow-platform-tools/internal/output"
	"engineering-flow-platform-tools/internal/visual/config"
	"engineering-flow-platform-tools/internal/visual/manifest"
	"engineering-flow-platform-tools/internal/visual/metadata"
	visualschema "engineering-flow-platform-tools/internal/visual/schema"
	"github.com/spf13/cobra"
)

func validateCmd(o *Opts) *cobra.Command {
	var templateID, inputPath string
	c := &cobra.Command{
		Use:   "validate",
		Short: "Validate visual input JSON against a template contract",
		RunE: func(cmd *cobra.Command, args []string) error {
			if templateID == "" {
				return invalidArgs(cmd, o, "--template is required", "Run visual template list --json and pass --template <template-id>.")
			}
			if inputPath == "" {
				return invalidArgs(cmd, o, "--input is required", "Pass --input <file> or --input - to read JSON from stdin.")
			}
			templateDir, err := config.ResolveTemplateDir(o.TemplateDir, o.Config)
			if err != nil {
				return print(cmd, o, failureFromError(err, "template_dir_missing"))
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
			raw, err := readValidationInput(cmd, inputPath)
			if err != nil {
				return print(cmd, o, failureFromError(err, "input_read_failed"))
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
	c.Flags().StringVar(&templateID, "template", "", "Template id from visual template list")
	c.Flags().StringVar(&inputPath, "input", "", "Input JSON file path, or - for stdin")
	return c
}

func readValidationInput(cmd *cobra.Command, inputPath string) ([]byte, error) {
	if strings.TrimSpace(inputPath) == "-" {
		b, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return nil, metadata.NewError("input_read_failed", "failed to read input JSON from stdin: "+err.Error(), "Pipe valid JSON to visual validate --input -.", 400)
		}
		return b, nil
	}
	b, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, metadata.NewError("input_read_failed", "failed to read input JSON: "+err.Error(), "Pass a readable JSON file path to --input.", 400)
	}
	return b, nil
}
