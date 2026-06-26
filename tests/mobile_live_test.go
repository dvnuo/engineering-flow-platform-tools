package tests

import (
	"context"
	"os"
	"testing"
	"time"

	"engineering-flow-platform-tools/internal/appium"
	"engineering-flow-platform-tools/internal/browserstack"
)

func TestMobileBrowserStackLiveAccountSmoke(t *testing.T) {
	if os.Getenv("EFP_MOBILE_LIVE") != "1" {
		t.Skip("set EFP_MOBILE_LIVE=1 to run BrowserStack live account smoke")
	}
	creds := liveBrowserStackCredentials(t)
	control, err := browserstack.New("", creds, true, "")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := control.AuthTest(ctx); err != nil {
		t.Fatal(err)
	}
	devices, err := control.ListDevices(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) == 0 {
		t.Fatal("BrowserStack returned no App Automate devices")
	}
}

func TestMobileBrowserStackLivePublicSessionSmoke(t *testing.T) {
	if os.Getenv("EFP_MOBILE_LIVE_SESSION") != "1" {
		t.Skip("set EFP_MOBILE_LIVE_SESSION=1 to run a real BrowserStack Appium session")
	}
	appURL := os.Getenv("EFP_MOBILE_APP_URL")
	if appURL == "" {
		t.Skip("set EFP_MOBILE_APP_URL=bs://... to run a real BrowserStack Appium session")
	}
	creds := liveBrowserStackCredentials(t)
	client, err := appium.New("", creds, true, "")
	if err != nil {
		t.Fatal(err)
	}
	device := firstNonEmptyEnv("EFP_MOBILE_DEVICE", "Samsung Galaxy S22")
	version := firstNonEmptyEnv("EFP_MOBILE_OS_VERSION", "12.0")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	session, err := client.CreateSession(ctx, appium.CreateSessionRequest{
		PlatformName:             "android",
		AutomationName:           "UiAutomator2",
		App:                      appURL,
		DeviceName:               device,
		PlatformVersion:          version,
		SessionName:              "efp-mobile-auto-live-smoke",
		NetworkMode:              "public",
		InteractiveDebugging:     true,
		Video:                    true,
		IdleTimeoutSeconds:       300,
		NewCommandTimeoutSeconds: 300,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.DeleteSession(context.Background(), session.ID)
	if _, err := client.GetSource(ctx, session.ID); err != nil {
		t.Fatal(err)
	}
	if shot, err := client.Screenshot(ctx, session.ID); err != nil {
		t.Fatal(err)
	} else if len(shot) == 0 {
		t.Fatal("empty screenshot")
	}
}

func liveBrowserStackCredentials(t *testing.T) browserstack.Credentials {
	t.Helper()
	creds := browserstack.Credentials{
		Username:  os.Getenv("BROWSERSTACK_USERNAME"),
		AccessKey: os.Getenv("BROWSERSTACK_ACCESS_KEY"),
	}
	if creds.Username == "" || creds.AccessKey == "" {
		t.Skip("set BROWSERSTACK_USERNAME and BROWSERSTACK_ACCESS_KEY")
	}
	return creds
}

func firstNonEmptyEnv(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
