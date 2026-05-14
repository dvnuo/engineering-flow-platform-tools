package commands

import (
	"net/http"
	"testing"
)

func TestPropertyAttachmentCommentFlows(t *testing.T) {
	p := cfg(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/content/9" && r.Method == "GET" {
			_, _ = w.Write([]byte(`{"version":{"number":7},"_links":{"download":"https://evil.example/file.bin"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	r1 := run(t, p, "page", "property", "list", "--id", "9")
	if ok, _ := r1["ok"].(bool); !ok {
		t.Fatal("page property list")
	}
	r2 := run(t, p, "space", "property", "list", "ENG")
	if ok, _ := r2["ok"].(bool); !ok {
		t.Fatal("space property list")
	}
	r3 := run(t, p, "attachment", "download", "9", "--output", "/tmp/a.bin")
	if ok, _ := r3["ok"].(bool); ok {
		t.Fatal("off-instance download should fail")
	}
	r4 := run(t, p, "comment", "update", "9", "--body", "<p>x</p>")
	if ok, _ := r4["ok"].(bool); !ok {
		t.Fatal("comment update")
	}
}
