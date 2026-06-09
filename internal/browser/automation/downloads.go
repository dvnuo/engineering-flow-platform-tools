package automation

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	cdpBrowser "github.com/chromedp/cdproto/browser"
	"github.com/chromedp/chromedp"
)

type DownloadListOptions struct {
	SessionName string
}

type DownloadWaitOptions struct {
	SessionName      string
	FilenameContains string
	TimeoutSeconds   int
}

type DownloadFile struct {
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	Bytes      int64     `json:"bytes"`
	ModifiedAt time.Time `json:"modified_at"`
}

type DownloadListResult struct {
	Session     string         `json:"session"`
	DownloadDir string         `json:"download_dir"`
	Count       int            `json:"count"`
	Files       []DownloadFile `json:"files"`
}

type DownloadWaitResult struct {
	Session          string         `json:"session"`
	DownloadDir      string         `json:"download_dir"`
	FilenameContains string         `json:"filename_contains,omitempty"`
	TimeoutSeconds   int            `json:"timeout_seconds"`
	Count            int            `json:"count"`
	Files            []DownloadFile `json:"files"`
}

func (m *Manager) DownloadList(ctx context.Context, opts DownloadListOptions) (DownloadListResult, error) {
	session, downloadDir, err := m.sessionDownloadDir(opts.SessionName)
	if err != nil {
		return DownloadListResult{}, err
	}
	files, err := listDownloadFiles(downloadDir, "")
	if err != nil {
		return DownloadListResult{}, err
	}
	return DownloadListResult{
		Session:     session.Name,
		DownloadDir: downloadDir,
		Count:       len(files),
		Files:       sanitizeDownloadFiles(files),
	}, nil
}

func (m *Manager) DownloadWait(ctx context.Context, opts DownloadWaitOptions) (DownloadWaitResult, error) {
	session, downloadDir, err := m.sessionDownloadDir(opts.SessionName)
	if err != nil {
		return DownloadWaitResult{}, err
	}
	timeout := time.Duration(opts.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
		opts.TimeoutSeconds = 30
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	files, err := waitForDownloadFiles(waitCtx, downloadDir, opts.FilenameContains)
	if err != nil {
		return DownloadWaitResult{}, err
	}
	return DownloadWaitResult{
		Session:          session.Name,
		DownloadDir:      downloadDir,
		FilenameContains: RedactString(opts.FilenameContains),
		TimeoutSeconds:   opts.TimeoutSeconds,
		Count:            len(files),
		Files:            sanitizeDownloadFiles(files),
	}, nil
}

func (m *Manager) sessionDownloadDir(sessionName string) (Session, string, error) {
	if err := m.ensureStore(); err != nil {
		return Session{}, "", err
	}
	session, err := m.Store.Load(defaultSessionName(sessionName))
	if err != nil {
		return Session{}, "", err
	}
	downloadDir := strings.TrimSpace(session.DownloadDir)
	if downloadDir == "" {
		downloadDir, err = DefaultDownloadDir(session.Name)
		if err != nil {
			return Session{}, "", err
		}
	}
	downloadDir, err = ValidateDownloadDir(downloadDir)
	if err != nil {
		return Session{}, "", err
	}
	return session, downloadDir, nil
}

func listDownloadFiles(downloadDir, filenameContains string) ([]DownloadFile, error) {
	entries, err := os.ReadDir(downloadDir)
	if errors.Is(err, os.ErrNotExist) {
		return []DownloadFile{}, nil
	}
	if err != nil {
		return nil, NewError("automation_failed", err.Error(), "Download directory could not be read.", 500)
	}
	var files []DownloadFile
	for _, entry := range entries {
		if entry.IsDir() || temporaryDownloadFile(entry.Name()) || !downloadNameMatches(entry.Name(), filenameContains) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, NewError("automation_failed", err.Error(), "Downloaded file metadata could not be read.", 500)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		files = append(files, DownloadFile{
			Path:       filepath.Join(downloadDir, entry.Name()),
			Name:       entry.Name(),
			Bytes:      info.Size(),
			ModifiedAt: info.ModTime().UTC(),
		})
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].ModifiedAt.Equal(files[j].ModifiedAt) {
			return files[i].Name < files[j].Name
		}
		return files[i].ModifiedAt.After(files[j].ModifiedAt)
	})
	return files, nil
}

func waitForDownloadFiles(ctx context.Context, downloadDir, filenameContains string) ([]DownloadFile, error) {
	tracker := map[string]downloadObservedFile{}
	for {
		files, err := listDownloadFiles(downloadDir, filenameContains)
		if err != nil {
			return nil, err
		}
		tempActive, err := matchingTemporaryDownloadExists(downloadDir, filenameContains)
		if err != nil {
			return nil, err
		}
		now := time.Now()
		if len(files) > 0 && !tempActive && downloadsSettled(files, tracker, now, 500*time.Millisecond) {
			return files, nil
		}
		select {
		case <-ctx.Done():
			return nil, NewError("timeout", ctx.Err().Error(), "No completed matching download appeared before --timeout.", 408)
		case <-time.After(200 * time.Millisecond):
		}
	}
}

