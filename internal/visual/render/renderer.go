package render

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/visual/manifest"
	"engineering-flow-platform-tools/internal/visual/metadata"
	visualschema "engineering-flow-platform-tools/internal/visual/schema"
)

type Options struct {
	TemplateDir   string
	TemplateID    string
	InputPath     string
	OutDir        string
	Title         string
	Overwrite     bool
	DryRun        bool
	DataMode      string
	OfflineStrict bool
	Stdin         io.Reader
}

type Result struct {
	TemplateID   string                    `json:"template_id"`
	TemplateDir  string                    `json:"template_dir"`
	InputSummary visualschema.InputSummary `json:"input_summary"`
	Artifact     Artifact                  `json:"artifact"`
	PlannedFiles []string                  `json:"planned_files,omitempty"`
	DryRun       bool                      `json:"dry_run,omitempty"`
}

type Artifact struct {
	TemplateID         string   `json:"template_id,omitempty"`
	TemplateVersion    string   `json:"template_version,omitempty"`
	Title              string   `json:"title,omitempty"`
	OutDir             string   `json:"out_dir"`
	Out                string   `json:"out"`
	Entrypoint         string   `json:"entrypoint"`
	RelativeEntrypoint string   `json:"relative_entrypoint"`
	Offline            bool     `json:"offline"`
	FileURLSafe        bool     `json:"file_url_safe"`
	HTTPSubpathSafe    bool     `json:"http_subpath_safe"`
	Files              []string `json:"files"`
}

type Inspection struct {
	Artifact Artifact     `json:"artifact"`
	Checks   OutputChecks `json:"checks"`
}

type OutputChecks struct {
	IndexHTML          bool `json:"index_html"`
	ManifestJSON       bool `json:"manifest_json"`
	ManifestJS         bool `json:"manifest_js"`
	DataJS             bool `json:"data_js"`
	RuntimeJS          bool `json:"runtime_js"`
	RuntimeRenderersJS bool `json:"runtime_renderers_js"`
	RuntimeCSS         bool `json:"runtime_css"`
	OfflineScan        bool `json:"offline_scan"`
}

type OutputInvalidError struct {
	Missing []string
}

func (e OutputInvalidError) Error() string {
	return e.Message()
}

func (e OutputInvalidError) Code() string {
	return "visual_output_invalid"
}

func (e OutputInvalidError) Message() string {
	return "Visual output directory is missing required files."
}

func (e OutputInvalidError) Hint() string {
	return "Run visual render again or inspect the template assets."
}

func (e OutputInvalidError) Status() int {
	return 400
}

func (e OutputInvalidError) MissingFiles() []string {
	return append([]string{}, e.Missing...)
}

func Render(opts Options) (Result, error) {
	if normalizeDataMode(opts.DataMode) != "js-file" {
		return Result{}, metadata.NewError("unsupported_data_mode", "visual render only supports --data-mode js-file.", "Use --data-mode js-file or omit the flag.", 400)
	}
	registry, entry, tpl, err := loadTemplate(opts.TemplateDir, opts.TemplateID)
	if err != nil {
		_ = registry
		return Result{}, err
	}
	raw, err := readInput(opts.InputPath, opts.Stdin)
	if err != nil {
		return Result{}, err
	}
	parsed, err := visualschema.ValidateInput(tpl.InputSchemaKind, raw, tpl.Limits)
	if err != nil {
		return Result{}, err
	}
	title := strings.TrimSpace(opts.Title)
	if title == "" {
		title = parsed.Title
	}
	if title == "" {
		title = tpl.Title
	}
	files := plannedFiles(tpl)
	artifact := NewArtifact(tpl.ID, tpl.Version, title, opts.OutDir, files)
	result := Result{TemplateID: tpl.ID, TemplateDir: opts.TemplateDir, InputSummary: parsed.Summary, Artifact: artifact}
	if opts.DryRun {
		result.PlannedFiles = files
		result.DryRun = true
		return result, nil
	}
	if err := prepareOutputDir(opts.OutDir, opts.Overwrite); err != nil {
		return Result{}, err
	}
	if _, err := copyAssets(opts.TemplateDir, entry, tpl, opts.OutDir); err != nil {
		return Result{}, err
	}
	outputManifest := manifest.OutputManifest{
		Schema: "efp.visual.output.manifest.v1",
		Template: manifest.OutputTemplate{
			ID:      tpl.ID,
			Version: tpl.Version,
		},
		Renderer:     tpl.Renderer,
		Title:        title,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		Offline:      true,
		Entrypoint:   "index.html",
		Layout:       tpl.Layout,
		Effects:      tpl.Effects,
		VisualDesign: tpl.VisualDesign,
		Interactions: tpl.Interactions,
	}
	if err := writeJSONFile(filepath.Join(opts.OutDir, "manifest.json"), outputManifest); err != nil {
		return Result{}, err
	}
	if err := writeJSAssignment(filepath.Join(opts.OutDir, "manifest.js"), "__EFP_VISUAL_MANIFEST__", outputManifest); err != nil {
		return Result{}, err
	}
	if err := writeJSAssignment(filepath.Join(opts.OutDir, "data.js"), "__EFP_VISUAL_DATA__", parsed.Data); err != nil {
		return Result{}, err
	}
	if err := writeIndex(opts.TemplateDir, opts.OutDir, outputManifest, tpl); err != nil {
		return Result{}, err
	}
	if opts.OfflineStrict {
		if err := ScanOffline(opts.OutDir); err != nil {
			return Result{}, err
		}
	}
	actualFiles, err := ListOutputFiles(opts.OutDir)
	if err != nil {
		return Result{}, err
	}
	result.Artifact = NewArtifact(tpl.ID, tpl.Version, title, opts.OutDir, actualFiles)
	return result, nil
}

