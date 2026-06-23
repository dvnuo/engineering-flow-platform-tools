package browserstack

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestNewSetsBoundedHTTPTimeouts(t *testing.T) {
	c, err := New("http://127.0.0.1:1", Credentials{}, true, "")
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
}

func TestUploadAppStreamsMultipartFile(t *testing.T) {
	var sawFile bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/app-automate/upload" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		if err := r.ParseMultipartForm(1024); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		if r.FormValue("custom_id") != "custom-app" || r.FormValue("ios_keychain_support") != "true" {
			t.Fatalf("unexpected fields: %#v", r.MultipartForm.Value)
		}
		f, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("FormFile: %v", err)
		}
		defer f.Close()
		body, _ := io.ReadAll(f)
		if header.Filename != "app.apk" || string(body) != "fake app payload" {
			t.Fatalf("bad file %s %q", header.Filename, string(body))
		}
		sawFile = true
		_, _ = w.Write([]byte(`{"app_url":"bs://uploaded"}`))
	}))
	defer srv.Close()
	appPath := filepath.Join(t.TempDir(), "app.apk")
	if err := os.WriteFile(appPath, []byte("fake app payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := New(srv.URL, Credentials{Username: "user", AccessKey: "key"}, true, "")
	if err != nil {
		t.Fatal(err)
	}
	app, err := c.UploadApp(context.Background(), UploadAppRequest{FilePath: appPath, CustomID: "custom-app", IOSKeychainSupport: true, SHA256: "sha"})
	if err != nil {
		t.Fatal(err)
	}
	if !sawFile || app.AppURL != "bs://uploaded" || app.SHA256 != "sha" {
		t.Fatalf("unexpected upload result saw=%v app=%#v", sawFile, app)
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
