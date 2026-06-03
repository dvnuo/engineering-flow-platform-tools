package commands

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"engineering-flow-platform-tools/internal/output"
	visualconfig "engineering-flow-platform-tools/internal/visual/config"
	"engineering-flow-platform-tools/internal/visual/manifest"
	"engineering-flow-platform-tools/internal/visual/render"
	visualschema "engineering-flow-platform-tools/internal/visual/schema"
	"github.com/spf13/cobra"
)

func templateCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{
		Use:   "template",
		Short: "Inspect local visual templates",
	}
	c.AddCommand(templateListCmd(o), templateGetCmd(o), templateSchemaCmd(o), templateDoctorCmd(o))
	return c
}

func templateListCmd(o *Opts) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List visual templates from registry.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			templateDir, err := visualconfig.ResolveTemplateDir(o.TemplateDir, o.Config)
			if err != nil {
				return print(cmd, o, failureFromError(err, "template_dir_missing"))
			}
			registry, err := manifest.LoadRegistry(templateDir)
			if err != nil {
				return print(cmd, o, failureFromError(err, "template_registry_missing"))
			}
			return print(cmd, o, output.Success("", map[string]any{
				"template_dir": templateDir,
				"version":      registry.Version,
				"templates":    registry.Templates,
			}))
		},
	}
}

func templateGetCmd(o *Opts) *cobra.Command {
	return &cobra.Command{
		Use:   "get <template-id>",
		Short: "Show one visual template manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			templateDir, err := visualconfig.ResolveTemplateDir(o.TemplateDir, o.Config)
			if err != nil {
				return print(cmd, o, failureFromError(err, "template_dir_missing"))
			}
			registry, err := manifest.LoadRegistry(templateDir)
			if err != nil {
				return print(cmd, o, failureFromError(err, "template_registry_missing"))
			}
			entry, ok := registry.Find(args[0])
			if !ok {
				return print(cmd, o, output.Failure("template_not_found", "visual template was not found: "+args[0], "Run visual template list --json and choose one of the returned ids.", 404))
			}
			tpl, err := manifest.LoadTemplateManifest(templateDir, entry)
			if err != nil {
				return print(cmd, o, failureFromError(err, "template_manifest_invalid"))
			}
			if err := manifest.ValidateTemplateManifest(templateDir, entry, &tpl); err != nil {
				return print(cmd, o, failureFromError(err, "template_manifest_invalid"))
			}
			return print(cmd, o, output.Success("", map[string]any{
				"template_dir":      templateDir,
				"registry":          entry,
				"template":          tpl,
				"id":                tpl.ID,
				"version":           tpl.Version,
				"renderer":          tpl.Renderer,
				"input_schema_kind": tpl.InputSchemaKind,
			}))
		},
	}
}

func templateSchemaCmd(o *Opts) *cobra.Command {
	return &cobra.Command{
		Use:   "schema <template_id>",
		Short: "Show one visual template input JSON schema and example",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			templateDir, err := visualconfig.ResolveTemplateDir(o.TemplateDir, o.Config)
			if err != nil {
				return print(cmd, o, failureFromError(err, "template_dir_missing"))
			}
			registry, err := manifest.LoadRegistry(templateDir)
			if err != nil {
				return print(cmd, o, failureFromError(err, "template_registry_missing"))
			}
			entry, ok := registry.Find(args[0])
			if !ok {
				return print(cmd, o, output.Failure("template_not_found", "Template "+args[0]+" was not found.", "Run visual template list --template-dir "+templateDir+" --json.", 404))
			}
			tpl, err := manifest.LoadTemplateManifest(templateDir, entry)
			if err != nil {
				return print(cmd, o, failureFromError(err, "template_manifest_invalid"))
			}
			if err := manifest.ValidateTemplateManifest(templateDir, entry, &tpl); err != nil {
				return print(cmd, o, failureFromError(err, "template_manifest_invalid"))
			}
			doc, schemaFile, err := manifest.LoadTemplateInputSchema(templateDir, entry, tpl)
			if err != nil {
				return print(cmd, o, failureFromError(err, "template_manifest_invalid"))
			}
			return print(cmd, o, output.Success("", map[string]any{
				"template": map[string]any{
					"id":                tpl.ID,
					"version":           tpl.Version,
					"title":             tpl.Title,
					"renderer":          tpl.Renderer.Contract,
					"input_schema_kind": tpl.InputSchemaKind,
				},
				"schema_file":  schemaFile,
				"json_schema":  doc.JSONSchema,
				"example_file": templateExampleRel(entry),
				"example":      doc.Example,
			}))
		},
	}
}

