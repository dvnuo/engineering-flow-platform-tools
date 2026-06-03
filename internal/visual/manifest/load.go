package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"

	"engineering-flow-platform-tools/internal/visual/metadata"
	"gopkg.in/yaml.v3"
)

func LoadRegistry(templateDir string) (Registry, error) {
	var registry Registry
	path := filepath.Join(templateDir, "registry.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return registry, metadata.NewError(
			"template_registry_missing",
			"visual template registry was not found: "+path,
			"Pass --template-dir pointing to a directory containing registry.json.",
			404,
		)
	}
	if err := json.Unmarshal(b, &registry); err != nil {
		return registry, metadata.NewError(
			"template_registry_invalid",
			"visual template registry is invalid JSON: "+err.Error(),
			"Fix templates/visual/registry.json.",
			400,
		)
	}
	if registry.Version == 0 || len(registry.Templates) == 0 {
		return registry, metadata.NewError(
			"template_registry_invalid",
			"visual template registry must contain version and templates.",
			"Add version and at least one template entry to registry.json.",
			400,
		)
	}
	if err := ValidateRegistry(registry); err != nil {
		return registry, err
	}
	return registry, nil
}

func LoadTemplateManifest(templateDir string, entry RegistryEntry) (TemplateManifest, error) {
	var m TemplateManifest
	path := filepath.Join(templateDir, filepath.Clean(entry.Path))
	b, err := os.ReadFile(path)
	if err != nil {
		return m, metadata.NewError(
			"template_manifest_invalid",
			"visual template manifest was not found for "+entry.ID+": "+path,
			"Ensure registry.json path points to an existing template.yaml.",
			404,
		)
	}
	if err := yaml.Unmarshal(b, &m); err != nil {
		return m, metadata.NewError(
			"template_manifest_invalid",
			"visual template manifest is invalid YAML for "+entry.ID+": "+err.Error(),
			"Fix the template.yaml file.",
			400,
		)
	}
	return m, nil
}
