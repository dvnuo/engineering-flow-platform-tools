package tests

import (
	"bufio"
	"engineering-flow-platform-tools/internal/app"
	"os"
	"strings"
	"testing"
)

func countSpec(prefix string) int {
	f, _ := os.Open("../docs/COMMAND_SPEC.md")
	defer f.Close()
	s := bufio.NewScanner(f)
	n := 0
	sec := ""
	for s.Scan() {
		l := strings.TrimSpace(s.Text())
		if l == "## Jira" {
			sec = "jira"
			continue
		}
		if l == "## Confluence" {
			sec = "confluence"
			continue
		}
		if strings.HasPrefix(l, "## ") {
			sec = ""
		}
		if sec == prefix && strings.HasPrefix(l, "- "+prefix+" ") {
			n++
		}
	}
	return n
}
func TestCommandCoverageCounts(t *testing.T) {
	if countSpec("jira") == 0 || countSpec("confluence") == 0 {
		t.Fatalf("spec parse failed")
	}
	if len(app.JiraCommandList()) < countSpec("jira") {
		t.Fatalf("jira commands missing")
	}
	if len(app.ConfluenceCommandList()) < countSpec("confluence") {
		t.Fatalf("confluence commands missing")
	}
}
