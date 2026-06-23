package appium

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"engineering-flow-platform-tools/internal/browserstack"
	"engineering-flow-platform-tools/internal/mobile"
)

func TestCreateSessionPublicDoesNotSetLocal(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/session" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if _, _, ok := r.BasicAuth(); !ok {
			t.Fatal("missing basic auth")
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(`{"value":{"sessionId":"abc","capabilities":{}}}`))
	}))
	defer srv.Close()
	c, err := New(srv.URL, browserstack.Credentials{Username: "u", AccessKey: "k"}, true, "")
	if err != nil {
		t.Fatal(err)
	}
	s, err := c.CreateSession(context.Background(), CreateSessionRequest{PlatformName: "android", AutomationName: "UiAutomator2", App: "bs://app", DeviceName: "Pixel", NetworkMode: "public"})
	if err != nil {
		t.Fatal(err)
	}
	if s.ID != "abc" {
		t.Fatalf("session id=%s", s.ID)
	}
	caps := body["capabilities"].(map[string]any)["alwaysMatch"].(map[string]any)
	bs := caps["bstack:options"].(map[string]any)
	if _, ok := bs["local"]; ok {
		t.Fatalf("public session set local capability: %#v", bs)
	}
}

func TestCreateSessionPrivateUsesBooleanLocalCapability(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(`{"value":{"sessionId":"abc","capabilities":{}}}`))
	}))
	defer srv.Close()
	c, err := New(srv.URL, browserstack.Credentials{Username: "u", AccessKey: "k"}, true, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.CreateSession(context.Background(), CreateSessionRequest{PlatformName: "android", AutomationName: "UiAutomator2", App: "bs://app", DeviceName: "Pixel", NetworkMode: "private-managed", LocalIdentifier: "local-1"})
	if err != nil {
		t.Fatal(err)
	}
	caps := body["capabilities"].(map[string]any)["alwaysMatch"].(map[string]any)
	bs := caps["bstack:options"].(map[string]any)
	if local, ok := bs["local"].(bool); !ok || !local {
		t.Fatalf("local capability should be boolean true: %#v", bs["local"])
	}
}

func TestCreateSessionSetsInteractiveDebuggingAndIdleTimeout(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(`{"value":{"sessionId":"abc","capabilities":{}}}`))
	}))
	defer srv.Close()
	c, err := New(srv.URL, browserstack.Credentials{Username: "u", AccessKey: "k"}, true, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.CreateSession(context.Background(), CreateSessionRequest{
		PlatformName:             "android",
		AutomationName:           "UiAutomator2",
		App:                      "bs://app",
		DeviceName:               "Pixel",
		InteractiveDebugging:     true,
		Video:                    true,
		IdleTimeoutSeconds:       300,
		NewCommandTimeoutSeconds: 300,
	})
	if err != nil {
		t.Fatal(err)
	}
	caps := body["capabilities"].(map[string]any)["alwaysMatch"].(map[string]any)
	bs := caps["bstack:options"].(map[string]any)
	if got, ok := bs["interactiveDebugging"].(bool); !ok || !got {
		t.Fatalf("interactiveDebugging should be true: %#v", bs)
	}
	if _, ok := bs["debug"]; ok {
		t.Fatalf("debug should not be used as an interactiveDebugging alias: %#v", bs)
	}
	if got, ok := bs["video"].(bool); !ok || !got {
		t.Fatalf("video should be true: %#v", bs)
	}
	if got, ok := bs["idleTimeout"].(float64); !ok || got != 300 {
		t.Fatalf("idleTimeout should be 300: %#v", bs)
	}
}

func TestCreateSessionRejectsUnsupportedIdleTimeout(t *testing.T) {
	c, err := New("http://127.0.0.1:1", browserstack.Credentials{Username: "u", AccessKey: "k"}, true, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.CreateSession(context.Background(), CreateSessionRequest{IdleTimeoutSeconds: 301})
	var me *mobile.Error
	if !errors.As(err, &me) || me.Code != "invalid_args" {
		t.Fatalf("expected invalid_args mobile error, got %#v", err)
	}
}

func TestFindElementsParsesW3CElementKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/session/s1/elements" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"value":[{"element-6066-11e4-a52e-4f735466cecf":"e1"}]}`))
	}))
	defer srv.Close()
	c, err := New(srv.URL, browserstack.Credentials{Username: "u", AccessKey: "k"}, true, "")
	if err != nil {
		t.Fatal(err)
	}
	elements, err := c.FindElements(context.Background(), "s1", Locator{Using: "id", Value: "login"})
	if err != nil {
		t.Fatal(err)
	}
	if len(elements) != 1 || elements[0].ID != "e1" {
		t.Fatalf("elements=%#v", elements)
	}
}

func TestSanitizeBoundsSnippetLength(t *testing.T) {
	got := sanitize(strings.Repeat("x", maxErrorSnippet+20), browserstack.Credentials{})
	if len(got) > maxErrorSnippet {
		t.Fatalf("len=%d max=%d", len(got), maxErrorSnippet)
	}
	if got[len(got)-3:] != "..." {
		t.Fatalf("missing ellipsis: %q", got[len(got)-10:])
	}
}
