package automation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCurrentTabChoosesFirstPageAndPersistsActiveID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json/version":
			_, _ = w.Write([]byte(`{"webSocketDebuggerUrl":"ws://127.0.0.1/devtools/browser/abc"}`))
		case "/json/list":
			_, _ = w.Write([]byte(`[
				{"id":"worker-1","type":"service_worker","title":"Worker","url":"https://intranet.example.test/sw.js","webSocketDebuggerUrl":"ws://127.0.0.1/devtools/page/worker-1"},
				{"id":"page-1","type":"page","title":"Home","url":"https://intranet.example.test/app?code=secret","webSocketDebuggerUrl":"ws://127.0.0.1/devtools/page/page-1"},
				{"id":"page-2","type":"page","title":"Other","url":"https://intranet.example.test/other","webSocketDebuggerUrl":"ws://127.0.0.1/devtools/page/page-2"}
			]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	host, port := splitHostPort(t, srv.Listener.Addr().String())

	store := NewStore(t.TempDir())
	if err := store.Save(Session{
		Name:      "default",
		DebugAddr: host,
		DebugPort: port,
		CreatedAt: time.Now().UTC(),
		Alive:     true,
	}); err != nil {
		t.Fatal(err)
	}
	mgr := NewManager(store, nil)
	result, err := mgr.CurrentTab(context.Background(), "default")
	if err != nil {
		t.Fatal(err)
	}
	if result.Tab.ID != "page-1" || !result.Tab.Active {
		t.Fatalf("tab result = %#v", result)
	}
	if strings.Contains(result.Tab.URL, "secret") || !strings.Contains(result.Tab.URL, "code=REDACTED") {
		t.Fatalf("URL was not redacted: %q", result.Tab.URL)
	}
	reloaded, err := store.Load("default")
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.ActiveTargetID != "page-1" {
		t.Fatalf("active target was not persisted: %#v", reloaded)
	}
}
