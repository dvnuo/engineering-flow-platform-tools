package tests

import (
	"bytes"
	"os"
	"strings"
	"testing"

	ccmd "engineering-flow-platform-tools/internal/confluence/commands"
	jcmd "engineering-flow-platform-tools/internal/jira/commands"
	"engineering-flow-platform-tools/internal/testutil"
)

func TestVersionJSONContract(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(*bytes.Buffer) error
	}{
		{name: "jira", run: func(b *bytes.Buffer) error {
			cmd := jcmd.NewRoot()
			cmd.SetOut(b)
			cmd.SetErr(b)
			cmd.SetArgs([]string{"version", "--json"})
			return cmd.Execute()
		}},
		{name: "confluence", run: func(b *bytes.Buffer) error {
			cmd := ccmd.NewRoot()
			cmd.SetOut(b)
			cmd.SetErr(b)
			cmd.SetArgs([]string{"version", "--json"})
			return cmd.Execute()
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var b bytes.Buffer
			if err := tc.run(&b); err != nil {
				t.Fatal(err)
			}
			obj := testutil.AssertJSONEnvelope(t, b.Bytes())
			if obj["ok"] != true {
				t.Fatalf("version failed: %s", b.String())
			}
			data := obj["data"].(map[string]any)
			for _, k := range []string{"version", "commit", "date"} {
				if data[k] == "" {
					t.Fatalf("missing %s in %v", k, data)
				}
			}
		})
	}
}

func TestDocsAndScriptsExist(t *testing.T) {
	for _, path := range []string{
		"../docs/INSTALL.md",
		"../docs/RELEASE.md",
		"../docs/SECURITY.md",
		"../docs/TROUBLESHOOTING.md",
		"../scripts/smoke.sh",
		"../scripts/smoke.ps1",
		"../.github/workflows/release.yml",
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s missing: %v", path, err)
		}
	}
}

func TestBuildScriptsListRequiredTargets(t *testing.T) {
	for _, path := range []string{"../scripts/build.sh", "../scripts/build.ps1"} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		for _, target := range []string{"linux", "amd64", "linux", "arm64", "darwin", "amd64", "darwin", "arm64", "windows", "amd64", "windows", "arm64"} {
			if !strings.Contains(s, target) {
				t.Fatalf("%s missing target token %s", path, target)
			}
		}
		if !strings.Contains(s, "-ldflags") || !strings.Contains(s, "internal/version.Version") {
			t.Fatalf("%s does not inject version ldflags", path)
		}
	}
}

func TestYAMLFormatAndVerboseDoNotLeak(t *testing.T) {
	var b bytes.Buffer
	cmd := jcmd.NewRoot()
	cmd.SetOut(&b)
	cmd.SetErr(&b)
	cmd.SetArgs([]string{"--format", "yaml", "--verbose", "version"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, "ok: true") || !strings.Contains(out, "version:") {
		t.Fatalf("unexpected yaml output: %s", out)
	}
	assertNoSecrets(t, out)
}
