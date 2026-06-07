package commands

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"engineering-flow-platform-tools/internal/output"
	"engineering-flow-platform-tools/internal/visual/authoring"
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
	c.AddCommand(templateCategoriesCmd(o), templateListCmd(o), templateGetCmd(o), templateSchemaCmd(o), templateGuideCmd(o), templatePanelGrammarCmd(o), templateDoctorCmd(o))
	return c
}

func templateCategoriesCmd(o *Opts) *cobra.Command {
	return &cobra.Command{
		Use:   "categories",
		Short: "List visual template categories and canonical counts",
		RunE: func(cmd *cobra.Command, args []string) error {
			templateDir, err := visualconfig.ResolveTemplateDir(o.TemplateDir, o.Config)
			if err != nil {
				return print(cmd, o, failureFromError(err, "template_dir_missing"))
			}
			registry, err := manifest.LoadRegistry(templateDir)
			if err != nil {
				return print(cmd, o, failureFromError(err, "template_registry_missing"))
			}
			counts := registry.CategoryCounts()
			return print(cmd, o, output.Success("", map[string]any{
				"template_dir":     templateDir,
				"categories":       manifest.SortedCategoryCounts(counts),
				"canonical_count":  registry.CanonicalCount(),
				"total_count":      registry.TotalCount(),
				"alias_count":      registry.AliasCount(),
				"registry_version": registry.Version,
			}))
		},
	}
}

func templateListCmd(o *Opts) *cobra.Command {
	var category, query, renderer, schemaKind string
	c := &cobra.Command{
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
			templates := filterRegistryTemplates(registry.Templates, templateListFilter{
				Category:   category,
				Query:      query,
				Renderer:   renderer,
				SchemaKind: schemaKind,
			})
			return print(cmd, o, output.Success("", map[string]any{
				"template_dir":    templateDir,
				"version":         registry.Version,
				"canonical_count": registry.CanonicalCount(),
				"total_count":     registry.TotalCount(),
				"alias_count":     registry.AliasCount(),
				"matched_count":   len(templates),
				"categories":      manifest.SortedCategoryCounts(registry.CategoryCounts()),
				"filters":         normalizedTemplateListFilter(category, query, renderer, schemaKind),
				"templates":       templates,
			}))
		},
	}
	c.Flags().StringVar(&category, "category", "", "Filter templates by category")
	c.Flags().StringVar(&query, "query", "", "Filter templates by id, title, description, or tag")
	c.Flags().StringVar(&renderer, "renderer", "", "Filter templates by renderer contract")
	c.Flags().StringVar(&schemaKind, "schema-kind", "", "Filter templates by input schema kind")
	return c
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
			entry, requestedID, ok := registry.Resolve(args[0])
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
			_, schemaFile, err := manifest.LoadTemplateInputSchema(templateDir, entry, tpl)
			if err != nil {
				return print(cmd, o, failureFromError(err, "template_manifest_invalid"))
			}
			return print(cmd, o, output.Success("", map[string]any{
				"template_dir":            templateDir,
				"requested_id":            requestedID,
				"canonical_id":            tpl.ID,
				"registry":                entry,
				"template":                tpl,
				"id":                      tpl.ID,
				"title":                   tpl.Title,
				"category":                tpl.Category,
				"description":             tpl.Description,
				"version":                 tpl.Version,
				"renderer":                tpl.Renderer,
				"layout":                  tpl.Layout,
				"visual_design":           tpl.VisualDesign,
				"input_schema_kind":       tpl.InputSchemaKind,
				"tags":                    tpl.Tags,
				"interactions":            tpl.Interactions,
				"limits":                  tpl.Limits,
				"schema_file":             schemaFile,
				"example_file":            templateExampleRel(entry),
				"aliases":                 entry.Aliases,
				"agent_guide_available":   authoring.GuideAvailable(templateDir, entry),
				"agent_guide_path":        authoring.GuideRelPath(entry),
				"panel_grammar_available": authoring.PanelGrammarAvailable(templateDir, entry),
				"panel_grammar_path":      authoring.PanelGrammarRelPath(entry),
				"quality_rules_available": authoring.QualityRulesAvailable(templateDir, entry),
				"quality_rules_path":      authoring.QualityRulesRelPath(entry),
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
			entry, requestedID, ok := registry.Resolve(args[0])
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
			guide, guideErr := authoring.LoadGuide(templateDir, entry, false)
			if guideErr != nil {
				return print(cmd, o, failureFromError(guideErr, "template_manifest_invalid"))
			}
			panelGrammar, panelErr := authoring.LoadPanelGrammar(templateDir, entry, false)
			if panelErr != nil {
				return print(cmd, o, failureFromError(panelErr, "template_manifest_invalid"))
			}
			return print(cmd, o, output.Success("", map[string]any{
				"template": map[string]any{
					"requested_id":      requestedID,
					"canonical_id":      tpl.ID,
					"id":                tpl.ID,
					"version":           tpl.Version,
					"category":          tpl.Category,
					"title":             tpl.Title,
					"description":       tpl.Description,
					"renderer":          tpl.Renderer.Contract,
					"layout":            tpl.Layout,
					"visual_design":     tpl.VisualDesign,
					"input_schema_kind": tpl.InputSchemaKind,
					"tags":              tpl.Tags,
					"aliases":           entry.Aliases,
				},
				"schema_file":             schemaFile,
				"json_schema":             doc.JSONSchema,
				"example_file":            templateExampleRel(entry),
				"example":                 doc.Example,
				"agent_guide_available":   authoring.GuideAvailable(templateDir, entry),
				"agent_guide_path":        authoring.GuideRelPath(entry),
				"agent_guide_summary":     guide.Summary,
				"panel_grammar_available": panelGrammar.Available,
				"panel_grammar_path":      authoring.PanelGrammarRelPath(entry),
				"panel_grammar_summary":   panelGrammar.Summary,
			}))
		},
	}
}

