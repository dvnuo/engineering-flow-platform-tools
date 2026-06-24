package browserstack

import "encoding/json"

type Credentials struct {
	Username  string
	AccessKey string
}

type UploadedApp struct {
	AppName                       string          `json:"app_name,omitempty"`
	AppVersion                    string          `json:"app_version,omitempty"`
	AppURL                        string          `json:"app_url,omitempty"`
	AppID                         string          `json:"app_id,omitempty"`
	UploadedAt                    string          `json:"uploaded_at,omitempty"`
	CustomID                      string          `json:"custom_id,omitempty"`
	ShareableID                   string          `json:"shareable_id,omitempty"`
	BypassSecureScreenRestriction string          `json:"bypass_secure_screen_restriction,omitempty"`
	SHA256                        string          `json:"sha256,omitempty"`
	Raw                           json.RawMessage `json:"raw,omitempty"`
}

type UploadAppRequest struct {
	FilePath           string
	URL                string
	CustomID           string
	IOSKeychainSupport bool
	SHA256             string
}

type ListAppsRequest struct {
	Limit    int
	Offset   int
	CustomID string
	Group    bool
}

type Device struct {
	OS          string          `json:"os"`
	OSVersion   string          `json:"os_version"`
	Name        string          `json:"device"`
	RealMobile  bool            `json:"realMobile"`
	DeviceTier  string          `json:"device_tier,omitempty"`
	DeviceLimit int             `json:"device_limit,omitempty"`
	GroupUsage  int             `json:"group_usage,omitempty"`
	TeamUsage   any             `json:"team_usage,omitempty"`
	Raw         json.RawMessage `json:"raw,omitempty"`
}

type Plan struct {
	AutomatePlan                   string `json:"automate_plan,omitempty"`
	TerminalAccess                 string `json:"terminal_access,omitempty"`
	ParallelSessionsRunning        int    `json:"parallel_sessions_running,omitempty"`
	TeamParallelSessionsMaxAllowed int    `json:"team_parallel_sessions_max_allowed,omitempty"`
	ParallelSessionsMaxAllowed     int    `json:"parallel_sessions_max_allowed,omitempty"`
	QueuedSessions                 int    `json:"queued_sessions,omitempty"`
	QueuedSessionsMaxAllowed       int    `json:"queued_sessions_max_allowed,omitempty"`
	Raw                            any    `json:"raw,omitempty"`
}

type Session struct {
	Name                  string         `json:"name,omitempty"`
	Duration              any            `json:"duration,omitempty"`
	OS                    string         `json:"os,omitempty"`
	OSVersion             string         `json:"os_version,omitempty"`
	BrowserVersion        string         `json:"browser_version,omitempty"`
	Browser               any            `json:"browser,omitempty"`
	Device                string         `json:"device,omitempty"`
	Status                string         `json:"status,omitempty"`
	HashedID              string         `json:"hashed_id,omitempty"`
	Reason                string         `json:"reason,omitempty"`
	BuildName             string         `json:"build_name,omitempty"`
	ProjectName           string         `json:"project_name,omitempty"`
	Logs                  string         `json:"logs,omitempty"`
	BrowserURL            string         `json:"browser_url,omitempty"`
	PublicURL             string         `json:"public_url,omitempty"`
	AppiumLogsURL         string         `json:"appium_logs_url,omitempty"`
	VideoURL              string         `json:"video_url,omitempty"`
	DeviceLogsURL         string         `json:"device_logs_url,omitempty"`
	CrashLogsURL          string         `json:"crash_logs_url,omitempty"`
	NetworkLogsURL        string         `json:"network_logs_url,omitempty"`
	BrowserConsoleLogsURL string         `json:"browser_console_logs_url,omitempty"`
	HARLogsURL            string         `json:"har_logs_url,omitempty"`
	AppDetails            map[string]any `json:"app_details,omitempty"`
	Raw                   any            `json:"raw,omitempty"`
}

type Build struct {
	Name     string `json:"name,omitempty"`
	Duration any    `json:"duration,omitempty"`
	Status   string `json:"status,omitempty"`
	HashedID string `json:"hashed_id,omitempty"`
	Raw      any    `json:"raw,omitempty"`
}

type Project struct {
	Name     string `json:"name,omitempty"`
	Duration any    `json:"duration,omitempty"`
	Status   string `json:"status,omitempty"`
	ID       any    `json:"id,omitempty"`
	HashedID string `json:"hashed_id,omitempty"`
	Raw      any    `json:"raw,omitempty"`
}

type UpdateSessionRequest struct {
	Name   string `json:"name,omitempty"`
	Status string `json:"status,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type Artifact struct {
	Kind        string `json:"kind"`
	URL         string `json:"url,omitempty"`
	Path        string `json:"path,omitempty"`
	Size        int64  `json:"size,omitempty"`
	SHA256      string `json:"sha256,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Missing     bool   `json:"missing,omitempty"`
	Warning     string `json:"warning,omitempty"`
}
