package config

import (
	"os"
	"path/filepath"
	"testing"

	sharedconfig "engineering-flow-platform-tools/internal/config"
)

func TestResolveTemplateDirUsesEFPHomeDefaultBeforeCheckout(t *testing.T) {
	home := setVisualConfigTestHome(t)
	cwd := t.TempDir()
	mustMkdir(t, filepath.Join(home, ".efp", "template", "visual"))
	mustMkdir(t, filepath.Join(cwd, "templates", "visual"))
	chdir(t, cwd)

	got, err := ResolveTemplateDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".efp", "template", "visual")
	if got != want {
		t.Fatalf("expected default EFP template dir %q, got %q", want, got)
	}
}

func TestResolveTemplateDirFallsBackToWorkspaceTemplates(t *testing.T) {
	setVisualConfigTestHome(t)
	cwd := t.TempDir()
	mustMkdir(t, filepath.Join(cwd, "templates", "visual"))
	chdir(t, cwd)

	got, err := ResolveTemplateDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Clean(filepath.Join(".", "templates", "visual"))
	if got != want {
		t.Fatalf("expected workspace template dir %q, got %q", want, got)
	}
}

func TestResolveTemplateDirExpandsTildeForExplicitPaths(t *testing.T) {
	home := setVisualConfigTestHome(t)
	mustMkdir(t, filepath.Join(home, ".efp", "template", "visual"))

	got, err := ResolveTemplateDir("~/.efp/template/visual", "")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".efp", "template", "visual")
	if got != want {
		t.Fatalf("expected tilde-expanded template dir %q, got %q", want, got)
	}
}

func TestResolveTemplateDirExpandsTildeFromConfig(t *testing.T) {
	home := setVisualConfigTestHome(t)
	mustMkdir(t, filepath.Join(home, ".efp", "template", "visual"))
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfg, []byte("visual:\n  template_dir: ~/.efp/template/visual\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveTemplateDir("", cfg)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".efp", "template", "visual")
	if got != want {
		t.Fatalf("expected config tilde-expanded template dir %q, got %q", want, got)
	}
}

func setVisualConfigTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
	t.Setenv(EnvTemplateDir, "")
	t.Setenv(sharedconfig.EnvConfigPath, "")
	return home
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func chdir(t *testing.T, path string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	})
}
