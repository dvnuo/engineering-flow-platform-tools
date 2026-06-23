package mobile

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const staleLockAge = 30 * time.Minute
const AppCacheReuseWindow = 25 * 24 * time.Hour

type RunStatus string

const (
	StatusStarting        RunStatus = "starting"
	StatusRunning         RunStatus = "running"
	StatusWaitingForHuman RunStatus = "waiting_for_human"
	StatusResuming        RunStatus = "resuming"
	StatusFinished        RunStatus = "finished"
	StatusFailed          RunStatus = "failed"
	StatusLost            RunStatus = "lost"
)

type RunState struct {
	Version             int               `json:"version"`
	RunID               string            `json:"run_id"`
	Provider            string            `json:"provider"`
	Status              RunStatus         `json:"status"`
	ControlOwner        string            `json:"control_owner"`
	SessionID           string            `json:"session_id"`
	Platform            string            `json:"platform"`
	Device              DeviceSelection   `json:"device"`
	App                 AppRef            `json:"app"`
	Network             NetworkState      `json:"network"`
	LatestObservationID string            `json:"latest_observation_id,omitempty"`
	ObservationVersion  int               `json:"observation_version"`
	BuildID             string            `json:"build_id,omitempty"`
	ProjectName         string            `json:"project_name,omitempty"`
	BuildName           string            `json:"build_name,omitempty"`
	SessionName         string            `json:"session_name,omitempty"`
	DashboardURL        string            `json:"dashboard_url,omitempty"`
	StartedAt           time.Time         `json:"started_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
	FinishedAt          *time.Time        `json:"finished_at,omitempty"`
	Metadata            map[string]string `json:"metadata,omitempty"`
}

type DeviceSelection struct {
	Name      string `json:"name"`
	OS        string `json:"os"`
	OSVersion string `json:"os_version"`
	Tier      string `json:"tier,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

type AppRef struct {
	AppURL     string    `json:"app_url,omitempty"`
	CustomID   string    `json:"custom_id,omitempty"`
	SHA256     string    `json:"sha256,omitempty"`
	Name       string    `json:"name,omitempty"`
	UploadedAt time.Time `json:"uploaded_at,omitempty"`
	ExpiresAt  time.Time `json:"expires_at,omitempty"`
}

type NetworkState struct {
	Mode            string `json:"mode"`
	LocalIdentifier string `json:"local_identifier,omitempty"`
	TunnelID        string `json:"tunnel_id,omitempty"`
}

type StateStore struct {
	RootDir      string
	ArtifactsDir string
}

func NewStateStore(rootDir, artifactsDir string) *StateStore {
	return &StateStore{RootDir: rootDir, ArtifactsDir: artifactsDir}
}

func (s *StateStore) Ensure() error {
	if err := os.MkdirAll(filepath.Join(s.RootDir, "runs"), 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(s.RootDir, "apps"), 0o700); err != nil {
		return err
	}
	return os.MkdirAll(s.ArtifactsDir, 0o700)
}

func (s *StateStore) RunDir(runID string) string {
	return filepath.Join(s.RootDir, "runs", cleanName(runID))
}

func (s *StateStore) StatePath(runID string) string {
	return filepath.Join(s.RunDir(runID), "state.json")
}

func (s *StateStore) SaveRun(st RunState) error {
	if st.Version == 0 {
		st.Version = 1
	}
	st.UpdatedAt = time.Now().UTC()
	dir := s.RunDir(st.RunID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return atomicWriteJSON(filepath.Join(dir, "state.json"), st, 0o600)
}

func (s *StateStore) LoadRun(runID string) (RunState, error) {
	var st RunState
	b, err := os.ReadFile(s.StatePath(runID))
	if err != nil {
		if os.IsNotExist(err) {
			return st, NewError("not_found", "run state was not found", "Check --run-id or run mobile run status.", 404)
		}
		return st, err
	}
	if err := json.Unmarshal(b, &st); err != nil {
		return st, NewError("state_error", "run state is corrupt", "Inspect the state file or finish the run manually.", 500)
	}
	return st, nil
}

func (s *StateStore) ListRuns() ([]RunState, error) {
	dir := filepath.Join(s.RootDir, "runs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []RunState
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		st, err := s.LoadRun(entry.Name())
		if err == nil {
			out = append(out, st)
		}
	}
	return out, nil
}

func (s *StateStore) WithRunLock(runID string, fn func() error) error {
	dir := s.RunDir(runID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	lock := filepath.Join(dir, "lock")
	f, err := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			if info, statErr := os.Stat(lock); statErr == nil && time.Since(info.ModTime()) > staleLockAge {
				_ = os.Remove(lock)
				f, err = os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
			}
			if err != nil {
				return NewError("run_locked", "run is locked by another process", "Wait for the other mobile command to finish. If the process crashed and the lock is stale, inspect and remove "+lock+".", 409)
			}
		} else {
			return err
		}
	}
	_, _ = f.WriteString(time.Now().UTC().Format(time.RFC3339Nano))
	_ = f.Close()
	defer os.Remove(lock)
	return fn()
}

func (s *StateStore) ObservationDir(runID string) string {
	return filepath.Join(s.RunDir(runID), "observations")
}

func (s *StateStore) SaveObservation(runID string, obs Observation) error {
	dir := filepath.Join(s.ObservationDir(runID), obs.ID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if obs.Source != "" {
		if err := os.WriteFile(filepath.Join(dir, "source.xml"), []byte(obs.Source), 0o600); err != nil {
			return err
		}
	}
	if len(obs.Screenshot) > 0 {
		if err := os.WriteFile(filepath.Join(dir, "screenshot.png"), obs.Screenshot, 0o600); err != nil {
			return err
		}
	}
	obs.Source = ""
	obs.Screenshot = nil
	return atomicWriteJSON(filepath.Join(dir, "candidates.json"), obs, 0o600)
}

func (s *StateStore) LoadObservation(runID, obsID string) (Observation, error) {
	var obs Observation
	b, err := os.ReadFile(filepath.Join(s.ObservationDir(runID), cleanName(obsID), "candidates.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return obs, NewError("stale_observation", "observation was not found", "Run mobile observe --json again.", 409)
		}
		return obs, err
	}
	if err := json.Unmarshal(b, &obs); err != nil {
		return obs, err
	}
	return obs, nil
}

func (s *StateStore) AppCachePath(sha string) string {
	return filepath.Join(s.RootDir, "apps", cleanName(sha)+".json")
}

func (s *StateStore) SaveAppCache(app AppRef) error {
	if strings.TrimSpace(app.SHA256) == "" {
		return nil
	}
	app = NormalizeAppCacheRef(app, time.Now().UTC())
	if err := os.MkdirAll(filepath.Join(s.RootDir, "apps"), 0o700); err != nil {
		return err
	}
	return atomicWriteJSON(s.AppCachePath(app.SHA256), app, 0o600)
}

func (s *StateStore) LoadAppCache(sha string) (AppRef, error) {
	var app AppRef
	b, err := os.ReadFile(s.AppCachePath(sha))
	if err != nil {
		return app, err
	}
	err = json.Unmarshal(b, &app)
	return app, err
}

func NormalizeAppCacheRef(app AppRef, now time.Time) AppRef {
	if app.UploadedAt.IsZero() {
		app.UploadedAt = now
	}
	if app.ExpiresAt.IsZero() {
		app.ExpiresAt = app.UploadedAt.Add(AppCacheReuseWindow)
	}
	return app
}

func AppCacheReusable(app AppRef, now time.Time) bool {
	return strings.TrimSpace(app.AppURL) != "" && !app.ExpiresAt.IsZero() && now.Before(app.ExpiresAt)
}

func atomicWriteJSON(path string, value any, perm os.FileMode) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, perm); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		_ = os.Remove(path)
	}
	return os.Rename(tmp, path)
}

func NewRunID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return "run-" + time.Now().UTC().Format("20060102T150405") + "-" + hex.EncodeToString(b[:])
}

func NewObservationID(version int) string {
	if version <= 0 {
		version = 1
	}
	return "obs-" + strings.TrimLeft(time.Now().UTC().Format("150405.000"), "0") + "-" + shortRandom()
}

func shortRandom() string {
	var b [3]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func cleanName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, "..", "_")
	if s == "" {
		return "_"
	}
	return s
}
