package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"engineering-flow-platform-tools/internal/testutil"
)

func runJenkins(t *testing.T, cfg string, args ...string) map[string]any {
	t.Helper()
	root := NewRoot()
	var b bytes.Buffer
	root.SetOut(&b)
	root.SetErr(&b)
	root.SetArgs(append([]string{"--config", cfg, "--json"}, args...))
	if err := root.Execute(); err != nil {
		t.Fatalf("execute failed: %v out=%s", err, b.String())
	}
	var out map[string]any
	if err := json.Unmarshal(b.Bytes(), &out); err != nil {
		t.Fatalf("invalid json: %v out=%s", err, b.String())
	}
	return out
}

func requireJenkinsOK(t *testing.T, out map[string]any) map[string]any {
	t.Helper()
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("command failed: %#v", out)
	}
	data, _ := out["data"].(map[string]any)
	return data
}

func TestJenkinsBuildQueueStatusLogAndArtifact(t *testing.T) {
	mock := testutil.NewMockJenkins(t)
	cfg, err := testutil.WriteConfig(testutil.JenkinsConfig(mock.Server.URL))
	if err != nil {
		t.Fatal(err)
	}
	data := requireJenkinsOK(t, runJenkins(t, cfg, "job", "build-with-params", "folder/app-main", "--param", "BRANCH=main"))
	if data["queue_id"] != "123" || mock.LastParamRef != "main" {
		t.Fatalf("bad build trigger data=%#v param=%q", data, mock.LastParamRef)
	}
	requireJenkinsOK(t, runJenkins(t, cfg, "queue", "get", "123"))
	status := requireJenkinsOK(t, runJenkins(t, cfg, "build", "status", "folder/app-main", "42"))
	if status["state"] != "success" {
		t.Fatalf("state=%#v", status)
	}
	log := requireJenkinsOK(t, runJenkins(t, cfg, "build", "log", "folder/app-main", "42"))
	if log["text"] == "" {
		t.Fatalf("missing log text: %#v", log)
	}
	out := filepath.Join(t.TempDir(), "app.jar")
	artifact := requireJenkinsOK(t, runJenkins(t, cfg, "artifact", "download", "folder/app-main", "42", "target/app.jar", "--output", out))
	if artifact["path"] != out {
		t.Fatalf("download path=%#v want %q", artifact["path"], out)
	}
	if b, err := os.ReadFile(out); err != nil || string(b) != "binary" {
		t.Fatalf("downloaded artifact=%q err=%v", string(b), err)
	}
	if mock.CrumbHits == 0 {
		t.Fatalf("expected crumb fetch for write command")
	}
}

func TestJenkinsDryRunDoesNotHitServer(t *testing.T) {
	mock := testutil.NewMockJenkins(t)
	cfg, err := testutil.WriteConfig(testutil.JenkinsConfig(mock.Server.URL))
	if err != nil {
		t.Fatal(err)
	}
	before := mock.Hits
	data := requireJenkinsOK(t, runJenkins(t, cfg, "--dry-run", "job", "build", "folder/app-main"))
	if data["dry_run"] != true {
		t.Fatalf("missing dry_run data: %#v", data)
	}
	if mock.Hits != before {
		t.Fatalf("dry-run hit server: before=%d after=%d", before, mock.Hits)
	}
}

func TestJenkinsCommandsExposeMetadata(t *testing.T) {
	root := NewRoot()
	var b bytes.Buffer
	root.SetOut(&b)
	root.SetErr(&b)
	root.SetArgs([]string{"commands", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	obj := testutil.AssertOKEnvelope(t, b.Bytes())
	data, _ := obj["data"].(map[string]any)
	commands, _ := data["commands"].([]any)
	if len(commands) < 20 {
		t.Fatalf("too few commands: %d", len(commands))
	}
}
