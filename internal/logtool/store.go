package logtool

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

const (
	manifestFile  = "manifest.json"
	entriesFile   = "entries.jsonl"
	templatesFile = "templates.json"
)

func EnsureRunDir(runDir string) error {
	if runDir == "" {
		return NewError("invalid_args", "--run is required.", "Pass --run <run-dir>.", 400)
	}
	return os.MkdirAll(runDir, 0o755)
}

func WriteManifest(runDir string, manifest Manifest) error {
	if err := EnsureRunDir(runDir); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(runDir, manifestFile), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(manifest)
}

func ReadManifest(runDir string) (Manifest, error) {
	f, err := os.Open(filepath.Join(runDir, manifestFile))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Manifest{}, NewError("run_not_found", "Analysis run directory was not found.", "Run log analyze --source <path> --run <run-dir> --json first.", 404)
		}
		return Manifest{}, err
	}
	defer f.Close()
	var manifest Manifest
	if err := json.NewDecoder(f).Decode(&manifest); err != nil {
		return Manifest{}, NewError("invalid_run", "manifest.json could not be decoded.", "Re-run log analyze for this source.", 400)
	}
	return manifest, nil
}

func AppendEntry(w io.Writer, entry Entry) error {
	b, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	_, err = w.Write([]byte("\n"))
	return err
}

func ReadEntries(runDir string, fn func(Entry) error) error {
	f, err := os.Open(filepath.Join(runDir, entriesFile))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NewError("run_not_found", "entries.jsonl was not found.", "Run log analyze --source <path> --run <run-dir> --json first.", 404)
		}
		return err
	}
	defer f.Close()
	reader := bufio.NewReader(f)
	lineNo := 0
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			lineNo++
			var entry Entry
			if unmarshalErr := json.Unmarshal(line, &entry); unmarshalErr != nil {
				return NewError("invalid_run", "entries.jsonl contains invalid JSON.", "Re-run log analyze for this source.", 400)
			}
			if fnErr := fn(entry); fnErr != nil {
				return fnErr
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
	}
	_ = lineNo
	return nil
}

func WriteTemplates(runDir string, templates []Template) error {
	if err := EnsureRunDir(runDir); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(runDir, templatesFile), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]any{"templates": templates})
}

func ReadTemplates(runDir string) ([]Template, error) {
	f, err := os.Open(filepath.Join(runDir, templatesFile))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, NewError("run_not_found", "templates.json was not found.", "Run log analyze --source <path> --run <run-dir> --json first.", 404)
		}
		return nil, err
	}
	defer f.Close()
	var wrapper struct {
		Templates []Template `json:"templates"`
	}
	if err := json.NewDecoder(f).Decode(&wrapper); err != nil {
		return nil, NewError("invalid_run", "templates.json could not be decoded.", "Re-run log analyze for this source.", 400)
	}
	return wrapper.Templates, nil
}
