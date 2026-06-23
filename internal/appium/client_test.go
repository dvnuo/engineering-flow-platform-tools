package appium

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"engineering-flow-platform-tools/internal/browserstack"
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