func templateGuideCmd(o *Opts) *cobra.Command {
	return &cobra.Command{
		Use:   "guide <template-id>",
		Short: "Show one visual template agent authoring guide",
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
			entry, requestedID, ok := registry.Resolve(args[0])
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
			guide, err := authoring.LoadGuide(templateDir, entry, true)
			if err != nil {
				return print(cmd, o, failureFromError(err, "template_manifest_invalid"))
			}
			data := map[string]any{
				"template_id":            tpl.ID,
				"requested_id":           requestedID,
				"canonical_id":           tpl.ID,
				"guide_path":             authoring.GuideRelPath(entry),
				"agent_guide_available":  guide.Available,
				"raw_markdown":           guide.Raw,
				"guide":                  guide.Sections,
				"guide_summary":          guide.Summary,
				"missing_guide_sections": authoring.MissingGuideSections(guide.Sections),
			}
			if !guide.Available {
				data["warning"] = "Template agent guide is missing; fall back to visual template schema and common visual quality guidance."
			}
			return print(cmd, o, output.Success("", data))
		},
	}
}

func templatePanelGrammarCmd(o *Opts) *cobra.Command {
	return &cobra.Command{
		Use:   "panel-grammar <template-id>",
		Short: "Show one Studio visual template panel grammar",
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
			entry, requestedID, ok := registry.Resolve(args[0])
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
			panelGrammar, err := authoring.LoadPanelGrammar(templateDir, entry, true)
			if err != nil {
				return print(cmd, o, failureFromError(err, "template_manifest_invalid"))
			}
			data := map[string]any{
				"template_id":             tpl.ID,
				"requested_id":            requestedID,
				"canonical_id":            tpl.ID,
				"panel_grammar_path":      authoring.PanelGrammarRelPath(entry),
				"panel_grammar_available": panelGrammar.Available,
				"raw_markdown":            panelGrammar.Raw,
				"panel_grammar":           panelGrammar.Sections,
				"panel_grammar_summary":   panelGrammar.Summary,
			}
			if !panelGrammar.Available {
				data["warning"] = "Template panel grammar is missing; fall back to visual template schema and agent guide."
			}
			return print(cmd, o, output.Success("", data))
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
			if err := render.ScanOffline(templateDir); err != nil {
				return print(cmd, o, doctorFailure(err, "", templateDir))
			}
			expected, warnings := registry.EffectiveExpected()
			counts := registry.CategoryCounts()
			if registry.CanonicalCount() != expected.CanonicalCount {
				return print(cmd, o, doctorFailure(visualCommandError{
					code:    "template_doctor_failed",
					message: "visual template registry expected canonical count mismatch.",
					hint:    "Expected canonical_count=" + strconv.Itoa(expected.CanonicalCount) + ", got " + strconv.Itoa(registry.CanonicalCount()) + ". Update registry.expected or the template catalog together.",
					status:  400,
					file:    filepath.Join(templateDir, "registry.json"),
				}, "", filepath.Join(templateDir, "registry.json")))
			}
			if err := manifest.ValidateExpectedCategoryCounts(counts, expected.Categories); err != nil {
				return print(cmd, o, doctorFailure(err, "", filepath.Join(templateDir, "registry.json")))
			}
			canonicalTemplateDirs := registry.CanonicalTemplateDirs()
			orphanTemplateDirs, err := registry.OrphanTemplateDirs(templateDir)
			if err != nil {
				return print(cmd, o, doctorFailure(err, "", templateDir))
			}
			if len(orphanTemplateDirs) > 0 {
				return print(cmd, o, doctorFailure(visualCommandError{
					code:               "template_doctor_failed",
					message:            "Found template directories that are not registered in templates/visual/registry.json.",
					hint:               "Remove legacy directories or add an explicit allowed_legacy_alias_dirs entry.",
					status:             400,
					file:               filepath.ToSlash(filepath.Join(templateDir, "registry.json")),
					orphanTemplateDirs: orphanTemplateDirs,
				}, "", filepath.Join(templateDir, "registry.json")))
			}
			var checked []doctorTemplateResult
			checkedExamples := 0
			renderedExamples := 0
			exampleHashes := map[string]string{}
			for _, entry := range registry.Templates {
				if err := checkTemplateRequiredFiles(templateDir, entry); err != nil {
					return print(cmd, o, doctorFailure(err, entry.ID, filepath.ToSlash(entry.Path)))
				}
				tpl, err := manifest.LoadTemplateManifest(templateDir, entry)
				if err != nil {
					return print(cmd, o, doctorFailure(err, entry.ID, filepath.ToSlash(entry.Path)))
				}
				if err := manifest.ValidateTemplateManifest(templateDir, entry, &tpl); err != nil {
					return print(cmd, o, doctorFailure(err, entry.ID, filepath.ToSlash(entry.Path)))
				}
				_, schemaFile, err := manifest.LoadTemplateInputSchema(templateDir, entry, tpl)
				if err != nil {
					return print(cmd, o, doctorFailure(err, entry.ID, schemaFile))
				}
				examplePath := templateExamplePath(templateDir, entry)
				exampleRel := templateExampleRel(entry)
				raw, err := os.ReadFile(examplePath)
				if err != nil {
					return print(cmd, o, doctorFailure(visualCommandError{
						code:       "template_doctor_failed",
						message:    "visual template example was not found: " + exampleRel,
						hint:       "Add examples/basic.input.json for " + entry.ID + ".",
						status:     400,
						templateID: entry.ID,
						file:       exampleRel,
					}, entry.ID, exampleRel))
				}
				if _, err := visualschema.ValidateInput(tpl.InputSchemaKind, raw, tpl.Limits); err != nil {
					return print(cmd, o, doctorFailure(err, entry.ID, exampleRel))
				}
				checkedExamples++
				exampleHashes[hashBytes(raw)] = entry.ID
				if err := renderDoctorExample(templateDir, entry, examplePath); err != nil {
					return print(cmd, o, doctorFailure(err, entry.ID, exampleRel))
				}
				renderedExamples++
				checked = append(checked, doctorTemplateResult{
					ID:              tpl.ID,
					Version:         tpl.Version,
					Category:        tpl.Category,
					InputSchemaKind: tpl.InputSchemaKind,
					Example:         exampleRel,
					Rendered:        true,
				})
			}
			if len(exampleHashes) < minInt(190, len(registry.Templates)) {
				return print(cmd, o, doctorFailure(visualCommandError{
					code:    "template_doctor_failed",
					message: "visual template examples are not sufficiently unique.",
					hint:    "Provide semantic examples; at least 190 examples/basic.input.json files must have unique content hashes.",
					status:  400,
				}, "", templateDir))
			}
			return print(cmd, o, output.Success("", map[string]any{
				"template_dir":                 templateDir,
				"registry_version":             registry.Version,
				"expected_canonical_templates": expected.CanonicalCount,
				"canonical_templates":          registry.CanonicalCount(),
				"total_templates":              registry.TotalCount(),
				"alias_count":                  registry.AliasCount(),
				"expected_categories":          expected.Categories,
				"categories":                   counts,
				"category_list":                manifest.SortedCategoryCounts(counts),
				"checked_templates":            len(checked),
				"checked_examples":             checkedExamples,
				"rendered_examples":            renderedExamples,
				"unique_example_hashes":        len(exampleHashes),
				"canonical_template_dirs":      len(canonicalTemplateDirs),
				"orphan_template_dirs":         orphanTemplateDirs,
				"offline":                      true,
				"offline_strict":               o.OfflineStrict,
				"templates":                    checked,
				"warnings":                     warnings,
			}))
		},
	}
}