func NewArtifact(templateID, templateVersion, title, outDir string, files []string) Artifact {
	out := ToArtifactPath(outDir)
	entrypoint := ToArtifactPath(filepath.Join(outDir, "index.html"))
	return Artifact{
		TemplateID:         templateID,
		TemplateVersion:    templateVersion,
		Title:              title,
		OutDir:             out,
		Out:                out,
		Entrypoint:         entrypoint,
		RelativeEntrypoint: "index.html",
		Offline:            true,
		FileURLSafe:        true,
		HTTPSubpathSafe:    true,
		Files:              files,
	}
}

func InspectOutput(outDir string, offlineStrict bool) (Inspection, error) {
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
		path := filepath.Join(outDir, rel)
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			missing = append(missing, rel)
		}
	}
	if len(missing) > 0 {
		return Inspection{}, OutputInvalidError{Missing: missing}
	}
	if offlineStrict {
		if err := ScanOffline(outDir); err != nil {
			return Inspection{}, err
		}
	}
	files, err := ListOutputFiles(outDir)
	if err != nil {
		return Inspection{}, err
	}
	return Inspection{
		Artifact: NewArtifact("", "", "", outDir, files),
		Checks: OutputChecks{
			IndexHTML:          true,
			ManifestJSON:       true,
			ManifestJS:         true,
			DataJS:             true,
			RuntimeJS:          true,
			RuntimeRenderersJS: true,
			RuntimeCSS:         true,
			OfflineScan:        offlineStrict,
		},
	}, nil
}

func ListOutputFiles(outDir string) ([]string, error) {
	var files []string
	rootAbs, err := filepath.Abs(outDir)
	if err != nil {
		return nil, metadata.NewError("output_path_invalid", "failed to resolve output directory: "+err.Error(), "Pass a valid --out directory.", 400)
	}
	err = filepath.WalkDir(rootAbs, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(rootAbs, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, metadata.NewError("output_path_invalid", "failed to list visual output files: "+err.Error(), "Inspect the output directory permissions.", 400)
	}
	sort.Strings(files)
	return files, nil
}

func loadTemplate(templateDir, templateID string) (manifest.Registry, manifest.RegistryEntry, manifest.TemplateManifest, error) {
	registry, err := manifest.LoadRegistry(templateDir)
	if err != nil {
		return registry, manifest.RegistryEntry{}, manifest.TemplateManifest{}, err
	}
	entry, ok := registry.Find(templateID)
	if !ok {
		return registry, manifest.RegistryEntry{}, manifest.TemplateManifest{}, metadata.NewError("template_not_found", "visual template was not found: "+templateID, "Run visual template list --json and choose one of the returned ids.", 404)
	}
	tpl, err := manifest.LoadTemplateManifest(templateDir, entry)
	if err != nil {
		return registry, entry, manifest.TemplateManifest{}, err
	}
	if err := manifest.ValidateTemplateManifest(templateDir, entry, &tpl); err != nil {
		return registry, entry, manifest.TemplateManifest{}, err
	}
	return registry, entry, tpl, nil
}

func readInput(path string, stdin io.Reader) ([]byte, error) {
	if strings.TrimSpace(path) == "-" {
		if stdin == nil {
			stdin = os.Stdin
		}
		b, err := io.ReadAll(stdin)
		if err != nil {
			return nil, metadata.NewError("input_read_failed", "failed to read input JSON from stdin: "+err.Error(), "Pipe valid JSON to visual render --input -.", 400)
		}
		return b, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, metadata.NewError("input_read_failed", "failed to read input JSON: "+err.Error(), "Pass a readable JSON file path to --input.", 400)
	}
	return b, nil
}

func prepareOutputDir(outDir string, overwrite bool) error {
	if strings.TrimSpace(outDir) == "" {
		return metadata.NewError("output_path_invalid", "visual output directory is empty.", "Pass --out <directory>.", 400)
	}
	info, err := os.Stat(outDir)
	if err == nil {
		if !info.IsDir() {
			return metadata.NewError("output_path_invalid", "visual output path exists and is not a directory.", "Pass --out as a directory path.", 400)
		}
		empty, err := dirEmpty(outDir)
		if err != nil {
			return metadata.NewError("output_path_invalid", "failed to inspect output directory: "+err.Error(), "Check --out permissions.", 400)
		}
		if !empty && !overwrite {
			return metadata.NewError("output_exists", "visual output directory already exists and is not empty.", "Pass --overwrite or choose a new --out directory.", 409)
		}
		if overwrite {
			if err := os.RemoveAll(outDir); err != nil {
				return metadata.NewError("output_write_failed", "failed to remove existing output directory: "+err.Error(), "Check --out permissions or choose a new directory.", 500)
			}
		}
	} else if !os.IsNotExist(err) {
		return metadata.NewError("output_path_invalid", "failed to inspect output path: "+err.Error(), "Check --out permissions.", 400)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return metadata.NewError("output_write_failed", "failed to create output directory: "+err.Error(), "Check --out permissions.", 500)
	}
	return nil
}

func dirEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

func normalizeDataMode(mode string) string {
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode == "" || mode == "js_file" {
		return "js-file"
	}
	return mode
}
