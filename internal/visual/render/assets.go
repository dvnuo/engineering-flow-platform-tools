package render

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"

	"engineering-flow-platform-tools/internal/visual/manifest"
	"engineering-flow-platform-tools/internal/visual/metadata"
)

const threeModuleSource = "_shared/vendor/three/efp-three.module.min.js"
const threeModuleOutput = "assets/vendor/three/efp-three.module.min.js"

var sharedAssetFiles = map[string]string{
	"_shared/agent-guidance/mark-grammar.md": "assets/agent-guidance/mark-grammar.md",
	"_shared/asset-registry.json":            "assets/asset-registry.json",
	"_shared/mark-registry.json":             "assets/mark-registry.json",
	"_shared/assets/ATTRIBUTIONS.md":         "assets/ATTRIBUTIONS.md",
}

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
	sharedFiles, err := copySharedVisualAssets(templateDir, outDir)
	if err != nil {
		return nil, err
	}
	files = append(files, sharedFiles...)
	sort.Strings(files)
	return files, nil
}

func copySharedVisualAssets(templateDir, outDir string) ([]string, error) {
	var files []string
	for srcRel, dstRel := range sharedAssetFiles {
		src := filepath.Join(templateDir, filepath.FromSlash(srcRel))
		info, err := os.Stat(src)
		if err != nil || info.IsDir() {
			return nil, metadata.NewError("template_asset_missing", "visual shared asset was not found: "+srcRel, "Ensure templates/visual/_shared visual assets are present.", 404)
		}
		dst, err := safeOutputPath(outDir, dstRel)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return nil, metadata.NewError("output_write_failed", "failed to create shared asset directory: "+err.Error(), "Check --out permissions.", 500)
		}
		if err := copyFile(dst, src); err != nil {
			return nil, err
		}
		files = append(files, ToArtifactPath(dstRel))
	}
	for _, dir := range []struct {
		src string
		dst string
	}{
		{src: "_shared/assets/icons", dst: "assets/icons"},
		{src: "_shared/assets/models", dst: "assets/models"},
	} {
		copied, err := copySharedDirectory(templateDir, outDir, dir.src, dir.dst)
		if err != nil {
			return nil, err
		}
		files = append(files, copied...)
	}
	sort.Strings(files)
	return files, nil
}

func copySharedDirectory(templateDir, outDir, srcRel, dstRel string) ([]string, error) {
	srcRoot := filepath.Join(templateDir, filepath.FromSlash(srcRel))
	info, err := os.Stat(srcRoot)
	if err != nil || !info.IsDir() {
		return nil, metadata.NewError("template_asset_missing", "visual shared asset directory was not found: "+srcRel, "Ensure templates/visual/_shared visual assets are present.", 404)
	}
	var files []string
	walkErr := filepath.WalkDir(srcRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		outRel := filepath.ToSlash(filepath.Join(dstRel, rel))
		dst, err := safeOutputPath(outDir, outRel)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return metadata.NewError("output_write_failed", "failed to create shared asset directory: "+err.Error(), "Check --out permissions.", 500)
		}
		if err := copyFile(dst, path); err != nil {
			return err
		}
		files = append(files, ToArtifactPath(outRel))
		return nil
	})
	if walkErr != nil {
		if meta, ok := walkErr.(*metadata.Error); ok {
			return nil, meta
		}
		return nil, metadata.NewError("output_write_failed", "failed to copy shared visual assets: "+walkErr.Error(), "Inspect shared visual asset files.", 500)
	}
	sort.Strings(files)
	return files, nil
}

type assetRegistryDoc struct {
	Icons        map[string]assetRegistryEntry `json:"icons"`
	Models       map[string]assetRegistryEntry `json:"models"`
	Attributions []manifest.AssetAttribution   `json:"attributions"`
}

type assetRegistryEntry struct {
	Path string `json:"path"`
}

func BuildOutputAssets(templateDir string) (manifest.OutputAssets, error) {
	var out manifest.OutputAssets
	assetRegistryPath := filepath.Join(templateDir, "_shared", "asset-registry.json")
	assetRegistryRaw, err := readJSONMap(assetRegistryPath)
	if err != nil {
		return out, err
	}
	markRegistryPath := filepath.Join(templateDir, "_shared", "mark-registry.json")
	markRegistryRaw, err := readJSONMap(markRegistryPath)
	if err != nil {
		return out, err
	}
	var registry assetRegistryDoc
	b, err := os.ReadFile(assetRegistryPath)
	if err != nil {
		return out, metadata.NewError("template_asset_missing", "visual asset registry was not found: "+assetRegistryPath, "Ensure templates/visual/_shared/asset-registry.json is present.", 404)
	}
	if err := json.Unmarshal(b, &registry); err != nil {
		return out, metadata.NewError("template_manifest_invalid", "visual asset registry is invalid JSON: "+err.Error(), "Fix templates/visual/_shared/asset-registry.json.", 400)
	}
	for _, entry := range registry.Icons {
		if entry.Path != "" {
			out.Icons = append(out.Icons, ToArtifactPath(entry.Path))
		}
	}
	for _, entry := range registry.Models {
		if entry.Path != "" {
			out.Models = append(out.Models, ToArtifactPath(entry.Path))
		}
	}
	sort.Strings(out.Icons)
	sort.Strings(out.Models)
	out.Attributions = registry.Attributions
	out.AssetRegistry = assetRegistryRaw
	out.MarkRegistry = markRegistryRaw
	return out, nil
}

func readJSONMap(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, metadata.NewError("template_asset_missing", "visual shared registry was not found: "+path, "Ensure templates/visual/_shared registries are present.", 404)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, metadata.NewError("template_manifest_invalid", "visual shared registry is invalid JSON: "+err.Error(), "Fix templates/visual/_shared registry JSON.", 400)
	}
	return out, nil
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
	for _, rel := range sharedAssetFiles {
		add(rel)
	}
	add("assets/icons/generic/service.svg")
	add("assets/icons/generic/api.svg")
	add("assets/icons/generic/database.svg")
	add("assets/icons/generic/queue.svg")
	add("assets/icons/aws/lambda.svg")
	add("assets/icons/jenkins/jenkins.svg")
	add("assets/models/generic/placeholder.json")
	return files
}

func usesThreeEffects(tpl manifest.TemplateManifest) bool {
	return tpl.Effects.Engine == "three.v1"
}
