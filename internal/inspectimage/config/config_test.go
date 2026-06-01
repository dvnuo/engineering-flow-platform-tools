package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDefaultPathUsesCopilotHome(t *testing.T) {
	t.Setenv(EnvConfigPath, "")
	home := t.TempDir()
	t.Setenv(EnvCopilotHome, home)
	got, err := ResolvePath("")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "inspect-image.json")
	if got != want {
		t.Fatalf("path=%q want %q", got, want)
	}
}

func TestEnvConfigOverridesPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom.json")
	t.Setenv(EnvConfigPath, path)
	got, err := ResolvePath("")
	if err != nil {
		t.Fatal(err)
	}
	if got != path {
		t.Fatalf("path=%q want %q", got, path)
	}
}

func TestDefaultPathUsesHomeCopilot(t *testing.T) {
	t.Setenv(EnvConfigPath, "")
	t.Setenv(EnvCopilotHome, "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	got, err := ResolvePath("")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(home, ".copilot", "inspect-image.json") {
		t.Fatalf("path=%q", got)
	}
}

func TestSaveUses0600WhereSupported(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose POSIX 0600 semantics through os.FileMode")
	}
	path := filepath.Join(t.TempDir(), "inspect-image.json")
	if err := Save(path, Default()); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("mode too open: %v", info.Mode().Perm())
	}
}
