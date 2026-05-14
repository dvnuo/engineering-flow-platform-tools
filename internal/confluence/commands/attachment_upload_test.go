package commands

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAttachmentUploadMultipartHeader(t *testing.T) {
	gotHeader := ""
	gotCT := ""
	p := cfg(t, func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Atlassian-Token")
		gotCT = r.Header.Get("Content-Type")
		_, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	tmp := filepath.Join(t.TempDir(), "a.txt")
	_ = os.WriteFile(tmp, []byte("hello"), 0644)
	r := run(t, p, "attachment", "upload", "9", tmp)
	if ok, _ := r["ok"].(bool); !ok {
		t.Fatal("upload failed")
	}
	if gotHeader != "no-check" {
		t.Fatal("missing x-atlassian-token")
	}
	if !strings.HasPrefix(gotCT, "multipart/form-data") {
		t.Fatal("not multipart")
	}
}
