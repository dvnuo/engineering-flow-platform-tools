package mobile

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type FileArtifact struct {
	Kind        string `json:"kind"`
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
	ContentType string `json:"content_type,omitempty"`
	Sensitive   bool   `json:"sensitive"`
}

var unsafePathChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func SafeArtifactName(parts ...string) string {
	joined := strings.Join(parts, "-")
	joined = strings.ReplaceAll(joined, "..", "_")
	joined = unsafePathChars.ReplaceAllString(joined, "_")
	joined = strings.Trim(joined, "._-")
	if joined == "" {
		return "artifact"
	}
	return joined
}

func WriteArtifact(dir, kind, name string, data []byte, contentType string) (FileArtifact, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return FileArtifact{}, err
	}
	path := filepath.Join(dir, SafeArtifactName(kind, name))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return FileArtifact{}, err
	}
	return InspectArtifact(kind, path, contentType)
}

func InspectArtifact(kind, path, contentType string) (FileArtifact, error) {
	f, err := os.Open(path)
	if err != nil {
		return FileArtifact{}, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return FileArtifact{}, err
	}
	return FileArtifact{Kind: kind, Path: path, Size: n, SHA256: hex.EncodeToString(h.Sum(nil)), ContentType: contentType, Sensitive: true}, nil
}
