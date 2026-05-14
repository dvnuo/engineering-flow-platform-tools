package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestJSONEnvelope(t *testing.T) {
	buf := &bytes.Buffer{}
	err := Print(buf, "json", Success("jira-main", map[string]string{"x": "y"}))
	if err != nil || !strings.Contains(buf.String(), `"ok": true`) {
		t.Fatal("bad json")
	}
}
func TestTableNoSecret(t *testing.T) {
	buf := &bytes.Buffer{}
	_ = Print(buf, "table", Failure("auth_failed", "failed", "password=secret", 401))
	if strings.Contains(buf.String(), "secret") {
		t.Fatal("secret leaked")
	}
}
