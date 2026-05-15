package commands

import (
	"net/http"
	"testing"
)

func TestPageUpdateFetchesVersionWithExpand(t *testing.T) {
	p := cfg(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/rest/api/content/123" {
			if r.URL.Query().Get("expand") != "version" {
				t.Fatalf("missing expand=version: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"version":{"number":2}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	if !run(t, p, "page", "update", "--id", "123", "--title", "Next")["ok"].(bool) {
		t.Fatal("page update failed")
	}
}

func TestPageExportMissingBodyDoesNotPanic(t *testing.T) {
	p := cfg(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"123"}`))
	})
	r := run(t, p, "page", "export-markdown", "--id", "123")
	if ok, _ := r["ok"].(bool); ok {
		t.Fatal("missing body should fail")
	}
	errObj, _ := r["error"].(map[string]any)
	if errObj["code"] != "not_found" {
		t.Fatalf("code=%v", errObj["code"])
	}
}
