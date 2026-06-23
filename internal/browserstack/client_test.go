package browserstack

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListDevicesUsesDocumentedPathAndAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/app-automate/devices.json" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		u, p, ok := r.BasicAuth()
		if !ok || u != "user" || p != "key" {
			t.Fatalf("bad auth")
		}
		_, _ = w.Write([]byte(`[{"os":"android","os_version":"14.0","device":"Pixel 8","realMobile":true}]`))
	}))
	defer srv.Close()
	c, err := New(srv.URL, Credentials{Username: "user", AccessKey: "key"}, true, "")
	if err != nil {
		t.Fatal(err)
	}
	devices, err := c.ListDevices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 || devices[0].Name != "Pixel 8" {
		t.Fatalf("devices=%#v", devices)
	}
}

func TestStatusErrorRedactsCredentials(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("user:key"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"bad user key Basic ` + encoded + `"}`))
	}))
	defer srv.Close()
	c, err := New(srv.URL, Credentials{Username: "user", AccessKey: "key"}, true, "")
	if err != nil {
		t.Fatal(err)
	}
	err = c.AuthTest(context.Background())
	if err == nil {
		t.Fatal("expected auth error")
	}
	msg := err.Error()
	if strings.Contains(msg, "user") || strings.Contains(msg, "key") || strings.Contains(msg, encoded) {
		t.Fatalf("secret leaked: %s", msg)
	}
}

func TestSanitizeSnippetBoundsLength(t *testing.T) {
	got := sanitizeSnippet([]byte(strings.Repeat("x", maxErrorSnippet+20)), Credentials{})
	if len(got) > maxErrorSnippet {
		t.Fatalf("len=%d max=%d", len(got), maxErrorSnippet)
	}
	if got[len(got)-3:] != "..." {
		t.Fatalf("missing ellipsis: %q", got[len(got)-10:])
	}
}
