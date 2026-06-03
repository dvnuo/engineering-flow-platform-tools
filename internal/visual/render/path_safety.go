package render

import (
	"path/filepath"
	"strings"

	"engineering-flow-platform-tools/internal/visual/metadata"
)

func safeResolveTemplatePath(templateDir, templatePath, assetFrom string) (string, error) {
	rootAbs, err := filepath.Abs(templateDir)
	if err != nil {
		return "", metadata.NewError("template_asset_missing", "failed to resolve template directory: "+err.Error(), "Pass a valid --template-dir.", 400)
	}
	currentAbs, err := filepath.Abs(filepath.Join(templateDir, filepath.Dir(filepath.Clean(templatePath))))
	if err != nil {
		return "", metadata.NewError("template_asset_missing", "failed to resolve template path: "+err.Error(), "Check registry.json template paths.", 400)
	}
	candidateAbs, err := filepath.Abs(filepath.Clean(filepath.Join(currentAbs, assetFrom)))
	if err != nil {
		return "", metadata.NewError("template_asset_missing", "failed to resolve template asset: "+err.Error(), "Check asset.from paths.", 400)
	}
	if !within(rootAbs, candidateAbs) {
		return "", metadata.NewError(
			"template_asset_outside_root",
			"visual template asset escapes template root: "+assetFrom,
			"Keep asset.from under templates/visual, using ../_shared only inside that root.",
			400,
		)
	}
	return candidateAbs, nil
}

func safeOutputPath(outDir, assetTo string) (string, error) {
	assetTo = strings.TrimSpace(assetTo)
	if assetTo == "" || filepath.IsAbs(assetTo) {
		return "", metadata.NewError("template_asset_target_invalid", "visual output asset path must be relative: "+assetTo, "Use a relative asset.to path without parent traversal.", 400)
	}
	clean := filepath.Clean(assetTo)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", metadata.NewError("template_asset_target_invalid", "visual output asset path is unsafe: "+assetTo, "Remove parent traversal from asset.to.", 400)
	}
	outAbs, err := filepath.Abs(outDir)
	if err != nil {
		return "", metadata.NewError("output_path_invalid", "failed to resolve output directory: "+err.Error(), "Pass a valid --out directory.", 400)
	}
	targetAbs, err := filepath.Abs(filepath.Join(outAbs, clean))
	if err != nil {
		return "", metadata.NewError("output_path_invalid", "failed to resolve output path: "+err.Error(), "Check output asset paths.", 400)
	}
	if !within(outAbs, targetAbs) {
		return "", metadata.NewError("output_path_invalid", "visual output path escapes output directory: "+assetTo, "Use relative output paths inside --out.", 400)
	}
	return targetAbs, nil
}

func within(rootAbs, candidateAbs string) bool {
	rootAbs = filepath.Clean(rootAbs)
	candidateAbs = filepath.Clean(candidateAbs)
	if rootAbs == candidateAbs {
		return true
	}
	rel, err := filepath.Rel(rootAbs, candidateAbs)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func ToArtifactPath(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}
