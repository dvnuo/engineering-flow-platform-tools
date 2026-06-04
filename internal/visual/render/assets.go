package render

import (
	"io"
	"os"
	"path/filepath"

	"engineering-flow-platform-tools/internal/visual/manifest"
	"engineering-flow-platform-tools/internal/visual/metadata"
)

const threeModuleSource = "_shared/vendor/three/efp-three.module.min.js"
const threeModuleOutput = "assets/vendor/three/efp-three.module.min.js"

func copyAssets(templateDir string, entry manifest.RegistryEntry, tpl manifest.TemplateManifest, outDir string) ([]string, error) {
	var files []string
	for _, asset := range tpl.Assets {
		src, err := safeResolveTemplatePath(templateDir, entry.Path, asset.From)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(src)
		if err != nil || info.IsDir() {
			return nil, metadata.NewError("template_asset_missing", "visual template asset was not found: "+asset.From, "Ensure every asset.from exists and is a file.", 404)
		}
		dst, err := safeOutputPath(outDir, asset.To)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return nil, metadata.NewError("output_write_failed", "failed to create asset directory: "+err.Error(), "Check --out permissions.", 500)
		}
		if err := copyFile(dst, src); err != nil {
			return nil, err
		}
		files = append(files, ToArtifactPath(asset.To))
	}
	if usesThreeEffects(tpl) {
		src := filepath.Join(templateDir, filepath.FromSlash(threeModuleSource))
		info, err := os.Stat(src)
		if err != nil || info.IsDir() {
			return nil, metadata.NewError("template_asset_missing", "visual Three.js vendor module was not found: "+threeModuleSource, "Ensure templates/visual/_shared/vendor/three is present.", 404)
		}
		dst, err := safeOutputPath(outDir, threeModuleOutput)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return nil, metadata.NewError("output_write_failed", "failed to create Three.js vendor directory: "+err.Error(), "Check --out permissions.", 500)
		}
		if err := copyFile(dst, src); err != nil {
			return nil, err
		}
		files = append(files, ToArtifactPath(threeModuleOutput))
	}
	return files, nil
}

func copyFile(dst, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return metadata.NewError("template_asset_missing", "failed to read template asset: "+err.Error(), "Ensure asset.from exists and is readable.", 404)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return metadata.NewError("output_write_failed", "failed to write asset: "+err.Error(), "Check --out permissions.", 500)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return metadata.NewError("output_write_failed", "failed to copy asset: "+err.Error(), "Check --out permissions and disk space.", 500)
	}
	return nil
}

func plannedFiles(tpl manifest.TemplateManifest) []string {
	seen := map[string]bool{}
	var files []string
	add := func(path string) {
		path = ToArtifactPath(path)
		if path != "" && !seen[path] {
			seen[path] = true
			files = append(files, path)
		}
	}
	for _, path := range []string{"index.html", "manifest.json", "manifest.js", "data.js"} {
		add(path)
	}
	for _, asset := range tpl.Assets {
		add(asset.To)
	}
	if usesThreeEffects(tpl) {
		add(threeModuleOutput)
	}
	return files
}

func usesThreeEffects(tpl manifest.TemplateManifest) bool {
	return tpl.Effects.Engine == "three.v1"
}
