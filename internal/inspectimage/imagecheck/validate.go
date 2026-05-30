package imagecheck

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"strings"

	"engineering-flow-platform-tools/internal/inspectimage/config"
)

type ImageInfo struct {
	Path      string `json:"path"`
	MIMEType  string `json:"mime_type"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Animated  bool   `json:"animated"`
	Data      []byte `json:"-"`
}

type ValidationError struct {
	Code    string
	Message string
	Hint    string
	Status  int
}

func (e *ValidationError) Error() string { return e.Code + ": " + e.Message }

func Validate(path string, maxBytes int64, allowed []string) (ImageInfo, []string, error) {
	if path == "" {
		return ImageInfo{}, nil, &ValidationError{Code: "invalid_args", Message: "--image is required.", Hint: "Run inspect-image schema inspect --json.", Status: 400}
	}
	lower := strings.ToLower(strings.TrimSpace(path))
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return ImageInfo{}, nil, &ValidationError{Code: "invalid_args", Message: "Remote image URLs are not supported.", Hint: "Download the image to a local JPEG, PNG, WEBP, or GIF file, then pass --image <path>.", Status: 400}
	}
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ImageInfo{}, nil, &ValidationError{Code: "image_not_found", Message: "Image path does not exist.", Hint: "Pass a valid local image path.", Status: 404}
		}
		return ImageInfo{}, nil, &ValidationError{Code: "invalid_args", Message: "Image path could not be inspected. " + config.RedactString(err.Error()), Hint: "Pass a readable local image file.", Status: 400}
	}
	if !info.Mode().IsRegular() {
		return ImageInfo{}, nil, &ValidationError{Code: "not_a_file", Message: "Image path is not a regular file.", Hint: "Pass a local JPEG, PNG, WEBP, or GIF file.", Status: 400}
	}
	if info.Size() > maxBytes {
		return ImageInfo{}, nil, &ValidationError{Code: "image_too_large", Message: "inspect-image supports images up to 3145728 bytes.", Hint: "Resize or compress the image, then retry.", Status: 400}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ImageInfo{}, nil, &ValidationError{Code: "invalid_args", Message: "Image file could not be read. " + config.RedactString(err.Error()), Hint: "Check file permissions and retry.", Status: 400}
	}
	mime := DetectMIME(data)
	if mime == "" {
		return ImageInfo{}, nil, &ValidationError{Code: "not_an_image", Message: "File is not a supported image by magic bytes.", Hint: "Use a JPEG, PNG, WEBP, or GIF image.", Status: 400}
	}
	if !AllowedMIME(mime, allowed) {
		return ImageInfo{}, nil, &ValidationError{Code: "unsupported_image_type", Message: "Image type is not supported.", Hint: "Use JPEG, PNG, WEBP, or GIF.", Status: 400}
	}
	sum := sha256.Sum256(data)
	meta, warn := ReadMetadata(data, mime)
	warnings := []string{}
	if warn != "" {
		warnings = append(warnings, warn)
	}
	if meta.Animated {
		warnings = append(warnings, "The image is an animated GIF. The model may inspect only a representative frame.")
	}
	return ImageInfo{
		Path:      path,
		MIMEType:  mime,
		SizeBytes: info.Size(),
		SHA256:    hex.EncodeToString(sum[:]),
		Width:     meta.Width,
		Height:    meta.Height,
		Animated:  meta.Animated,
		Data:      data,
	}, warnings, nil
}
