package automation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateUploadFilesRequiresFileUnlessClearing(t *testing.T) {
	if _, _, err := validateUploadFiles(nil, false); err == nil {
		t.Fatal("missing --file should fail when --clear is not set")
	}
	files, paths, err := validateUploadFiles(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 || len(paths) != 0 {
		t.Fatalf("clear-only upload should not return files: %#v %#v", files, paths)
	}
}

func TestValidateUploadFilesRequiresRegularFiles(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := validateUploadFiles([]string{dir}, false); err == nil {
		t.Fatal("directory should not validate as upload file")
	}
	file := filepath.Join(dir, "report.txt")
	if err := os.WriteFile(file, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	files, paths, err := validateUploadFiles([]string{file}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || len(paths) != 1 || files[0].Name != "report.txt" || files[0].Bytes != 5 {
		t.Fatalf("upload metadata = %#v paths=%#v", files, paths)
	}
	if !filepath.IsAbs(paths[0]) {
		t.Fatalf("upload path should be absolute: %s", paths[0])
	}
}

func TestSanitizeUploadFileMetadataRedactsPathAndName(t *testing.T) {
	got := sanitizeUploadFileMetadata([]UploadFileMetadata{{
		Path:  "/tmp/token=secret/report.txt",
		Name:  "token=secret-report.txt",
		Bytes: 10,
	}})
	if strings.Contains(got[0].Path+got[0].Name, "token=secret") {
		t.Fatalf("upload metadata leaked token: %#v", got[0])
	}
}
