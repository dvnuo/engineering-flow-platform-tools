package httpclient

import (
	"context"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
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

func TestDisallowAbsoluteURLPathBoundary(t *testing.T) {
	v := true
	c, _ := New(config.InstanceConfig{BaseURL: "https://a.example.com/jira", RESTPath: "/rest/api/2", VerifySSL: &v, Auth: config.AuthConfig{Type: "bearer_token", Token: "t"}})
	_, err := c.Do(Request{Method: "GET", Path: "https://a.example.com/jiraevil/rest/api/2/myself"})
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

func TestClientUsesHTTPProxyFromEnvironment(t *testing.T) {
	clearProxyEnv(t)

	var proxyHits atomic.Int32
	var badProxyRequest atomic.Bool
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHits.Add(1)
		if !r.URL.IsAbs() || r.URL.Host != "jira.internal" {
			badProxyRequest.Store(true)
			http.Error(w, "bad proxy request", http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer proxy.Close()

	t.Setenv("HTTP_PROXY", proxy.URL)
	t.Setenv("NO_PROXY", "")
	t.Setenv("HTTPCLIENT_PROXY_HELPER", "http_proxy")
	t.Setenv("HTTPCLIENT_PROXY_BASE_URL", "http://jira.internal")

	runProxyEnvironmentHelper(t)

	if proxyHits.Load() == 0 {
		t.Fatal("expected proxy server to be hit")
	}
	if badProxyRequest.Load() {
		t.Fatal("expected proxy request to use an absolute target URL for jira.internal")
	}
}

func TestClientRespectsNOProxyFromEnvironment(t *testing.T) {
	clearProxyEnv(t)

	var targetHits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHits.Add(1)
		if r.URL.Path != "/rest/api/2/myself" {
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer target.Close()

	targetURL, err := url.Parse(target.URL)
	if err != nil {
		t.Fatal(err)
	}
	_, targetPort, err := net.SplitHostPort(targetURL.Host)
	if err != nil {
		t.Fatal(err)
	}
	dialHost := net.JoinHostPort("jira.internal", targetPort)

	var proxyHits atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHits.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"proxied":true}`))
	}))
	defer proxy.Close()

	t.Setenv("HTTP_PROXY", proxy.URL)
	t.Setenv("NO_PROXY", "jira.internal")
	t.Setenv("HTTPCLIENT_PROXY_HELPER", "no_proxy")
	t.Setenv("HTTPCLIENT_PROXY_BASE_URL", "http://"+dialHost)
	t.Setenv("HTTPCLIENT_PROXY_DIAL_HOST", dialHost)
	t.Setenv("HTTPCLIENT_PROXY_DIAL_ADDR", targetURL.Host)

	runProxyEnvironmentHelper(t)

	if targetHits.Load() == 0 {
		t.Fatal("expected target server to be hit")
	}
	if proxyHits.Load() != 0 {
		t.Fatalf("expected proxy to be bypassed, got %d proxy hits", proxyHits.Load())
	}
}

func TestProxyEnvironmentHelper(t *testing.T) {
	if os.Getenv("HTTPCLIENT_PROXY_HELPER") == "" {
		t.Skip("helper process only")
	}

	if dialHost := os.Getenv("HTTPCLIENT_PROXY_DIAL_HOST"); dialHost != "" {
		configureDefaultTransportDial(t, dialHost, os.Getenv("HTTPCLIENT_PROXY_DIAL_ADDR"))
	}

	baseURL := os.Getenv("HTTPCLIENT_PROXY_BASE_URL")
	if baseURL == "" {
		t.Fatal("missing HTTPCLIENT_PROXY_BASE_URL")
	}
	v := true
	c, err := New(config.InstanceConfig{Name: "x", BaseURL: baseURL, RESTPath: "/rest/api/2", VerifySSL: &v, Auth: config.AuthConfig{Type: "bearer_token", Token: "t"}})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Do(Request{Method: "GET", Path: "myself"})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
}

func TestClientAllowsSelfSignedServerWhenVerifySSLDisabled(t *testing.T) {
	clearProxyEnv(t)

	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer s.Close()

	v := false
	c, err := New(config.InstanceConfig{Name: "x", BaseURL: s.URL, RESTPath: "/rest/api/2", VerifySSL: &v, Auth: config.AuthConfig{Type: "bearer_token", Token: "t"}})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Do(Request{Method: "GET", Path: "myself"})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
}

func TestClientUsesConfiguredCACertificate(t *testing.T) {
	clearProxyEnv(t)

	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer s.Close()

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: s.Certificate().Raw})
	v := true
	c, err := New(config.InstanceConfig{Name: "x", BaseURL: s.URL, RESTPath: "/rest/api/2", VerifySSL: &v, CACert: string(certPEM), Auth: config.AuthConfig{Type: "bearer_token", Token: "t"}})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Do(Request{Method: "GET", Path: "myself"})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
}

func clearProxyEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"HTTP_PROXY",
		"HTTPS_PROXY",
		"ALL_PROXY",
		"NO_PROXY",
		"http_proxy",
		"https_proxy",
		"all_proxy",
		"no_proxy",
		"REQUEST_METHOD",
	} {
		t.Setenv(key, "")
	}
}

func runProxyEnvironmentHelper(t *testing.T) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestProxyEnvironmentHelper$", "-test.count=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("proxy environment helper failed: %v\n%s", err, output)
	}
}

func configureDefaultTransportDial(t *testing.T, dialHost, targetAddr string) {
	t.Helper()
	if targetAddr == "" {
		t.Fatal("missing HTTPCLIENT_PROXY_DIAL_ADDR")
	}
	original := http.DefaultTransport
	dialer := &net.Dialer{}
	http.DefaultTransport = &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if addr == dialHost {
				addr = targetAddr
			}
			return dialer.DialContext(ctx, network, addr)
		},
	}
	t.Cleanup(func() {
		http.DefaultTransport = original
	})
}
