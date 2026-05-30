package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"engineering-flow-platform-tools/internal/inspectimage/config"
)

var fixedNow = time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC)

func TestStatusMissingConfigEquivalent(t *testing.T) {
	cfg := config.Default()
	st := Summarize(cfg, time.Now())
	if st.AuthConfigured || st.CopilotTokenValid {
		t.Fatalf("unexpected auth: %#v", st)
	}
}

func TestLogoutClearsTokenFieldsOnly(t *testing.T) {
	cfg := config.Default()
	cfg.Defaults.Model = "gpt-5-mini"
	cfg.Auth.GitHubAccessToken = "gh"
	cfg.Auth.CopilotToken = "cp"
	out := Logout(cfg)
	if out.Auth.GitHubAccessToken != "" || out.Auth.CopilotToken != "" {
		t.Fatal("tokens not cleared")
	}
	if out.Defaults.Model != "gpt-5-mini" {
		t.Fatal("defaults changed")
	}
}

func TestDeviceFlowWithMockEndpoints(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login/device/code":
			if r.Header.Get("Content-Type") != "application/json" || r.Header.Get("User-Agent") != "GitHubCopilotChat/0.35.0" {
				t.Fatalf("device request did not match portal headers")
			}
			_, _ = w.Write([]byte(`{"device_code":"dev","user_code":"ABCD-EFGH","verification_uri":"https://github.com/login/device","expires_in":60,"interval":1}`))
		case "/login/oauth/access_token":
			if r.Header.Get("Content-Type") != "application/json" || r.Header.Get("User-Agent") != "GitHubCopilotChat/0.35.0" {
				t.Fatalf("poll request did not match portal headers")
			}
			_, _ = w.Write([]byte(`{"access_token":"github-token"}`))
		case "/user":
			_, _ = w.Write([]byte(`{"login":"octocat"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer s.Close()
	client := &DeviceClient{HTTPClient: s.Client(), GitHubBaseURL: s.URL, Now: func() time.Time { return fixedNow }}
	var human strings.Builder
	cfg, result, err := client.Login(context.Background(), config.Default(), &human)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Auth.GitHubAccessToken != "github-token" || cfg.Auth.CopilotToken != "github-token" || result.GitHubUser != "octocat" {
		t.Fatalf("bad login cfg=%#v result=%#v", cfg.Auth, result)
	}
	if strings.Contains(human.String(), "github-token") {
		t.Fatal("token leaked")
	}
}

func TestTokenWithoutExpiryIsValid(t *testing.T) {
	cfg := config.Default()
	cfg.Auth.CopilotToken = "token"
	if !TokenValid(cfg, fixedNow) {
		t.Fatal("expected portal-style token without expiry to be valid")
	}
}

func TestEndpointErrorIncludesSanitizedDetail(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"bad_verification_code"}`, http.StatusBadRequest)
	}))
	defer s.Close()
	client := &DeviceClient{HTTPClient: s.Client(), GitHubBaseURL: s.URL, Now: func() time.Time { return fixedNow }}
	_, err := client.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "bad_verification_code") {
		t.Fatalf("expected detail in error, got %v", err)
	}
}
