package commands

import (
	"os"
	"path/filepath"

	"engineering-flow-platform-tools/internal/output"
	visualconfig "engineering-flow-platform-tools/internal/visual/config"
	"engineering-flow-platform-tools/internal/visual/manifest"
	"engineering-flow-platform-tools/internal/visual/render"
	"github.com/spf13/cobra"
)

func templateCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{
		Use:   "template",
		Short: "Inspect local visual templates",
	}
	c.AddCommand(templateListCmd(o), templateGetCmd(o), templateDoctorCmd(o))
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

func templateDoctorCmd(o *Opts) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Validate the visual template registry, manifests, assets, and offline contract",
		RunE: func(cmd *cobra.Command, args []string) error {
			templateDir, err := visualconfig.ResolveTemplateDir(o.TemplateDir, o.Config)
			if err != nil {
				return print(cmd, o, failureFromError(err, "template_dir_missing"))
			}
			registry, err := manifest.LoadRegistry(templateDir)
			if err != nil {
				return print(cmd, o, failureFromError(err, "template_registry_missing"))
			}
			var checked []string
			for _, entry := range registry.Templates {
				if err := checkTemplateRequiredFiles(templateDir, entry); err != nil {
					return print(cmd, o, failureFromError(err, "template_manifest_invalid"))
				}
				tpl, err := manifest.LoadTemplateManifest(templateDir, entry)
				if err != nil {
					return print(cmd, o, failureFromError(err, "template_manifest_invalid"))
				}
				if err := manifest.ValidateTemplateManifest(templateDir, entry, &tpl); err != nil {
					return print(cmd, o, failureFromError(err, "template_manifest_invalid"))
				}
				checked = append(checked, entry.ID)
			}
			if o.OfflineStrict {
				if err := render.ScanOffline(templateDir); err != nil {
					return print(cmd, o, failureFromError(err, "offline_violation"))
				}
			}
			return print(cmd, o, output.Success("", map[string]any{
				"template_dir":      templateDir,
				"registry_version":  registry.Version,
				"checked_templates": len(checked),
				"templates":         checked,
				"offline_strict":    o.OfflineStrict,
			}))
		},
	}
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

func outputFileError(code, message, hint string) error {
	return visualCommandError{code: code, message: message, hint: hint, status: 400}
}

type visualCommandError struct {
	code    string
	message string
	hint    string
	status  int
}

func (e visualCommandError) Error() string   { return e.message }
func (e visualCommandError) Code() string    { return e.code }
func (e visualCommandError) Message() string { return e.message }
func (e visualCommandError) Hint() string    { return e.hint }
func (e visualCommandError) Status() int     { return e.status }
