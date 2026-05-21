package probe

type ProbeOptions struct {
	URL              string
	Selector         string
	RequireSelector  bool
	WaitSeconds      int
	TimeoutSeconds   int
	OutDir           string
	ProfileDir       string
	CleanProfile     bool
	BrowserExe       string
	Browser          string
	Headless         bool
	IgnoreCertErrors bool
	FetchAPI         string
	NetworkFilter    string
	MaxNetworkEvents int
	SaveHTML         bool
	SaveScreenshot   bool
	Verbose          bool
}

type NetworkEvent struct {
	Kind         string `json:"kind"`
	Time         string `json:"time"`
	RequestID    string `json:"request_id,omitempty"`
	Method       string `json:"method,omitempty"`
	URL          string `json:"url"`
	ResourceType string `json:"resource_type,omitempty"`
	Status       int    `json:"status,omitempty"`
	MimeType     string `json:"mime_type,omitempty"`
}

type AuthIndicators struct {
	MicrosoftLoginSeen bool `json:"microsoft_login_seen"`
	LoginPageLikely    bool `json:"login_page_likely"`
	Negotiate401Seen   bool `json:"negotiate_401_seen"`
	RedirectSeen       bool `json:"redirect_seen"`
	SelectorFound      bool `json:"selector_found"`
	BusinessPageLikely bool `json:"business_page_likely"`
}

type ProbeFiles struct {
	Screenshot string `json:"screenshot,omitempty"`
	HTML       string `json:"html,omitempty"`
	Network    string `json:"network,omitempty"`
	Summary    string `json:"summary,omitempty"`
	FetchAPI   string `json:"fetch_api,omitempty"`
}

type ProbeResult struct {
	InputURL       string         `json:"input_url"`
	FinalURL       string         `json:"final_url"`
	Title          string         `json:"title"`
	Selector       string         `json:"selector,omitempty"`
	SelectorFound  bool           `json:"selector_found"`
	ProfileDir     string         `json:"profile_dir"`
	BrowserPath    string         `json:"browser_path"`
	OutDir         string         `json:"out_dir"`
	BodyPreview    string         `json:"body_preview,omitempty"`
	AuthIndicators AuthIndicators `json:"auth_indicators"`
	APIEvents      []NetworkEvent `json:"api_events"`
	NetworkCount   int            `json:"network_count"`
	Files          ProbeFiles     `json:"files"`
	FetchAPIResult map[string]any `json:"fetch_api_result,omitempty"`
}

type ProbeError struct {
	Code    string
	Message string
	Hint    string
	Status  int
}

func (e *ProbeError) Error() string {
	return e.Message
}
