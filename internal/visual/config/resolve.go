package config

import (
	"os"
	"path/filepath"
	"strings"

	sharedconfig "engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/visual/metadata"
)

const EnvTemplateDir = "EFP_VISUAL_TEMPLATE_DIR"

func ResolveTemplateDir(flagTemplateDir string, flagConfig string) (string, error) {
	if p := strings.TrimSpace(flagTemplateDir); p != "" {
		return ensureTemplateDir(p)
	}
	if p := strings.TrimSpace(os.Getenv(EnvTemplateDir)); p != "" {
		return ensureTemplateDir(p)
	}
	if p := configTemplateDir(flagConfig); p != "" {
		return ensureTemplateDir(p)
	}
	if p, err := DefaultTemplateDir(); err == nil {
		if p, ok := existingTemplateDir(p); ok {
			return p, nil
		}
	}
	if p, ok := existingTemplateDir(filepath.Join(".", "templates", "visual")); ok {
		return p, nil
	}
	if exe, err := os.Executable(); err == nil && exe != "" {
		exeDir := filepath.Dir(exe)
		for _, candidate := range []string{
			filepath.Join(exeDir, "templates", "visual"),
			filepath.Join(exeDir, "..", "share", "efp-tools", "visual", "templates"),
		} {
			if p, ok := existingTemplateDir(candidate); ok {
				return p, nil
			}
		}
	}
	return "", metadata.NewError(
		"template_dir_missing",
		"visual template directory was not found.",
		"Pass --template-dir, set EFP_VISUAL_TEMPLATE_DIR, configure visual.template_dir, install templates under ~/.efp/template/visual, or run from a checkout containing ./templates/visual.",
		404,
	)
}

func DefaultTemplateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".efp", "template", "visual"), nil
}

func configTemplateDir(flagConfig string) string {
	cfgPath := strings.TrimSpace(flagConfig)
	if cfgPath == "" {
		cfgPath = strings.TrimSpace(os.Getenv(sharedconfig.EnvConfigPath))
	}
	if cfgPath == "" {
		p, err := sharedconfig.DefaultPath()
		if err != nil {
			return ""
		}
		cfgPath = p
	}
	if cfgPath == "" {
		return ""
	}
	if _, err := os.Stat(cfgPath); err != nil {
		return ""
	}
	cfg, err := sharedconfig.Load(cfgPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cfg.Visual.TemplateDir)
}

func ensureTemplateDir(path string) (string, error) {
	clean := filepath.Clean(expandUserPath(path))
	info, err := os.Stat(clean)
	if err != nil || !info.IsDir() {
		return "", metadata.NewError(
			"template_dir_missing",
			"visual template directory does not exist or is not a directory: "+clean,
			"Pass a directory that contains templates/visual/registry.json.",
			404,
		)
	}
	return clean, nil
}

func existingTemplateDir(path string) (string, bool) {
	clean := filepath.Clean(expandUserPath(path))
	info, err := os.Stat(clean)
	if err != nil || !info.IsDir() {
		return "", false
	}
	return clean, true
}

func expandUserPath(path string) string {
	p := strings.TrimSpace(path)
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return p
		}
		if p == "~" {
			return home
		}
		return filepath.Join(home, strings.TrimLeft(p[1:], `/\`))
	}
	return p
}