func templateDoctorCmd(o *Opts) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Validate the visual template registry, manifests, schemas, examples, and offline contract",
		RunE: func(cmd *cobra.Command, args []string) error {
			templateDir, err := visualconfig.ResolveTemplateDir(o.TemplateDir, o.Config)
			if err != nil {
				return print(cmd, o, failureFromError(err, "template_dir_missing"))
			}
			registry, err := manifest.LoadRegistry(templateDir)
			if err != nil {
				return print(cmd, o, failureFromError(err, "template_registry_missing"))
			}
			var checked []doctorTemplateResult
			checkedExamples := 0
			renderedExamples := 0
			for _, entry := range registry.Templates {
				tpl, err := manifest.LoadTemplateManifest(templateDir, entry)
				if err != nil {
					return print(cmd, o, failureFromError(withTemplateContext(err, entry.ID, filepath.ToSlash(entry.Path)), "template_manifest_invalid"))
				}
				if err := manifest.ValidateTemplateManifest(templateDir, entry, &tpl); err != nil {
					return print(cmd, o, failureFromError(withTemplateContext(err, entry.ID, filepath.ToSlash(entry.Path)), "template_manifest_invalid"))
				}
				_, schemaFile, err := manifest.LoadTemplateInputSchema(templateDir, entry, tpl)
				if err != nil {
					return print(cmd, o, failureFromError(withTemplateContext(err, entry.ID, schemaFile), "template_manifest_invalid"))
				}
				examplePath := templateExamplePath(templateDir, entry)
				exampleRel := templateExampleRel(entry)
				raw, err := os.ReadFile(examplePath)
				if err != nil {
					return print(cmd, o, failureFromError(visualCommandError{
						code:       "template_manifest_invalid",
						message:    "visual template example was not found: " + exampleRel,
						hint:       "Add examples/basic.input.json for " + entry.ID + ".",
						status:     400,
						templateID: entry.ID,
						file:       exampleRel,
					}, "template_manifest_invalid"))
				}
				if _, err := visualschema.ValidateInput(tpl.InputSchemaKind, raw, tpl.Limits); err != nil {
					return print(cmd, o, failureFromError(withTemplateContext(err, entry.ID, exampleRel), "template_input_invalid"))
				}
				checkedExamples++
				if err := renderDoctorExample(templateDir, entry, examplePath); err != nil {
					return print(cmd, o, failureFromError(withTemplateContext(err, entry.ID, exampleRel), "output_write_failed"))
				}
				renderedExamples++
				checked = append(checked, doctorTemplateResult{
					ID:              tpl.ID,
					Version:         tpl.Version,
					InputSchemaKind: tpl.InputSchemaKind,
					Example:         exampleRel,
					Rendered:        true,
				})
			}
			return print(cmd, o, output.Success("", map[string]any{
				"template_dir":      templateDir,
				"registry_version":  registry.Version,
				"checked_templates": len(checked),
				"checked_examples":  checkedExamples,
				"rendered_examples": renderedExamples,
				"offline":           true,
				"offline_strict":    o.OfflineStrict,
				"templates":         checked,
			}))
		},
	}
}

type doctorTemplateResult struct {
	ID              string `json:"id"`
	Version         string `json:"version"`
	InputSchemaKind string `json:"input_schema_kind"`
	Example         string `json:"example"`
	Rendered        bool   `json:"rendered"`
}

func checkTemplateRequiredFiles(templateDir string, entry manifest.RegistryEntry) error {
	templateBase := filepath.Dir(filepath.Join(templateDir, filepath.Clean(entry.Path)))
	for _, rel := range []string{"template.yaml", "schema.input.json", "style.css", filepath.Join("examples", "basic.input.json")} {
		path := filepath.Join(templateBase, rel)
		info, err := os.Stat(path)
		if err != nil || info.IsDir() || info.Size() == 0 {
			return outputFileError("template_manifest_invalid", "visual template required file is missing or empty: "+filepath.ToSlash(filepath.Join(entry.ID, rel)), "Add non-empty template.yaml, schema.input.json, style.css, and examples/basic.input.json files.")
		}
	}
	return nil
}

