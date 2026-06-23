package mobile

import "testing"

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
