package automation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDevToolsClientParsesVersionAndList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json/version":
			_, _ = w.Write([]byte(`{
				"Browser": "Chrome/126.0",
				"Protocol-Version": "1.3",
				"User-Agent": "agent",
				"V8-Version": "12.6",
				"WebKit-Version": "537.36",
				"webSocketDebuggerUrl": "ws://127.0.0.1/devtools/browser/abc"
			}`))
		case "/json/list":
			_, _ = w.Write([]byte(`[
				{
					"id": "page-1",
					"type": "page",
					"title": "Home",
					"url": "https://intranet.example.test/app?code=secret",
					"webSocketDebuggerUrl": "ws://127.0.0.1/devtools/page/page-1"
				},
				{
					"id": "worker-1",
					"type": "service_worker",
					"title": "Worker",
					"url": "https://intranet.example.test/sw.js",
					"webSocketDebuggerUrl": "ws://127.0.0.1/devtools/page/worker-1"
				}
			]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := &DevToolsClient{BaseURL: srv.URL, HTTPClient: srv.Client()}
	version, err := client.Version(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if version.Browser != "Chrome/126.0" || version.WebSocketDebuggerURL == "" {
		t.Fatalf("version = %#v", version)
	}
	targets, err := client.ListTargets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 || targets[0].ID != "page-1" || targets[0].WebSocketDebuggerURL == "" {
		t.Fatalf("targets = %#v", targets)
	}
	pages := PageTargets(targets)
	if len(pages) != 1 || pages[0].Type != "page" {
		t.Fatalf("pages = %#v", pages)
	}
	redacted := RedactedTarget(targets[0])
	if redacted.URL != "https://intranet.example.test/app?code=REDACTED" {
		t.Fatalf("redacted URL = %q", redacted.URL)
	}
}
