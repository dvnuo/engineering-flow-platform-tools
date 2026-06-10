package automation

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

type ScreenshotAssertionOptions struct {
	PageOptions
	Baseline    string
	OutPath     string
	DiffPath    string
	Selector    string
	Ref         string
	Threshold   float64
	FullPage    bool
	FullPageSet bool
}

func (m *Manager) AssertScreenshot(ctx context.Context, opts ScreenshotAssertionOptions) (AssertionResult, error) {
	opts, err := normalizeScreenshotAssertionOptions(opts)
	if err != nil {
		return AssertionResult{}, err
	}
	actual := opts.OutPath
	if actual == "" {
		actual, err = DefaultPageScreenshotPath(m.now())
		if err != nil {
			return AssertionResult{}, err
		}
	}
	shot, err := m.Screenshot(ctx, ScreenshotOptions{
		PageOptions: opts.PageOptions,
		OutPath:     actual,
		FullPage:    opts.FullPage,
		FullPageSet: opts.FullPageSet,
		Selector:    opts.Selector,
		Ref:         opts.Ref,
	})
	if err != nil {
		return AssertionResult{}, err
	}
	ratio, pixels, err := diffPNGs(opts.Baseline, actual, opts.DiffPath)
	if err != nil {
		return AssertionResult{}, err
	}
	pass := ratio <= opts.Threshold
	result := AssertionResult{
		Session:   shot.Session,
		TargetID:  shot.TargetID,
		Assertion: "screenshot",
		Pass:      pass,
		Selector:  shot.Selector,
		Ref:       shot.Ref,
		URL:       shot.URL,
		Title:     shot.Title,
		Observed: AssertionObserved{
			DifferenceRatio: &ratio,
			ComparedPixels:  pixels,
		},
		Artifacts: map[string]string{
			"baseline": filepath.Clean(expandHome(opts.Baseline)),
			"actual":   actual,
			"diff":     opts.DiffPath,
		},
	}
	return result, assertionFailure(result)
}

func normalizeScreenshotAssertionOptions(opts ScreenshotAssertionOptions) (ScreenshotAssertionOptions, error) {
	if strings.TrimSpace(opts.Baseline) == "" {
		return opts, invalidArgs("--baseline is required", "Pass a baseline PNG path.")
	}
	if strings.TrimSpace(opts.DiffPath) == "" {
		return opts, invalidArgs("--diff-out is required", "Pass a diff PNG path.")
	}
	if opts.Threshold < 0 {
		opts.Threshold = 0
	}
	if opts.Threshold > 1 {
		opts.Threshold = 1
	}
	if err := validateOptionalActionTarget(opts.Selector, opts.Ref, "assert.screenshot"); err != nil {
		return opts, err
	}
	if strings.TrimSpace(opts.OutPath) != "" {
		opts.OutPath = filepath.Clean(expandHome(opts.OutPath))
	}
	opts.Baseline = filepath.Clean(expandHome(opts.Baseline))
	opts.DiffPath = filepath.Clean(expandHome(opts.DiffPath))
	if opts.Selector != "" || opts.Ref != "" {
		opts.FullPage = false
	} else if !opts.FullPageSet {
		opts.FullPage = true
	}
	return opts, nil
}

func diffPNGs(baselinePath, actualPath, diffPath string) (float64, int, error) {
	base, err := readPNG(baselinePath)
	if err != nil {
		return 0, 0, NewError("artifact_read_failed", err.Error(), "Check --baseline points to a readable PNG.", 400)
	}
	actual, err := readPNG(actualPath)
	if err != nil {
		return 0, 0, NewError("artifact_read_failed", err.Error(), "The captured screenshot could not be decoded as PNG.", 500)
	}
	bounds := base.Bounds()
	actualBounds := actual.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if actualBounds.Dx() < width {
		width = actualBounds.Dx()
	}
	if actualBounds.Dy() < height {
		height = actualBounds.Dy()
	}
	if width <= 0 || height <= 0 {
		return 1, 0, NewError("artifact_invalid", "PNG dimensions do not overlap.", "Use matching baseline and actual screenshot dimensions.", 400)
	}
	diff := image.NewRGBA(image.Rect(0, 0, width, height))
	changed := 0
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			br, bg, bb, ba := base.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			ar, ag, ab, aa := actual.At(actualBounds.Min.X+x, actualBounds.Min.Y+y).RGBA()
			if br != ar || bg != ag || bb != ab || ba != aa {
				changed++
				diff.Set(x, y, color.RGBA{R: 255, A: 255})
			} else {
				diff.Set(x, y, color.RGBA{R: 0, G: 0, B: 0, A: 0})
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(diffPath), 0o700); err != nil {
		return 0, 0, NewError("artifact_write_failed", err.Error(), "Check --diff-out directory permissions.", 500)
	}
	f, err := os.OpenFile(diffPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return 0, 0, NewError("artifact_write_failed", err.Error(), "Diff PNG could not be created.", 500)
	}
	defer f.Close()
	if err := png.Encode(f, diff); err != nil {
		return 0, 0, NewError("artifact_write_failed", err.Error(), "Diff PNG could not be encoded.", 500)
	}
	pixels := width * height
	ratio := float64(changed) / float64(pixels)
	if bounds.Dx() != actualBounds.Dx() || bounds.Dy() != actualBounds.Dy() {
		ratio = 1
	}
	return ratio, pixels, nil
}

func readPNG(path string) (image.Image, error) {
	f, err := os.Open(filepath.Clean(expandHome(path)))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}