type downloadObservedFile struct {
	Bytes      int64
	ModifiedAt time.Time
	SeenAt     time.Time
}

func downloadsSettled(files []DownloadFile, tracker map[string]downloadObservedFile, now time.Time, stableFor time.Duration) bool {
	allStable := true
	for _, file := range files {
		observed, ok := tracker[file.Path]
		if !ok || observed.Bytes != file.Bytes || !observed.ModifiedAt.Equal(file.ModifiedAt) {
			tracker[file.Path] = downloadObservedFile{Bytes: file.Bytes, ModifiedAt: file.ModifiedAt, SeenAt: now}
			allStable = false
			continue
		}
		if now.Sub(observed.SeenAt) < stableFor {
			allStable = false
		}
	}
	return allStable
}

func matchingTemporaryDownloadExists(downloadDir, filenameContains string) (bool, error) {
	entries, err := os.ReadDir(downloadDir)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, NewError("automation_failed", err.Error(), "Download directory could not be read.", 500)
	}
	for _, entry := range entries {
		if entry.IsDir() || !temporaryDownloadFile(entry.Name()) {
			continue
		}
		name := entry.Name()
		base := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(name, ".crdownload"), ".download"), ".part")
		if downloadNameMatches(name, filenameContains) || downloadNameMatches(base, filenameContains) {
			return true, nil
		}
	}
	return false, nil
}

func temporaryDownloadFile(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	return strings.HasSuffix(lower, ".crdownload") ||
		strings.HasSuffix(lower, ".download") ||
		strings.HasSuffix(lower, ".part") ||
		strings.HasSuffix(lower, ".tmp") ||
		strings.HasPrefix(lower, ".org.chromium.chromium.")
}

func downloadNameMatches(name, filenameContains string) bool {
	filter := strings.ToLower(strings.TrimSpace(filenameContains))
	if filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(name), filter)
}

func sanitizeDownloadFiles(raw []DownloadFile) []DownloadFile {
	out := make([]DownloadFile, len(raw))
	for i, file := range raw {
		file.Path = RedactString(file.Path)
		file.Name = RedactString(file.Name)
		out[i] = file
	}
	return out
}

func ensureDownloadPreferences(profileDir, downloadDir string) error {
	prefsPath := filepath.Join(profileDir, "Default", "Preferences")
	prefs := map[string]any{}
	if b, err := os.ReadFile(prefsPath); err == nil && len(b) > 0 {
		if err := json.Unmarshal(b, &prefs); err != nil {
			return NewError("artifact_write_failed", err.Error(), "Existing browser profile preferences could not be parsed.", 500)
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return NewError("artifact_write_failed", err.Error(), "Browser profile preferences could not be read.", 500)
	}
	setNestedPreference(prefs, []string{"download", "default_directory"}, downloadDir)
	setNestedPreference(prefs, []string{"download", "directory_upgrade"}, true)
	setNestedPreference(prefs, []string{"download", "prompt_for_download"}, false)
	setNestedPreference(prefs, []string{"profile", "default_content_setting_values", "automatic_downloads"}, float64(1))
	if err := os.MkdirAll(filepath.Dir(prefsPath), 0o700); err != nil {
		return NewError("artifact_write_failed", err.Error(), "Browser profile preferences directory could not be created.", 500)
	}
	b, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return NewError("artifact_write_failed", err.Error(), "Browser profile preferences could not be encoded.", 500)
	}
	b = append(b, '\n')
	if err := os.WriteFile(prefsPath, b, 0o600); err != nil {
		return NewError("artifact_write_failed", err.Error(), "Browser profile preferences could not be written.", 500)
	}
	return nil
}

func setNestedPreference(root map[string]any, path []string, value any) {
	current := root
	for _, part := range path[:len(path)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[part] = next
		}
		current = next
	}
	current[path[len(path)-1]] = value
}

func configureBrowserDownloadBehavior(ctx context.Context, browserWebSocketURL, downloadDir string) error {
	if strings.TrimSpace(browserWebSocketURL) == "" || strings.TrimSpace(downloadDir) == "" {
		return nil
	}
	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(ctx, browserWebSocketURL)
	defer cancelAlloc()
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()
	return chromedp.Run(browserCtx, cdpBrowser.SetDownloadBehavior(cdpBrowser.SetDownloadBehaviorBehaviorAllow).WithDownloadPath(downloadDir).WithEventsEnabled(true))
}
