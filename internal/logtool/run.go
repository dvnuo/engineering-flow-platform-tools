package logtool

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func DefaultWorkspace() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return filepath.Join(".", ".log-runs")
	}
	return filepath.Join(home, ".efp", "log-runs")
}

func NewAutoRunDir() string {
	base := filepath.Join(DefaultWorkspace(), "run_"+time.Now().UTC().Format("20060102T150405Z"))
	if _, err := os.Stat(base); errors.Is(err, os.ErrNotExist) {
		return base
	}
	for i := 1; i <= 999; i++ {
		candidate := fmt.Sprintf("%s_%03d", base, i)
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
	return base
}

func ResolveRunDir(run string) string {
	run = strings.TrimSpace(run)
	if run == "" {
		return run
	}
	cleaned := filepath.Clean(run)
	if filepath.IsAbs(cleaned) || strings.ContainsAny(cleaned, `\/`) || strings.HasPrefix(cleaned, ".") {
		return cleaned
	}
	if _, err := os.Stat(cleaned); err == nil {
		return cleaned
	}
	return filepath.Join(DefaultWorkspace(), cleaned)
}

func AnalyzeDryRun(opts AnalyzeOptions) (AnalyzeDryRunResult, error) {
	if strings.TrimSpace(opts.Source) == "" {
		return AnalyzeDryRunResult{}, NewError("invalid_args", "--source is required.", "Pass --source <file|dir|glob>.", 400)
	}
	if opts.FormatHint == "" {
		opts.FormatHint = "auto"
	}
	switch opts.FormatHint {
	case "auto", "json", "plain":
	default:
		return AnalyzeDryRunResult{}, NewError("invalid_args", "--format-hint must be auto, json, or plain.", "Run log schema analyze --json.", 400)
	}
	runDir := strings.TrimSpace(opts.RunDir)
	if runDir == "" {
		runDir = NewAutoRunDir()
	}
	sources, err := expandSources(opts.Source)
	if err != nil {
		return AnalyzeDryRunResult{}, err
	}
	var result AnalyzeDryRunResult
	result.DryRun = true
	result.RunDir = runDir
	for _, source := range sources {
		item := AnalyzeDryRunSource{Path: source, Exists: true}
		if info, statErr := os.Stat(source); statErr == nil {
			item.Bytes = info.Size()
			if f, openErr := os.Open(source); openErr == nil {
				item.Readable = true
				_ = f.Close()
			}
		}
		result.Sources = append(result.Sources, item)
		result.EstimatedWork.TotalBytes += item.Bytes
	}
	result.EstimatedWork.SourceCount = len(result.Sources)
	result.EstimatedWork.WillWriteWorkspace = false
	return result, nil
}

func RunList(workspace string) (RunListResult, error) {
	if strings.TrimSpace(workspace) == "" {
		workspace = DefaultWorkspace()
	}
	result := RunListResult{Workspace: workspace}
	entries, err := os.ReadDir(workspace)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return result, nil
		}
		return RunListResult{}, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		runDir := filepath.Join(workspace, entry.Name())
		manifest, err := ReadManifest(runDir)
		if err != nil {
			continue
		}
		result.Runs = append(result.Runs, runListItem(runDir, manifest))
	}
	sort.SliceStable(result.Runs, func(i, j int) bool {
		return result.Runs[i].CreatedAt > result.Runs[j].CreatedAt
	})
	return result, nil
}

func RunGet(run string) (RunGetResult, error) {
	runDir := ResolveRunDir(run)
	manifest, err := ReadManifest(runDir)
	if err != nil {
		return RunGetResult{}, err
	}
	return RunGetResult{
		RunID:    manifest.RunID,
		RunDir:   runDir,
		Status:   "ready",
		Manifest: manifest,
		Index: RunIndex{
			Entries:          manifest.EntriesCount,
			Templates:        manifest.TemplatesCount,
			RedactionEnabled: manifest.RedactionEnabled,
		},
		Workspace: DefaultWorkspace(),
	}, nil
}

func RunDelete(run string, yes, dryRun bool) (RunDeleteResult, error) {
	if strings.TrimSpace(run) == "" {
		return RunDeleteResult{}, NewError("invalid_args", "run is required.", "Pass a run id or run directory.", 400)
	}
	if !yes {
		return RunDeleteResult{}, NewError("invalid_args", "--yes is required for run delete.", "Add --yes only after confirming the run should be deleted.", 400)
	}
	runDir := ResolveRunDir(run)
	manifest, err := ReadManifest(runDir)
	if err != nil {
		return RunDeleteResult{}, err
	}
	result := RunDeleteResult{RunID: manifest.RunID, RunDir: runDir, DryRun: dryRun}
	if dryRun {
		return result, nil
	}
	if err := os.RemoveAll(runDir); err != nil {
		return RunDeleteResult{}, err
	}
	result.Deleted = true
	return result, nil
}

func RunVerify(run string) (RunVerifyResult, error) {
	runDir := ResolveRunDir(run)
	manifest, err := ReadManifest(runDir)
	if err != nil {
		return RunVerifyResult{}, err
	}
	files := map[string]bool{}
	for _, name := range []string{manifestFile, entriesFile, templatesFile} {
		_, err := os.Stat(filepath.Join(runDir, name))
		files[name] = err == nil
	}
	counts := map[string]int{"manifest_entries": manifest.EntriesCount, "manifest_templates": manifest.TemplatesCount}
	entryCount := 0
	if err := ReadEntries(runDir, func(Entry) error {
		entryCount++
		return nil
	}); err != nil {
		return RunVerifyResult{}, err
	}
	templates, err := ReadTemplates(runDir)
	if err != nil {
		return RunVerifyResult{}, err
	}
	counts["entries"] = entryCount
	counts["templates"] = len(templates)
	ok := files[manifestFile] && files[entriesFile] && files[templatesFile] && entryCount == manifest.EntriesCount && len(templates) == manifest.TemplatesCount
	var hints []string
	if !ok {
		hints = append(hints, "Run log analyze again if counts or files do not match.")
	}
	return RunVerifyResult{RunID: manifest.RunID, RunDir: runDir, OK: ok, Files: files, Counts: counts, Hints: hints}, nil
}

func Doctor() DoctorResult {
	workspace := DefaultWorkspace()
	checks := []DoctorCheck{
		{Name: "default_workspace", OK: true, Message: workspace},
		{Name: "local_only", OK: true, Message: "P0 analyzes local files, directories, and globs."},
		{Name: "redaction", OK: true, Message: "Redaction is enabled before run files and command output."},
	}
	if info, err := os.Stat(workspace); err == nil && !info.IsDir() {
		checks[0] = DoctorCheck{Name: "default_workspace", OK: false, Message: "Default workspace path exists but is not a directory."}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		checks[0] = DoctorCheck{Name: "default_workspace", OK: false, Message: RedactError(err.Error())}
	}
	return DoctorResult{Tool: "log", DefaultWorkspace: workspace, LocalOnly: true, Checks: checks}
}

func runListItem(runDir string, manifest Manifest) RunListItem {
	return RunListItem{
		RunID:          manifest.RunID,
		RunDir:         runDir,
		CreatedAt:      manifest.CreatedAt,
		Sources:        len(manifest.Sources),
		EntriesCount:   manifest.EntriesCount,
		TemplatesCount: manifest.TemplatesCount,
		TimeRange:      manifest.TimeRange,
		Truncated:      manifest.Truncated,
	}
}