type doctorTemplateResult struct {
	ID              string `json:"id"`
	Version         string `json:"version"`
	Category        string `json:"category"`
	InputSchemaKind string `json:"input_schema_kind"`
	Example         string `json:"example"`
	Rendered        bool   `json:"rendered"`
}

type templateListFilter struct {
	Category   string
	Query      string
	Renderer   string
	SchemaKind string
}

func filterRegistryTemplates(entries []manifest.RegistryEntry, filter templateListFilter) []manifest.RegistryEntry {
	filter.Category = strings.ToLower(strings.TrimSpace(filter.Category))
	filter.Query = strings.ToLower(strings.TrimSpace(filter.Query))
	filter.Renderer = strings.TrimSpace(filter.Renderer)
	filter.SchemaKind = strings.ToLower(strings.TrimSpace(filter.SchemaKind))
	var out []manifest.RegistryEntry
	for _, entry := range entries {
		if filter.Category != "" && strings.ToLower(entry.Category) != filter.Category {
			continue
		}
		if filter.Renderer != "" && entry.Renderer != filter.Renderer {
			continue
		}
		if filter.SchemaKind != "" && strings.ToLower(entry.InputSchemaKind) != filter.SchemaKind {
			continue
		}
		if filter.Query != "" && !entryMatchesQuery(entry, filter.Query) {
			continue
		}
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func entryMatchesQuery(entry manifest.RegistryEntry, query string) bool {
	fields := []string{entry.ID, entry.Title, entry.Description, entry.Category, entry.Renderer, entry.InputSchemaKind, entry.LayoutPreset}
	fields = append(fields, entry.Tags...)
	fields = append(fields, entry.Aliases...)
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), query) {
			return true
		}
	}
	return false
}

