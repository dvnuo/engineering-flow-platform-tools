package appium

type CreateSessionRequest struct {
	PlatformName             string
	AutomationName           string
	App                      string
	DeviceName               string
	PlatformVersion          string
	ProjectName              string
	BuildName                string
	SessionName              string
	NetworkMode              string
	LocalIdentifier          string
	InteractiveDebugging     bool
	Debug                    bool
	Video                    bool
	IdleTimeoutSeconds       int
	NewCommandTimeoutSeconds int
	ExtraCaps                map[string]any
}

type Session struct {
	ID           string         `json:"id"`
	Capabilities map[string]any `json:"capabilities,omitempty"`
}

type Locator struct {
	Using string `json:"using"`
	Value string `json:"value"`
}

type RemoteElement struct {
	ID string `json:"id"`
}

type Rect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type ActionsRequest struct {
	Actions []map[string]any `json:"actions"`
}
