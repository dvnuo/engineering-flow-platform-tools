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
	now := time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login/device/code":
			_, _ = w.Write([]byte(`{"device_code":"dev","user_code":"ABCD-EFGH","verification_uri":"https://github.com/login/device","expires_in":60,"interval":1}`))
		case "/login/oauth/access_token":
			_, _ = w.Write([]byte(`{"access_token":"github-token"}`))
		case "/copilot":
			if r.Header.Get("Authorization") != "Bearer github-token" {
				t.Fatalf("bad auth header")
			}
			_, _ = w.Write([]byte(`{"token":"copilot-token","expires_at":1780000000}`))
		case "/user":
			_, _ = w.Write([]byte(`{"login":"octocat"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer s.Close()
	client := &DeviceClient{HTTPClient: s.Client(), GitHubBaseURL: s.URL, CopilotTokenURL: s.URL + "/copilot", Now: func() time.Time { return now }}
	var human strings.Builder
	cfg, result, err := client.Login(context.Background(), config.Default(), &human)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Auth.GitHubAccessToken != "github-token" || cfg.Auth.CopilotToken != "copilot-token" || result.GitHubUser != "octocat" {
		t.Fatalf("bad login cfg=%#v result=%#v", cfg.Auth, result)
	}
	if strings.Contains(human.String(), "github-token") || strings.Contains(human.String(), "copilot-token") {
		t.Fatal("token leaked")
	}
}
