package tests

import (
	"bufio"
	"os"
	"sort"
	"strings"
	"testing"

	"engineering-flow-platform-tools/internal/catalog"
	ccmd "engineering-flow-platform-tools/internal/confluence/commands"
	jcmd "engineering-flow-platform-tools/internal/jira/commands"
	"github.com/spf13/cobra"
)

func readSpecCommands(t *testing.T, prefix string) []string {
	t.Helper()
	f, err := openSpec()
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []string
	sec := ""
	s := bufio.NewScanner(f)
	for s.Scan() {
		l := strings.TrimSpace(s.Text())
		switch l {
		case "## Jira":
			sec = "jira"
			continue
		case "## Confluence":
			sec = "confluence"
			continue
		}
		if strings.HasPrefix(l, "## ") {
			sec = ""
		}
		if sec == prefix && strings.HasPrefix(l, "- "+prefix+" ") {
			out = append(out, strings.TrimPrefix(l, "- "))
		}
	}
	sort.Strings(out)
	return out
}

func WalkCobraCommands(root *cobra.Command) []string {
	var out []string
	var walk func(*cobra.Command, []string)
	walk = func(cmd *cobra.Command, parent []string) {
		for _, child := range cmd.Commands() {
			if child.Hidden || (child.Name() == "help" && strings.TrimSpace(child.Use) == "help") {
				continue
			}
			parts := append(append([]string{}, parent...), strings.Fields(child.Use)...)
			visibleChildren := 0
			for _, grand := range child.Commands() {
				if !grand.Hidden && !(grand.Name() == "help" && strings.TrimSpace(grand.Use) == "help") {
					visibleChildren++
				}
			}
			if visibleChildren == 0 {
				out = append(out, strings.Join(parts, " "))
				continue
			}
			if child.RunE != nil || child.Run != nil {
				out = append(out, strings.Join(parts, " "))
			}
			walk(child, parts)
		}
	}
	walk(root, []string{root.Use})
	sort.Strings(out)
	return out
}

func openSpec() (*os.File, error) {
	return os.Open("../docs/COMMAND_SPEC.md")
}

func assertSame(t *testing.T, name string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s mismatch count got=%d want=%d\nmissing=%v\nextra=%v", name, len(got), len(want), diff(want, got), diff(got, want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s mismatch\nmissing=%v\nextra=%v", name, diff(want, got), diff(got, want))
		}
	}
}

func diff(a, b []string) []string {
	set := map[string]bool{}
	for _, x := range b {
		set[x] = true
	}
	var out []string
	for _, x := range a {
		if !set[x] {
			out = append(out, x)
		}
	}
	return out
}

func TestCommandCoverage(t *testing.T) {
	jiraSpec := readSpecCommands(t, "jira")
	confSpec := readSpecCommands(t, "confluence")
	assertSame(t, "jira cobra/docs", WalkCobraCommands(jcmd.NewRoot()), jiraSpec)
	assertSame(t, "confluence cobra/docs", WalkCobraCommands(ccmd.NewRoot()), confSpec)
	assertSame(t, "jira catalog/docs", catalog.SortedUsages("jira"), jiraSpec)
	assertSame(t, "confluence catalog/docs", catalog.SortedUsages("confluence"), confSpec)
	for _, product := range []string{"jira", "confluence"} {
		for _, item := range catalog.Commands(product) {
			if item.Risk != "read" && item.Risk != "write" && item.Risk != "write_requires_confirmation" && item.Risk != "delete" && item.Risk != "admin" {
				t.Fatalf("%s has invalid risk %q", item.Usage, item.Risk)
			}
			if len(item.Examples) == 0 {
				t.Fatalf("%s has no examples", item.Usage)
			}
		}
	}
}
