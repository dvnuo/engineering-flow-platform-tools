package imagecheck

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

var tinyPNG = []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0x0d, 'I', 'H', 'D', 'R', 0, 0, 0, 1, 0, 0, 0, 1, 8, 2, 0, 0, 0, 0x90, 0x77, 0x53, 0xde, 0, 0, 0, 0x0c, 'I', 'D', 'A', 'T', 8, 0xd7, 0x63, 0xf8, 0xff, 0xff, 0x3f, 0, 5, 0, 1, 0xfe, 0xa7, 0x69, 0x81, 0x84, 0, 0, 0, 0, 'I', 'E', 'N', 'D', 0xae, 0x42, 0x60, 0x82}

func TestDetectMIMEByMagicBytes(t *testing.T) {
	tests := map[string][]byte{
		"image/jpeg": {0xff, 0xd8, 0xff, 0x00},
		"image/png":  tinyPNG[:8],
		"image/webp": []byte("RIFFxxxxWEBP"),
		"image/gif":  []byte("GIF89a"),
	}
	for want, data := range tests {
		if got := DetectMIME(data); got != want {
			t.Fatalf("DetectMIME=%q want %q", got, want)
		}
	}
}

func TestRejectsUnsupportedFiles(t *testing.T) {
	for _, data := range [][]byte{[]byte("hello"), []byte("%PDF"), []byte("<svg></svg>"), []byte{0, 0, 0, 24, 'f', 't', 'y', 'p'}} {
		path := write(t, data)
		_, _, err := Validate(path, 1000, []string{"image/png"})
		if err == nil {
			t.Fatalf("expected rejection for %q", data)
		}
	}
}

func TestRejectsTooLarge(t *testing.T) {
	path := write(t, append(tinyPNG, bytes.Repeat([]byte{0}, 20)...))
	_, _, err := Validate(path, 8, []string{"image/png"})
	if err == nil {
		t.Fatal("expected image_too_large")
	}
}

func TestRejectsNonRegularFile(t *testing.T) {
	_, _, err := Validate(t.TempDir(), 1000, []string{"image/png"})
	if err == nil {
		t.Fatal("expected not_a_file")
	}
}

func TestValidateComputesSHAAndDimensions(t *testing.T) {
	path := write(t, tinyPNG)
	info, _, err := Validate(path, 1000, []string{"image/png"})
	if err != nil {
		t.Fatal(err)
	}
	if info.SHA256 == "" || info.Width != 1 || info.Height != 1 || info.Data == nil {
		t.Fatalf("bad info: %#v", info)
	}
}

func TestDetectAnimatedGIF(t *testing.T) {
	data := append([]byte("GIF89a"), []byte{0, 0, 0x2c, 1, 2, 3, 0x2c}...)
	if !DetectAnimatedGIF(data) {
		t.Fatal("expected animated gif")
	}
}

func write(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "image.bin")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
