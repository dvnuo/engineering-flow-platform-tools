package httpclient

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"engineering-flow-platform-tools/internal/config"
)

func TestClientHTTPEndToEnd(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/2/issue/ABC-1" {
			t.Fatalf("bad path %s", r.URL.Path)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer s.Close()
	v := true
	c, err := New(config.InstanceConfig{Name: "x", BaseURL: s.URL, RESTPath: "/rest/api/2", VerifySSL: &v, Auth: config.AuthConfig{Type: "bearer_token", Token: "t"}})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Do(Request{Method: "GET", Path: "issue/ABC-1"})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)
}
func TestDisallowOtherDomain(t *testing.T) {
	v := true
	c, _ := New(config.InstanceConfig{BaseURL: "https://a.example.com", RESTPath: "/rest/api/2", VerifySSL: &v, Auth: config.AuthConfig{Type: "bearer_token", Token: "t"}})
	_, err := c.Do(Request{Method: "GET", Path: "https://evil.example.com/steal"})
	if err == nil || !strings.Contains(err.Error(), "instance_url_mismatch") {
		t.Fatal("should fail")
	}
}

func TestHTTPStatusErrorCodes(t *testing.T) {
	cases := map[int]string{
		401: "auth_failed",
		403: "permission_denied",
		404: "not_found",
		429: "rate_limited",
		500: "server_error",
	}
	for status, code := range cases {
		t.Run(code, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			defer s.Close()
			v := true
			c, err := New(config.InstanceConfig{Name: "x", BaseURL: s.URL, RESTPath: "/rest/api/2", VerifySSL: &v, Auth: config.AuthConfig{Type: "bearer_token", Token: "t"}})
			if err != nil {
				t.Fatal(err)
			}
			_, err = c.Do(Request{Method: "GET", Path: "myself"})
			if err == nil {
				t.Fatal("expected error")
			}
			httpErr, ok := err.(*HTTPError)
			if !ok {
				t.Fatalf("expected HTTPError, got %T", err)
			}
			if httpErr.Code != code {
				t.Fatalf("code=%s want=%s", httpErr.Code, code)
			}
		})
	}
}
