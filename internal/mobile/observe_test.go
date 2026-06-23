package mobile

import (
	"strconv"
	"strings"
	"testing"
)

func TestExtractCandidatesAndroidRedactsPassword(t *testing.T) {
	source := `<hierarchy>
  <node class="android.widget.EditText" text="secret123" password="true" resource-id="com.example:id/password" enabled="true" clickable="true" bounds="[1,2][101,52]"/>
  <node class="android.widget.Button" text="Login" content-desc="Login" enabled="true" clickable="true" bounds="[10,20][110,70]"/>
</hierarchy>`
	candidates := ExtractCandidates(source, "obs-1")
	if len(candidates) != 2 {
		t.Fatalf("len=%d", len(candidates))
	}
	if candidates[0].Text != "" || candidates[0].Value != "" || !candidates[0].Password {
		t.Fatalf("password candidate was not redacted: %#v", candidates[0])
	}
	button := candidates[1]
	if button.Ref != "obs-1:e2" || button.Role != "button" || button.Text != "Login" || !button.Clickable {
		t.Fatalf("unexpected button candidate: %#v", button)
	}
	if button.Bounds.Width != 100 || button.Bounds.Height != 50 {
		t.Fatalf("bounds not parsed: %#v", button.Bounds)
	}
}

func TestLocateDeterministicAndAmbiguous(t *testing.T) {
	obs := Observation{ID: "obs-1", Candidates: []Candidate{
		{Ref: "obs-1:e1", CandidateID: "e1", Role: "button", Text: "Login", AccessibilityID: "Login", Enabled: true, Visible: true},
		{Ref: "obs-1:e2", CandidateID: "e2", Role: "button", Text: "Login", Enabled: true, Visible: true},
	}}
	res := Locate(obs, LocateQuery{Role: "button", Name: "Login", Actionable: true})
	if res.RecommendedRef != "obs-1:e1" || res.Ambiguous {
		t.Fatalf("accessibility id should win: %#v", res)
	}
	obs.Candidates[0].AccessibilityID = ""
	res = Locate(obs, LocateQuery{Role: "button", Name: "Login", Actionable: true})
	if !res.Ambiguous || res.RecommendedRef != "" {
		t.Fatalf("same score buttons should be ambiguous: %#v", res)
	}
}

func TestBuildObservationOmitsEmptyScreenshotHash(t *testing.T) {
	obs := BuildObservation("run-1", "session-1", "obs-1", `<hierarchy/>`, nil, 100)
	if obs.ScreenshotHash != "" {
		t.Fatalf("screenshot hash should be empty when no screenshot is captured: %s", obs.ScreenshotHash)
	}
	obs = BuildObservation("run-1", "session-1", "obs-1", `<hierarchy/>`, []byte("png"), 100)
	if obs.ScreenshotHash == "" {
		t.Fatal("screenshot hash should be set when screenshot bytes are present")
	}
}

func TestDeviceResolveLatestCompatible(t *testing.T) {
	devices := []DeviceInfo{
		{OS: "android", OSVersion: "12.0", Name: "Pixel 5", RealMobile: true},
		{OS: "android", OSVersion: "14.0", Name: "Pixel 8", RealMobile: true},
		{OS: "ios", OSVersion: "17", Name: "iPhone 15", RealMobile: true},
	}
	res, err := ResolveDevice(devices, DeviceQuery{Platform: "android", MinOSVersion: "11", RealOnly: true, Strategy: "latest-compatible"})
	if err != nil {
		t.Fatalf("ResolveDevice: %v", err)
	}
	if res.Recommended.Name != "Pixel 8" {
		t.Fatalf("recommended=%#v", res.Recommended)
	}
}

func TestExtractCandidatesKeepsNumericOrder(t *testing.T) {
	source := `<hierarchy>`
	for i := 1; i <= 12; i++ {
		source += `<node class="android.widget.Button" text="Button` + strconv.Itoa(i) + `" enabled="true" clickable="true"/>`
	}
	source += `</hierarchy>`
	candidates := ExtractCandidates(source, "obs-1")
	if len(candidates) != 12 {
		t.Fatalf("len=%d", len(candidates))
	}
	if candidates[1].CandidateID != "e2" || candidates[9].CandidateID != "e10" {
		t.Fatalf("candidate order was not numeric: e2=%s e10slot=%s", candidates[1].CandidateID, candidates[9].CandidateID)
	}
}

func TestLocatorHintsEscapeXPathLiteral(t *testing.T) {
	c := Candidate{Class: "android.widget.Button", Name: `Save John's "draft"`}
	hints := LocatorHints(c)
	var xpath string
	for _, hint := range hints {
		if hint.Using == "xpath" {
			xpath = hint.Value
			break
		}
	}
	if xpath == "" {
		t.Fatal("missing xpath hint")
	}
	if !strings.Contains(xpath, `concat(`) || strings.Contains(xpath, `@name="Save John's "draft""`) {
		t.Fatalf("xpath literal was not escaped: %s", xpath)
	}
}

func TestLocatorHintsEscapeSelectorBackslashes(t *testing.T) {
	hints := LocatorHints(Candidate{Text: `C:\Temp "draft"`})
	for _, hint := range hints {
		if hint.Using == "-android uiautomator" || hint.Using == "-ios predicate string" {
			if !strings.Contains(hint.Value, `C:\\Temp \"draft\"`) {
				t.Fatalf("selector hint was not escaped: %#v", hint)
			}
		}
	}
}
