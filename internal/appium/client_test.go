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
	"engineering-flow-platform-tools/internal/httpclient"
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

func TestClientHasBoundedHTTPTimeouts(t *testing.T) {
	c, err := New("http://127.0.0.1:1", browserstack.Credentials{Username: "u", AccessKey: "k"}, true, "")
	if err != nil {
		t.Fatal(err)
	}
	if c.http.Timeout != defaultHTTPTimeout {
		t.Fatalf("timeout=%s", c.http.Timeout)
	}
	tr, ok := c.http.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport=%T", c.http.Transport)
	}
	if tr.TLSHandshakeTimeout <= 0 || tr.ResponseHeaderTimeout <= 0 || tr.IdleConnTimeout <= 0 {
		t.Fatalf("transport timeouts not set: %#v", tr)
	}
	if tr.ResponseHeaderTimeout != sessionResponseHeaderTimeout {
		t.Fatalf("response header timeout=%s", tr.ResponseHeaderTimeout)
	}
}

func TestClientUsesExplicitProxyConfig(t *testing.T) {
	c, err := New("https://hub.browserstack.com/wd/hub", browserstack.Credentials{Username: "u", AccessKey: "k"}, true, "", httpclient.ProxySettings{ProxyHost: "proxy.internal", ProxyPort: 8080})
	if err != nil {
		t.Fatal(err)
	}
	diag := c.ProxyDiagnostic()
	if !diag.Enabled || diag.Source != "config" || diag.Host != "proxy.internal" || diag.Port != "8080" {
		t.Fatalf("diag=%#v", diag)
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

func TestCreateSessionPreservesRemoteLocalError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"value":{"error":"unknown error","message":"BrowserStack Local is required to access this host"}}`))
	}))
	defer srv.Close()
	c, err := New(srv.URL, browserstack.Credentials{Username: "u", AccessKey: "k"}, true, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.CreateSession(context.Background(), CreateSessionRequest{PlatformName: "android"})
	var me *mobile.Error
	if !errors.As(err, &me) || me.Code != "local_tunnel_required" {
		t.Fatalf("expected local_tunnel_required, got %#v", err)
	}
	var missing *SessionIDMissingError
	if !errors.As(err, &missing) || missing.Response == nil {
		t.Fatalf("missing response was not preserved: %#v", err)
	}
}

func TestCreateSessionPreservesRemoteCapabilityError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"value":{"error":"invalid argument","message":"Invalid capabilities: appium:deviceName is not supported"}}`))
	}))
	defer srv.Close()
	c, err := New(srv.URL, browserstack.Credentials{Username: "u", AccessKey: "k"}, true, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.CreateSession(context.Background(), CreateSessionRequest{PlatformName: "android"})
	var me *mobile.Error
	if !errors.As(err, &me) || me.Code != "invalid_capabilities" {
		t.Fatalf("expected invalid_capabilities, got %#v", err)
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
