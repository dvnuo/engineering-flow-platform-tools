package render

import (
	"io"
	"os"
	"path/filepath"
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
	OutDir     string   `json:"out"`
	Entrypoint string   `json:"entrypoint"`
	Files      []string `json:"files"`
	Offline    bool     `json:"offline"`
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
	artifact := Artifact{OutDir: opts.OutDir, Entrypoint: ToArtifactPath(filepath.Join(opts.OutDir, "index.html")), Files: files, Offline: true}
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
	return result, nil
}

func InspectOutput(outDir string, offlineStrict bool) (Artifact, error) {
	for _, rel := range []string{"index.html", "manifest.json", "manifest.js", "data.js"} {
		path := filepath.Join(outDir, rel)
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return Artifact{}, metadata.NewError("output_path_invalid", "visual output is missing "+rel+".", "Pass a directory created by visual render.", 404)
		}
	}
	if offlineStrict {
		if err := ScanOffline(outDir); err != nil {
			return Artifact{}, err
		}
	}
	return Artifact{
		OutDir:     outDir,
		Entrypoint: ToArtifactPath(filepath.Join(outDir, "index.html")),
		Files:      []string{"index.html", "manifest.json", "manifest.js", "data.js"},
		Offline:    true,
	}, nil
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
