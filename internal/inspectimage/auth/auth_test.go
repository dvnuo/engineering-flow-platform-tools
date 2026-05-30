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
		case "/copilot_internal/v2/token":
			if r.Header.Get("Authorization") != "Bearer github-token" || r.Header.Get("Editor-Version") != "vscode/1.107.0" || r.Header.Get("Copilot-Integration-Id") != "vscode-chat" {
				t.Fatalf("exchange request did not match runtime headers")
			}
			_, _ = w.Write([]byte(`{"token":"tid=abc;proxy-ep=proxy.individual.githubcopilot.com;","expires_at":1780000000}`))
		case "/user":
			_, _ = w.Write([]byte(`{"login":"octocat"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer s.Close()
	client := &DeviceClient{HTTPClient: s.Client(), GitHubBaseURL: s.URL, CopilotGitHubAPIBaseURL: s.URL, Now: func() time.Time { return fixedNow }}
	var human strings.Builder
	cfg, result, err := client.Login(context.Background(), config.Default(), &human)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Auth.GitHubAccessToken != "github-token" || cfg.Auth.CopilotToken == "" || cfg.Auth.CopilotToken == "github-token" || result.GitHubUser != "octocat" {
		t.Fatalf("bad login cfg=%#v result=%#v", cfg.Auth, result)
	}
	if cfg.Auth.CopilotTokenExpiresAt == "" || cfg.API.BaseURL != "https://api.individual.githubcopilot.com" {
		t.Fatalf("exchange metadata not stored: cfg=%#v", cfg)
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

func TestNeedsExchangeForOAuthToken(t *testing.T) {
	cfg := config.Default()
	cfg.Auth.GitHubAccessToken = "gho_source"
	cfg.Auth.CopilotToken = "gho_source"
	if !NeedsExchange(cfg) || TokenValid(cfg, fixedNow) {
		t.Fatal("expected source token to require exchange")
	}
}

func TestParseCopilotAPIBaseURL(t *testing.T) {
	got := ParseCopilotAPIBaseURL("tid=abc;proxy-ep=proxy.enterprise.githubcopilot.com;")
	if got != "https://api.enterprise.githubcopilot.com" {
		t.Fatalf("base=%q", got)
	}
}

func TestEndpointErrorIncludesSanitizedDetail(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad_verification_code gho_SECRET tid=SECRET","code":"bad_verification_code","type":"oauth_error"},"request_id":"req_123"}`))
	}))
	defer s.Close()
	client := &DeviceClient{HTTPClient: s.Client(), GitHubBaseURL: s.URL, Now: func() time.Time { return fixedNow }}
	_, err := client.Start(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{"bad_verification_code", "code=bad_verification_code", "type=oauth_error", "request_id=req_123"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("missing %q in error %s", want, msg)
		}
	}
	if strings.Contains(msg, "gho_SECRET") || strings.Contains(msg, "tid=SECRET") {
		t.Fatalf("expected detail in error, got %v", err)
	}
}
