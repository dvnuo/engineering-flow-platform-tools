package tests

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func readTextFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return normalizeLineEndings(string(b))
}

func normalizeLineEndings(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

func requireLineCount(t *testing.T, path string, min int) {
	t.Helper()
	lines := strings.Count(readTextFile(t, path), "\n")
	if lines <= min {
		t.Fatalf("%s has %d lines, want more than %d", path, lines, min)
	}
}

func TestTextFormatContracts(t *testing.T) {
	goMod := readTextFile(t, "../go.mod")
	if !strings.HasPrefix(goMod, "module engineering-flow-platform-tools\n") {
		t.Fatal("go.mod missing standalone module line")
	}
	if !strings.Contains(goMod, "\ngo 1.22\n") {
		t.Fatal("go.mod missing standalone go 1.22 line")
	}

	for _, path := range []string{"../scripts/build.sh", "../scripts/smoke.sh"} {
		lines := strings.Split(readTextFile(t, path), "\n")
		if len(lines) < 3 {
			t.Fatalf("%s is not a normal multi-line shell script", path)
		}
		if lines[0] != "#!/usr/bin/env bash" {
			t.Fatalf("%s first line=%q", path, lines[0])
		}
		if path == "../scripts/build.sh" && !strings.Contains(lines[1], "set -euo pipefail") {
			t.Fatalf("%s second line missing set -euo pipefail", path)
		}
	}

	for _, path := range []string{"../.github/workflows/test.yml", "../.github/workflows/release.yml"} {
		var doc any
		if err := yaml.NewDecoder(bytes.NewReader([]byte(readTextFile(t, path)))).Decode(&doc); err != nil {
			t.Fatalf("%s is not parseable YAML: %v", path, err)
		}
	}

	attrs := readTextFile(t, "../.gitattributes")
	for _, literal := range []string{
		"go.mod text eol=lf",
		"*.sh text eol=lf",
		"*.yml text eol=lf",
		"*.md text eol=lf",
	} {
		if !strings.Contains(attrs, literal) {
			t.Fatalf(".gitattributes missing literal %q", literal)
		}
	}

	requireLineCount(t, "../README.md", 50)
	requireLineCount(t, "../docs/COMMAND_SPEC.md", 100)
	requireLineCount(t, "../docs/LLM_USAGE.md", 40)

	spec := readTextFile(t, "../docs/COMMAND_SPEC.md")
	for _, literal := range []string{
		"jira issue get <issue-or-url>",
		"jira version get <version-id>",
		"confluence content get <content-id>",
		"confluence page get --id",
		"confluence page get --url",
		"jenkins job build <job>",
		"jenkins artifact download <job> <build> <path>",
	} {
		if !strings.Contains(spec, literal) {
			t.Fatalf("COMMAND_SPEC.md missing literal %q", literal)
		}
	}

	allDocs := strings.Join([]string{
		readTextFile(t, "../README.md"),
		readTextFile(t, "../docs/COMMAND_SPEC.md"),
		readTextFile(t, "../docs/LLM_USAGE.md"),
		readTextFile(t, "../docs/ARCHITECTURE.md"),
	}, "\n")
	for _, forbidden := range []string{"efp" + "-jira", "efp" + "-confluence", "EFP" + "_ATLASSIAN_CONFIG"} {
		if strings.Contains(allDocs, forbidden) {
			t.Fatalf("forbidden user-visible token found: %s", forbidden)
		}
	}
}

func TestNormalizeLineEndings(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{name: "crlf", in: "a\r\nb\r\n", want: "a\nb\n"},
		{name: "cr", in: "a\rb\r", want: "a\nb\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeLineEndings(tc.in); got != tc.want {
				t.Fatalf("normalizeLineEndings(%q)=%q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
