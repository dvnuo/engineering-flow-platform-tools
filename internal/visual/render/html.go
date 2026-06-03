package render

import (
	"html/template"
	"os"
	"path/filepath"

	"engineering-flow-platform-tools/internal/visual/manifest"
	"engineering-flow-platform-tools/internal/visual/metadata"
)

func writeIndex(templateDir, outDir string, outputManifest manifest.OutputManifest, tpl manifest.TemplateManifest) error {
	path := filepath.Join(templateDir, "_shared", "shell", "index.tmpl.html")
	t, err := template.ParseFiles(path)
	if err != nil {
		return metadata.NewError("output_write_failed", "failed to parse visual shell template: "+err.Error(), "Ensure templates/visual/_shared/shell/index.tmpl.html exists.", 500)
	}
	target := filepath.Join(outDir, "index.html")
	f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return metadata.NewError("output_write_failed", "failed to write index.html: "+err.Error(), "Check --out permissions.", 500)
	}
	defer f.Close()
	data := map[string]any{
		"Title":   outputManifest.Title,
		"Styles":  tpl.Styles,
		"Scripts": tpl.Scripts,
	}
	if err := t.Execute(f, data); err != nil {
		return metadata.NewError("output_write_failed", "failed to render index.html: "+err.Error(), "Inspect visual shell template data.", 500)
	}
	return nil
}