func normalizedTemplateListFilter(category, query, renderer, schemaKind string) map[string]string {
	return map[string]string{
		"category":    strings.ToLower(strings.TrimSpace(category)),
		"query":       strings.TrimSpace(query),
		"renderer":    strings.TrimSpace(renderer),
		"schema_kind": strings.ToLower(strings.TrimSpace(schemaKind)),
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

func doctorFailure(err error, templateID, file string) output.Envelope {
	message := "visual template doctor failed."
	hint := "Fix this template and rerun visual template doctor --template-dir ./templates/visual --json."
	status := 400
	var missing []string
	var orphan []string
	var ce codedError
	if errors.As(err, &ce) {
		message = ce.Message()
		if strings.TrimSpace(ce.Hint()) != "" {
			hint = ce.Hint()
		}
		status = ce.Status()
		missing = missingFilesFromError(err)
		orphan = orphanTemplateDirsFromError(err)
	} else if err != nil {
		message = err.Error()
	}
	if templateID == "" {
		var te templateIDError
		if errors.As(err, &te) {
			templateID = te.TemplateID()
		}
	}
	if file == "" {
		var fe fileError
		if errors.As(err, &fe) {
			file = fe.File()
		}
	}
	return failureFromError(visualCommandError{
		code:               "template_doctor_failed",
		message:            message,
		hint:               hint,
		status:             status,
		templateID:         templateID,
		file:               file,
		missingFiles:       missing,
		orphanTemplateDirs: orphan,
	}, "template_doctor_failed")
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

func hashBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
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

func orphanTemplateDirsFromError(err error) []string {
	var oe orphanTemplateDirsError
	if errors.As(err, &oe) {
		return oe.OrphanTemplateDirs()
	}
	return nil
}

type visualCommandError struct {
	code               string
	message            string
	hint               string
	status             int
	templateID         string
	file               string
	missingFiles       []string
	orphanTemplateDirs []string
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
func (e visualCommandError) OrphanTemplateDirs() []string {
	return append([]string{}, e.orphanTemplateDirs...)
}
