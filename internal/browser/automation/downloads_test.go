package automation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultDownloadDirUsesBrowserHome(t *testing.T) {
	t.Setenv(envBrowserHome, t.TempDir())
	got, err := DefaultDownloadDir("default")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, filepath.Join("downloads", "default")) {
		t.Fatalf("unexpected download dir: %s", got)
	}
}

func TestValidateDownloadDirRejectsRoot(t *testing.T) {
	if _, err := ValidateDownloadDir("/"); err == nil {
		t.Fatal("filesystem root should be rejected")
	}
	dir := filepath.Join(t.TempDir(), "downloads")
	got, err := ValidateDownloadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != dir {
		t.Fatalf("download dir = %q want %q", got, dir)
	}
}

func TestListDownloadFilesSkipsTemporaryFilesFiltersAndSorts(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old-report.txt")
	newPath := filepath.Join(dir, "new-report.txt")
	tempPath := filepath.Join(dir, "new-report.txt.crdownload")
	if err := os.WriteFile(oldPath, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("newer"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tempPath, []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Date(2026, 6, 8, 1, 0, 0, 0, time.UTC)
	newTime := oldTime.Add(time.Minute)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newPath, newTime, newTime); err != nil {
		t.Fatal(err)
	}

	files, err := listDownloadFiles(dir, "report")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("files = %#v", files)
	}
	if files[0].Name != "new-report.txt" || files[1].Name != "old-report.txt" {
		t.Fatalf("files not sorted newest first: %#v", files)
	}
}

func TestDownloadHelpersDetectTemporaryAndSettledFiles(t *testing.T) {
	if !temporaryDownloadFile("report.pdf.crdownload") || !temporaryDownloadFile("report.pdf.part") {
		t.Fatalf("temporary download suffix not detected")
	}
	now := time.Date(2026, 6, 8, 1, 2, 3, 0, time.UTC)
	files := []DownloadFile{{Path: "/tmp/report.pdf", Bytes: 10, ModifiedAt: now}}
	tracker := map[string]downloadObservedFile{}
	if downloadsSettled(files, tracker, now, 500*time.Millisecond) {
		t.Fatalf("first observation should not be settled")
	}
	if downloadsSettled(files, tracker, now.Add(400*time.Millisecond), 500*time.Millisecond) {
		t.Fatalf("file should not settle before stable window")
	}
	if !downloadsSettled(files, tracker, now.Add(500*time.Millisecond), 500*time.Millisecond) {
		t.Fatalf("file should settle after stable window")
	}
	files[0].Bytes = 11
	if downloadsSettled(files, tracker, now.Add(600*time.Millisecond), 500*time.Millisecond) {
		t.Fatalf("size change should reset stable window")
	}
}

func TestSanitizeDownloadFilesRedactsNamesAndPaths(t *testing.T) {
	got := sanitizeDownloadFiles([]DownloadFile{{
		Path: "/tmp/access_token=secret/report.txt",
		Name: "access_token=secret-report.txt",
	}})
	if strings.Contains(got[0].Path+got[0].Name, "access_token=secret") {
		t.Fatalf("download metadata leaked token: %#v", got[0])
	}
}
