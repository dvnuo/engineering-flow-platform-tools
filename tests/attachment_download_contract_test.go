package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"engineering-flow-platform-tools/internal/testutil"
)

func attachmentDownloadConfig(t *testing.T, baseURL string) string {
	t.Helper()
	cfg, err := testutil.WriteConfig(testutil.JiraConfig(baseURL))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func errorCode(t *testing.T, out map[string]any) string {
	t.Helper()
	errObj, _ := out["error"].(map[string]any)
	code, _ := errObj["code"].(string)
	return code
}

func errorMessage(t *testing.T, out map[string]any) string {
	t.Helper()
	errObj, _ := out["error"].(map[string]any)
	msg, _ := errObj["message"].(string)
	return msg
}

func requireEnvelopeFailure(t *testing.T, out map[string]any, code string) {
	t.Helper()
	if ok, _ := out["ok"].(bool); ok {
		t.Fatalf("unexpected success: %#v", out)
	}
	if got := errorCode(t, out); got != code {
		t.Fatalf("error.code=%q want %q: %#v", got, code, out)
	}
}

func requireNoSecretText(t *testing.T, out map[string]any) {
	t.Helper()
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	rendered := strings.ToLower(string(b))
	for _, key := range []string{"password", "api_key", "token", "authorization"} {
		if strings.Contains(rendered, key) {
			t.Fatalf("secret-related text leaked: %#v", out)
		}
	}
}

func TestJiraAttachmentDownloadContracts(t *testing.T) {
	var mode string
	contentHits := 0
	var serverURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/api/2/attachment/10000":
			switch mode {
			case "missing-content":
				_, _ = w.Write([]byte(`{"id":"10000"}`))
			case "off-instance":
				_, _ = w.Write([]byte(`{"id":"10000","content":"https://evil.example/file.bin"}`))
			default:
				_, _ = w.Write([]byte(`{"id":"10000","content":"` + serverURL + `/download.bin"}`))
			}
		case "/download.bin":
			contentHits++
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte("jira attachment bytes"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	serverURL = srv.URL
	cfg := attachmentDownloadConfig(t, srv.URL)

	t.Run("success writes output", func(t *testing.T) {
		mode = ""
		contentHits = 0
		outPath := filepath.Join(t.TempDir(), "out.bin")
		out := runSemantic(t, "jira", cfg, "attachment", "download", "10000", "--output", outPath)
		if ok, _ := out["ok"].(bool); !ok {
			t.Fatalf("download failed: %#v", out)
		}
		b, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != "jira attachment bytes" {
			t.Fatalf("output content=%q", b)
		}
		if contentHits != 1 {
			t.Fatalf("contentHits=%d want 1", contentHits)
		}
	})

	t.Run("write failure returns invalid args", func(t *testing.T) {
		mode = ""
		outPath := filepath.Join(t.TempDir(), "missing", "out.bin")
		out := runSemantic(t, "jira", cfg, "attachment", "download", "10000", "--output", outPath)
		requireEnvelopeFailure(t, out, "invalid_args")
		if !strings.Contains(errorMessage(t, out), "failed to write --output") {
			t.Fatalf("message missing write failure: %#v", out)
		}
		requireNoSecretText(t, out)
	})

	t.Run("missing content returns not found", func(t *testing.T) {
		mode = "missing-content"
		out := runSemantic(t, "jira", cfg, "attachment", "download", "10000", "--output", filepath.Join(t.TempDir(), "out.bin"))
		requireEnvelopeFailure(t, out, "not_found")
	})

	t.Run("off instance content url returns mismatch", func(t *testing.T) {
		mode = "off-instance"
		contentHits = 0
		out := runSemantic(t, "jira", cfg, "attachment", "download", "10000", "--output", filepath.Join(t.TempDir(), "out.bin"))
		requireEnvelopeFailure(t, out, "instance_url_mismatch")
		if contentHits != 0 {
			t.Fatalf("off-instance path was fetched: %d", contentHits)
		}
	})
}

func TestConfluenceAttachmentDownloadContracts(t *testing.T) {
	var mode string
	contentHits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/content/10000":
			w.Header().Set("Content-Type", "application/json")
			switch mode {
			case "malformed":
				_, _ = w.Write([]byte(`{`))
			case "off-instance":
				_, _ = w.Write([]byte(`{"id":"10000","_links":{"download":"https://evil.example/file.bin"}}`))
			default:
				_, _ = w.Write([]byte(`{"id":"10000","_links":{"download":"/download.bin"}}`))
			}
		case "/download.bin":
			contentHits++
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte("confluence attachment bytes"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	cfg := attachmentDownloadConfig(t, srv.URL)

	t.Run("success writes output", func(t *testing.T) {
		mode = ""
		contentHits = 0
		outPath := filepath.Join(t.TempDir(), "out.bin")
		out := runSemantic(t, "confluence", cfg, "attachment", "download", "10000", "--output", outPath)
		if ok, _ := out["ok"].(bool); !ok {
			t.Fatalf("download failed: %#v", out)
		}
		b, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != "confluence attachment bytes" {
			t.Fatalf("output content=%q", b)
		}
		if contentHits != 1 {
			t.Fatalf("contentHits=%d want 1", contentHits)
		}
	})

	t.Run("without output returns metadata", func(t *testing.T) {
		mode = ""
		contentHits = 0
		out := runSemantic(t, "confluence", cfg, "attachment", "download", "10000")
		if ok, _ := out["ok"].(bool); !ok {
			t.Fatalf("metadata output failed: %#v", out)
		}
		if contentHits != 0 {
			t.Fatalf("metadata-only path fetched content: %d", contentHits)
		}
		data, _ := out["data"].(map[string]any)
		if _, ok := data["metadata"].(map[string]any); !ok {
			t.Fatalf("metadata missing from data: %#v", out)
		}
	})

	t.Run("write failure returns invalid args", func(t *testing.T) {
		mode = ""
		outPath := filepath.Join(t.TempDir(), "missing", "out.bin")
		out := runSemantic(t, "confluence", cfg, "attachment", "download", "10000", "--output", outPath)
		requireEnvelopeFailure(t, out, "invalid_args")
		if !strings.Contains(errorMessage(t, out), "failed to write --output") {
			t.Fatalf("message missing write failure: %#v", out)
		}
		requireNoSecretText(t, out)
	})

	t.Run("malformed metadata returns server error", func(t *testing.T) {
		mode = "malformed"
		out := runSemantic(t, "confluence", cfg, "attachment", "download", "10000", "--output", filepath.Join(t.TempDir(), "out.bin"))
		requireEnvelopeFailure(t, out, "server_error")
	})

	t.Run("off instance download url returns mismatch", func(t *testing.T) {
		mode = "off-instance"
		contentHits = 0
		out := runSemantic(t, "confluence", cfg, "attachment", "download", "10000", "--output", filepath.Join(t.TempDir(), "out.bin"))
		requireEnvelopeFailure(t, out, "instance_url_mismatch")
		if contentHits != 0 {
			t.Fatalf("off-instance path was fetched: %d", contentHits)
		}
	})
}
