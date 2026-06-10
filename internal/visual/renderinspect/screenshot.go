package renderinspect

import (
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"strings"

	"engineering-flow-platform-tools/internal/visual/metadata"
	"engineering-flow-platform-tools/internal/visual/preview"
)

type ScreenshotInspection struct {
	Provided        bool    `json:"provided"`
	Path            string  `json:"path,omitempty"`
	Width           int     `json:"width,omitempty"`
	Height          int     `json:"height,omitempty"`
	NonBlank        bool    `json:"non_blank"`
	ContrastOK      bool    `json:"contrast_ok"`
	CoverageOK      bool    `json:"coverage_ok"`
	LuminanceMean   float64 `json:"luminance_mean,omitempty"`
	LuminanceStdDev float64 `json:"luminance_stddev,omitempty"`
	ContentCoverage float64 `json:"content_coverage,omitempty"`
}

func inspectScreenshot(path string) (ScreenshotInspection, []preview.Warning, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return ScreenshotInspection{}, nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return ScreenshotInspection{}, nil, metadata.NewError("screenshot_read_failed", "visual render screenshot could not be read: "+err.Error(), "Pass a readable PNG, JPEG, or GIF screenshot path.", 400)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return ScreenshotInspection{}, nil, metadata.NewError("screenshot_decode_failed", "visual render screenshot could not be decoded: "+err.Error(), "Pass a PNG, JPEG, or GIF screenshot.", 400)
	}
	result := analyzeImage(path, img)
	warnings := screenshotWarnings(result)
	return result, warnings, nil
}

func analyzeImage(path string, img image.Image) ScreenshotInspection {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	stepX := maxInt(1, w/220)
	stepY := maxInt(1, h/160)
	var count int
	var sum float64
	var sumSq float64
	bins := make([]int, 32)
	for y := bounds.Min.Y; y < bounds.Max.Y; y += stepY {
		for x := bounds.Min.X; x < bounds.Max.X; x += stepX {
			lum := luminance(img.At(x, y).RGBA())
			count++
			sum += lum
			sumSq += lum * lum
			idx := int(lum * float64(len(bins)))
			if idx >= len(bins) {
				idx = len(bins) - 1
			}
			bins[idx]++
		}
	}
	mean := 0.0
	stddev := 0.0
	coverage := 0.0
	if count > 0 {
		mean = sum / float64(count)
		variance := sumSq/float64(count) - mean*mean
		if variance > 0 {
			stddev = math.Sqrt(variance)
		}
		maxBin := 0
		for _, bin := range bins {
			if bin > maxBin {
				maxBin = bin
			}
		}
		coverage = 1 - float64(maxBin)/float64(count)
	}
	return ScreenshotInspection{
		Provided:        true,
		Path:            path,
		Width:           w,
		Height:          h,
		NonBlank:        stddev >= 0.015 || coverage >= 0.04,
		ContrastOK:      stddev >= 0.035,
		CoverageOK:      coverage >= 0.08,
		LuminanceMean:   roundMetric(mean),
		LuminanceStdDev: roundMetric(stddev),
		ContentCoverage: roundMetric(coverage),
	}
}

func screenshotWarnings(result ScreenshotInspection) []preview.Warning {
	if !result.Provided {
		return nil
	}
	var warnings []preview.Warning
	add := func(code, severity, message, suggestion string) {
		warnings = append(warnings, preview.Warning{Code: code, Severity: severity, Message: message, Suggestion: suggestion, AutoFixHint: map[string]any{"action": code}})
	}
	if !result.NonBlank {
		add("screenshot_blank", "error", "The rendered screenshot appears blank or nearly uniform.", "Open the artifact in a browser and verify that runtime scripts, data.js, and WebGL are loading.")
	}
	if result.NonBlank && !result.ContrastOK {
		add("screenshot_low_contrast", "warning", "The rendered screenshot has low visual contrast.", "Increase contrast between objects, lines, labels, and the background, or reduce translucent overlays.")
	}
	if result.NonBlank && !result.CoverageOK {
		add("screenshot_low_content_coverage", "warning", "The rendered screenshot uses very little visible canvas area.", "Check camera framing, first-view focus, and object scale so the primary scene occupies more of the viewport.")
	}
	return warnings
}

func luminance(r, g, b, a uint32) float64 {
	alpha := float64(a) / 65535
	if alpha <= 0 {
		return 0
	}
	rf := float64(r) / 65535
	gf := float64(g) / 65535
	bf := float64(b) / 65535
	return (0.2126*rf + 0.7152*gf + 0.0722*bf) * alpha
}

func roundMetric(v float64) float64 {
	return math.Round(v*10000) / 10000
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