func renderDoctorExample(templateDir string, entry manifest.RegistryEntry, examplePath string) error {
	tempDir, err := os.MkdirTemp("", "efp-visual-doctor-"+safeTempName(entry.ID)+"-")
	if err != nil {
		return visualCommandError{code: "output_write_failed", message: "failed to create temporary visual doctor directory: " + err.Error(), hint: "Check temporary directory permissions.", status: 500}
	}
	defer os.RemoveAll(tempDir)
	outDir := filepath.Join(tempDir, "artifact")
	if _, err := render.Render(render.Options{
		TemplateDir:   templateDir,
		TemplateID:    entry.ID,
		InputPath:     examplePath,
		OutDir:        outDir,
		DataMode:      "js-file",
		OfflineStrict: true,
	}); err != nil {
		return err
	}
	if err := checkRenderedOutputFiles(outDir); err != nil {
		return err
	}
	return render.ScanOffline(outDir)
}

func checkRenderedOutputFiles(outDir string) error {
	required := []string{
		"index.html",
		"manifest.json",
		"manifest.js",
		"data.js",
		"assets/runtime/efp-visual-runtime.iife.js",
		"assets/runtime/efp-visual-renderers.iife.js",
		"assets/runtime/efp-visual-runtime.css",
	}
	var missing []string
	for _, rel := range required {
		info, err := os.Stat(filepath.Join(outDir, rel))
		if err != nil || info.IsDir() {
			missing = append(missing, rel)
		}
	}
	if len(missing) > 0 {
		return visualCommandError{
			code:         "visual_output_invalid",
			message:      "Visual output directory is missing required files.",
			hint:         "Run visual render again or inspect the template assets.",
			status:       400,
			missingFiles: missing,
		}
	}
	return nil
}

func templateExamplePath(templateDir string, entry manifest.RegistryEntry) string {
	return filepath.Join(manifest.TemplateBaseDir(templateDir, entry), "examples", "basic.input.json")
}

func templateExampleRel(entry manifest.RegistryEntry) string {
	return filepath.ToSlash(filepath.Join(filepath.Dir(filepath.Clean(entry.Path)), "examples", "basic.input.json"))
}

func safeTempName(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('-')
	}
	if b.Len() == 0 {
		return "template"
	}
	return b.String()
}

func outputFileError(code, message, hint string) error {
	return visualCommandError{code: code, message: message, hint: hint, status: 400}
}

func withTemplateContext(err error, templateID, file string) error {
	if err == nil {
		return nil
	}
	var ce codedError
	if strings.TrimSpace(file) != "" && errors.As(err, &ce) {
		return visualCommandError{
			code:         ce.Code(),
			message:      ce.Message(),
			hint:         ce.Hint(),
			status:       ce.Status(),
			templateID:   templateID,
			file:         file,
			missingFiles: missingFilesFromError(err),
		}
	}
	if errors.As(err, &ce) {
		return visualCommandError{
			code:         ce.Code(),
			message:      ce.Message(),
			hint:         ce.Hint(),
			status:       ce.Status(),
			templateID:   templateID,
			missingFiles: missingFilesFromError(err),
		}
	}
	return visualCommandError{code: "template_manifest_invalid", message: err.Error(), hint: "Inspect the template manifest, schema, and example files.", status: 400, templateID: templateID, file: file}
}

func missingFilesFromError(err error) []string {
	var me missingFilesError
	if errors.As(err, &me) {
		return me.MissingFiles()
	}
	return nil
}

type visualCommandError struct {
	code         string
	message      string
	hint         string
	status       int
	templateID   string
	file         string
	missingFiles []string
}

func (e visualCommandError) Error() string   { return e.message }
func (e visualCommandError) Code() string    { return e.code }
func (e visualCommandError) Message() string { return e.message }
func (e visualCommandError) Hint() string    { return e.hint }
func (e visualCommandError) Status() int     { return e.status }
func (e visualCommandError) TemplateID() string {
	return e.templateID
}
func (e visualCommandError) File() string {
	return e.file
}
func (e visualCommandError) MissingFiles() []string {
	return append([]string{}, e.missingFiles...)
}
