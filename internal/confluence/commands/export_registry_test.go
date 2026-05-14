package commands

import (
	"net/http"
	"testing"
)

func TestExportMarkdownAndRegistry(t *testing.T) {
	p := cfg(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"body":{"view":{"value":"<p>Hello</p>"}}}`))
	})
	r := run(t, p, "page", "export-markdown", "--id", "1")
	if ok, _ := r["ok"].(bool); !ok {
		t.Fatal("export markdown failed")
	}
	data := r["data"].(map[string]any)
	if data["markdown"] == "" {
		t.Fatal("markdown missing")
	}
	cmds := run(t, p, "commands")["data"].(map[string]any)["commands"].([]any)
	if len(cmds) < 50 {
		t.Fatal("commands list too short")
	}
	s := run(t, p, "schema", "page.create")
	req := s["data"].(map[string]any)["required"].([]any)
	if len(req) == 0 {
		t.Fatal("schema required missing")
	}
}
