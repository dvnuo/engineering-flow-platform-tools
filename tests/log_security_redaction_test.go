package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogSecretsDoNotAppearInOutputsOrRunFiles(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")
	content := strings.Join([]string{
		`2026-06-03T10:00:00Z ERROR timeout after 3000ms Authorization: Bearer bearersecretshouldnotappear Authorization: Basic basicsecretshouldnotappear password=secret api_key=xyz api-key=apihyphenshouldnotappear apikey=apikeyshouldnotappear token=tok secret=hidden access_token=accessshouldnotappear refresh_token=refreshshouldnotappear client_secret=clientsecretshouldnotappear AWS_ACCESS_KEY_ID=akidshouldnotappear AWS_SECRET_ACCESS_KEY=awssecretshouldnotappear AWS_SESSION_TOKEN=awssessionshouldnotappear X-API-Key: xapikeyshouldnotappear Cookie: sessionid=cookieshouldnotappear; csrftoken=csrfshouldnotappear`,
		`Set-Cookie: sid=setcookieshouldnotappear`,
		`user@example.test`,
		"Traceback (most recent call last):",
		`  File "/srv/app.py", line 10, in main`,
		"Exception: boom",
	}, "\n") + "\n"
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(dir, "run")
	var combined strings.Builder
	for _, args := range [][]string{
		{"analyze", "--source", logPath, "--run", runDir, "--json"},
		{"search", "--run", runDir, "--query", "timeout", "--json"},
		{"window", "--run", runDir, "--entry-id", "entry_000001", "--before", "0", "--after", "3", "--json"},
		{"extract", "--run", runDir, "--kind", "stacktrace", "--json"},
	} {
		out, _ := runLog(t, args...)
		combined.Write(out)
	}
	for _, name := range []string{"entries.jsonl", "templates.json"} {
		b, err := os.ReadFile(filepath.Join(runDir, name))
		if err != nil {
			t.Fatal(err)
		}
		combined.Write(b)
	}
	for _, leak := range []string{
		"bearersecretshouldnotappear",
		"basicsecretshouldnotappear",
		"password=secret",
		"api_key=xyz",
		"api-key=apihyphenshouldnotappear",
		"apikeyshouldnotappear",
		"token=tok",
		"secret=hidden",
		"accessshouldnotappear",
		"refreshshouldnotappear",
		"clientsecretshouldnotappear",
		"akidshouldnotappear",
		"awssecretshouldnotappear",
		"awssessionshouldnotappear",
		"xapikeyshouldnotappear",
		"cookieshouldnotappear",
		"csrfshouldnotappear",
		"setcookieshouldnotappear",
		"user@example.test",
	} {
		assertNoLiteral(t, combined.String(), leak)
	}
}
