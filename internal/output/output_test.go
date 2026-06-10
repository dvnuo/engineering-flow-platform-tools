package output

import (
	"bytes"
	"encoding/json"
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

func TestJSONRedactsSensitiveOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	err := Print(buf, "json", Success("jira-main", map[string]any{
		"token":             "secret-token-should-not-appear",
		"jobProgressToken":  "secret-job-token-should-not-appear",
		"token_state":       "refreshable",
		"max_output_tokens": 1200,
		"profile_url":       "https://example.test/callback?access_token=secret-token-should-not-appear&ok=1#frag",
		"message":           `Authorization: Bearer secret-token-should-not-appear {"api_key":"secret-api-key-should-not-appear"} cookie: session=secret-password-should-not-appear`,
		"nested": map[string]any{
			"password": "secret-password-should-not-appear",
			"note":     "tid=secret-token-should-not-appear",
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	for _, leaked := range []string{
		"secret-token-should-not-appear",
		"secret-api-key-should-not-appear",
		"secret-password-should-not-appear",
		"Bearer secret",
		"tid=secret",
		"access_token=secret",
	} {
		if strings.Contains(got, leaked) {
			t.Fatalf("secret leaked %q in:\n%s", leaked, got)
		}
	}
	for _, want := range []string{
		`"token": "***REDACTED***"`,
		`"jobProgressToken": "***REDACTED***"`,
		`"token_state": "refreshable"`,
		`"max_output_tokens": 1200`,
		`ok=1`,
		`access_token=REDACTED`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("redacted output is not valid json: %v", err)
	}
}

func TestYAMLRedactsSensitiveOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	err := Print(buf, "yaml", Success("", map[string]any{
		"api_key": "secret-api-key-should-not-appear",
		"url":     "https://example.test/path?code=secret-token-should-not-appear&safe=yes",
	}))
	if err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if strings.Contains(got, "secret-api-key-should-not-appear") || strings.Contains(got, "code=secret") {
		t.Fatalf("secret leaked in yaml:\n%s", got)
	}
	if !strings.Contains(got, Redacted) || !strings.Contains(got, "safe=yes") {
		t.Fatalf("expected redacted yaml with safe query retained:\n%s", got)
	}
}

func TestFailureRedactsMessageHintAndFileLists(t *testing.T) {
	buf := &bytes.Buffer{}
	env := Failure(
		"server_error",
		`request failed Authorization: Bearer secret-token-should-not-appear`,
		`retry without password=secret-password-should-not-appear`,
		500,
	)
	env.Error.File = `C:\tmp\token=secret-token-should-not-appear\out.json`
	env.Error.MissingFiles = []string{`https://example.test/a?token=secret-token-should-not-appear`}
	err := Print(buf, "json", env)
	if err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	for _, leaked := range []string{"secret-token-should-not-appear", "secret-password-should-not-appear", "Bearer secret"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("secret leaked %q in:\n%s", leaked, got)
		}
	}
}

func TestRedactionPreservesLargeJSONNumbers(t *testing.T) {
	const large = int64(9223372036854775807)
	buf := &bytes.Buffer{}
	err := Print(buf, "json", Success("", map[string]any{"id": large, "token_state": "refreshable"}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "9223372036854775807") {
		t.Fatalf("large integer was not preserved:\n%s", buf.String())
	}
}
