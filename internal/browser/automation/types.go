package automation

import "time"

const (
	LocalDebugAddr     = "127.0.0.1"
	DefaultSessionName = "default"
)

type Session struct {
	Name                string    `json:"name"`
	BrowserPath         string    `json:"browser_path,omitempty"`
	ProfileDir          string    `json:"profile_dir,omitempty"`
	DownloadDir         string    `json:"download_dir,omitempty"`
	DebugAddr           string    `json:"debug_addr"`
	DebugPort           int       `json:"debug_port"`
	BrowserWebSocketURL string    `json:"browser_websocket_url,omitempty"`
	PID                 int       `json:"pid,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	LastSeenAt          time.Time `json:"last_seen_at,omitempty"`
	Alive               bool      `json:"alive"`
	ActiveTargetID      string    `json:"active_target_id,omitempty"`
	MetadataPath        string    `json:"metadata_path,omitempty"`
}

type StartOptions struct {
	Name         string
	Browser      string
	BrowserExe   string
	Headless     bool
	ProfileDir   string
	DownloadDir  string
	CleanProfile bool
	Port         int
	URL          string
	Verbose      bool
}

type StopOptions struct {
	Name         string
	KeepMetadata bool
}

type AttachOptions struct {
	Name      string
	DebugAddr string
	DebugPort int
}

type DiscoverOptions struct {
	DebugAddr string
	Ports     []int
}

type DiscoveredSession struct {
	DebugAddr           string    `json:"debug_addr"`
	DebugPort           int       `json:"debug_port"`
	Alive               bool      `json:"alive"`
	Browser             string    `json:"browser,omitempty"`
	ProtocolVersion     string    `json:"protocol_version,omitempty"`
	BrowserWebSocketURL string    `json:"browser_websocket_url,omitempty"`
	Targets             []Target  `json:"targets,omitempty"`
	CheckedAt           time.Time `json:"checked_at"`
}

type VersionInfo struct {
	Browser              string `json:"browser,omitempty"`
	ProtocolVersion      string `json:"protocol_version,omitempty"`
	UserAgent            string `json:"user_agent,omitempty"`
	V8Version            string `json:"v8_version,omitempty"`
	WebKitVersion        string `json:"webkit_version,omitempty"`
	WebSocketDebuggerURL string `json:"browser_websocket_url,omitempty"`
}

type devToolsVersionJSON struct {
	Browser              string `json:"Browser"`
	ProtocolVersion      string `json:"Protocol-Version"`
	UserAgent            string `json:"User-Agent"`
	V8Version            string `json:"V8-Version"`
	WebKitVersion        string `json:"WebKit-Version"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

type Target struct {
	ID                   string `json:"id"`
	Type                 string `json:"type"`
	Title                string `json:"title,omitempty"`
	URL                  string `json:"url,omitempty"`
	WebSocketDebuggerURL string `json:"web_socket_debugger_url,omitempty"`
	Active               bool   `json:"active,omitempty"`
}

type devToolsTargetJSON struct {
	ID                   string `json:"id"`
	Type                 string `json:"type"`
	Title                string `json:"title"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}
