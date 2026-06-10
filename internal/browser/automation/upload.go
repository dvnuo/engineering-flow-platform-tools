package automation

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/chromedp/chromedp"
)

type UploadOptions struct {
	PageOptions
	Selector string
	Files    []string
	Clear    bool
}

type UploadFileMetadata struct {
	Path  string `json:"path"`
	Name  string `json:"name"`
	Bytes int64  `json:"bytes"`
}

type UploadResult struct {
	Session   string               `json:"session"`
	TargetID  string               `json:"target_id"`
	URL       string               `json:"url"`
	Title     string               `json:"title"`
	Selector  string               `json:"selector"`
	Clear     bool                 `json:"clear,omitempty"`
	FileCount int                  `json:"file_count"`
	Files     []UploadFileMetadata `json:"files"`
}

func (m *Manager) Upload(ctx context.Context, opts UploadOptions) (UploadResult, error) {
	if strings.TrimSpace(opts.Selector) == "" {
		return UploadResult{}, invalidArgs("--selector is required", "Pass a CSS selector for an input[type=file] element.")
	}
	files, uploadPaths, err := validateUploadFiles(opts.Files, opts.Clear)
	if err != nil {
		return UploadResult{}, err
	}
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return UploadResult{}, err
	}
	defer cancel()

	actions := []chromedp.Action{}
	if opts.Clear {
		actions = append(actions, chromedp.SetUploadFiles(opts.Selector, []string{}, chromedp.ByQuery))
	}
	if len(uploadPaths) > 0 {
		actions = append(actions, chromedp.SetUploadFiles(opts.Selector, uploadPaths, chromedp.ByQuery))
	}
	var finalURL, title string
	actions = append(actions, chromedp.Location(&finalURL), chromedp.Title(&title))
	if err := chromedp.Run(pageCtx, actions...); err != nil {
		return UploadResult{}, mapPageError(err, "automation_failed")
	}
	return UploadResult{
		Session:   session.Name,
		TargetID:  target.ID,
		URL:       RedactURL(finalURL),
		Title:     RedactString(title),
		Selector:  RedactString(opts.Selector),
		Clear:     opts.Clear,
		FileCount: len(files),
		Files:     sanitizeUploadFileMetadata(files),
	}, nil
}

func validateUploadFiles(paths []string, clear bool) ([]UploadFileMetadata, []string, error) {
	if len(paths) == 0 && !clear {
		return nil, nil, invalidArgs("--file is required", "Pass --file <path> for each file to attach, or --clear to clear the input.")
	}
	files := make([]UploadFileMetadata, 0, len(paths))
	uploadPaths := make([]string, 0, len(paths))
	for _, raw := range paths {
		clean := filepath.Clean(expandHome(strings.TrimSpace(raw)))
		if clean == "" || clean == "." {
			return nil, nil, invalidArgs("--file must point at a regular file", "Pass an existing local file path.")
		}
		abs, err := filepath.Abs(clean)
		if err != nil {
			return nil, nil, NewError("invalid_args", err.Error(), "Pass an existing local file path.", 400)
		}
		info, err := os.Stat(abs)
		if err != nil {
			return nil, nil, NewError("invalid_args", err.Error(), "Pass an existing local file path.", 400)
		}
		if !info.Mode().IsRegular() {
			return nil, nil, invalidArgs("--file must point at a regular file", "Directories, devices, and special files cannot be uploaded.")
		}
		files = append(files, UploadFileMetadata{
			Path:  abs,
			Name:  filepath.Base(abs),
			Bytes: info.Size(),
		})
		uploadPaths = append(uploadPaths, abs)
	}
	return files, uploadPaths, nil
}

func sanitizeUploadFileMetadata(raw []UploadFileMetadata) []UploadFileMetadata {
	out := make([]UploadFileMetadata, len(raw))
	for i, file := range raw {
		file.Path = RedactString(file.Path)
		file.Name = RedactString(file.Name)
		out[i] = file
	}
	return out
}
