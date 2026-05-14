package tests

import (
	"bufio"
	"engineering-flow-platform-tools/internal/app"
	"os"
	"sort"
	"strings"
	"testing"
)

func readSpecCommands(prefix string) []string {
	f, err := os.Open("../docs/COMMAND_SPEC.md")
	if err != nil {
		return nil
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

func equalList(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestCommandCoverageCounts(t *testing.T) {
	jiraSpec := readSpecCommands("jira")
	confSpec := readSpecCommands("confluence")
	if len(jiraSpec) == 0 || len(confSpec) == 0 {
		t.Fatalf("spec parse failed")
	}
	jiraReg := append([]string{}, app.JiraCommandList()...)
	confReg := append([]string{}, app.ConfluenceCommandList()...)
	sort.Strings(jiraReg)
	sort.Strings(confReg)
	if !equalList(jiraReg, jiraSpec) {
		t.Fatalf("jira registry/docs mismatch: registry=%d docs=%d", len(jiraReg), len(jiraSpec))
	}
	if !equalList(confReg, confSpec) {
		t.Fatalf("confluence registry/docs mismatch: registry=%d docs=%d", len(confReg), len(confSpec))
	}
}
