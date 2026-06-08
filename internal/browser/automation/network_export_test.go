package automation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNetworkExportEntriesRedactsFiltersAndLimits(t *testing.T) {
	entries := []NetworkRecordEntry{
		{URL: "https://intranet.test/api/me?access_token=secret", Method: "get", Status: 200, ResourceType: "fetch", InitiatorType: "fetch", Source: "fetch"},
		{URL: "https://intranet.test/static/app.js", Method: "GET", Status: 200, ResourceType: "script", Source: "resource_timing"},
		{URL: "https://intranet.test/api/orders?code=abc", Method: "POST", Status: 201, ResourceType: "xhr", Source: "xhr"},
	}
	got, count := networkExportEntries(entries, NetworkExportOptions{OutPath: "out.json", Format: "json", Filter: "/api/", Limit: 1})
	if count != 2 || len(got) != 1 {
		t.Fatalf("count=%d entries=%#v", count, got)
	}
	if strings.Contains(got[0].URL, "access_token=secret") || got[0].Method != "GET" {
		t.Fatalf("entry was not sanitized: %#v", got[0])
	}
}

func TestNetworkExportWritesHARLiteMetadataOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json/version":
			_, _ = w.Write([]byte(`{"webSocketDebuggerUrl":"ws://127.0.0.1/devtools/browser/abc"}`))
		case "/json/list":
			_, _ = w.Write([]byte(`[{"id":"page-1","type":"page","title":"Home","url":"https://intranet.test/app","webSocketDebuggerUrl":"ws://127.0.0.1/devtools/page/page-1"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	host, port := splitHostPort(t, srv.Listener.Addr().String())

	store := NewStore(t.TempDir())
	if err := store.Save(Session{
		Name:           "default",
		DebugAddr:      host,
		DebugPort:      port,
		CreatedAt:      time.Now().UTC(),
		Alive:          true,
		ActiveTargetID: "page-1",
	}); err != nil {
		t.Fatal(err)
	}
	artifactPath, err := store.NetworkArtifactPath("default", "page-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o700); err != nil {
		t.Fatal(err)
	}
	artifact := NetworkRecorderArtifact{
		Session:  "default",
		TargetID: "page-1",
		Limit:    500,
		Count:    1,
		Entries: []NetworkRecordEntry{{
			URL:                  RedactURL("https://intranet.test/api/me?access_token=secret"),
			Method:               "GET",
			Status:               200,
			ResourceType:         "fetch",
			InitiatorType:        "fetch",
			StartedAt:            "2026-06-08T00:00:00Z",
			DurationMilliseconds: 25,
			TransferSizeBytes:    100,
			Source:               "fetch",
		}},
		UpdatedAt:  time.Now().UTC(),
		Limitation: networkRecorderLimitation,
	}
	b, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, append(b, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(t.TempDir(), "network.har-lite.json")
	mgr := NewManager(store, nil)
	result, err := mgr.NetworkExport(context.Background(), NetworkExportOptions{
		PageOptions: PageOptions{SessionName: "default", TimeoutSeconds: 5},
		OutPath:     outPath,
		Format:      "har-lite",
		Filter:      "/api/",
		Limit:       10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Path != outPath || result.Format != "har-lite" || result.Count != 1 || result.Bytes == 0 {
		t.Fatalf("unexpected export result: %#v", result)
	}
	written, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(written)
	for _, leaked := range []string{"access_token=secret", `"headers"`, `"cookies"`, `"body"`, `"authorization"`} {
		if strings.Contains(strings.ToLower(text), leaked) {
			t.Fatalf("network export leaked %q in %s", leaked, text)
		}
	}
	if !strings.Contains(text, `"request"`) || !strings.Contains(text, `"response"`) || !strings.Contains(text, `"sizes"`) {
		t.Fatalf("har-lite metadata missing: %s", text)
	}
}
