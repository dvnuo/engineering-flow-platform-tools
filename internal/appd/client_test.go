package appd

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/httpclient"
	"engineering-flow-platform-tools/internal/testutil"
)

func apiClientInstance(base string) config.InstanceConfig {
	return config.InstanceConfig{Name: "local", BaseURL: base, Account: "customer1", Auth: config.AuthConfig{Type: AuthAPIClient, Username: "efp-reader", APIKey: "secret-api-key-should-not-appear"}}
}

func TestClientCachesOAuthTokenInMemoryUntilExpiry(t *testing.T) {
	mock := testutil.NewMockAppD(t)
	c, err := New(apiClientInstance(mock.Server.URL))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 19, 8, 0, 0, 0, time.UTC)
	c.now = func() time.Time { return now }
	for i := 0; i < 2; i++ {
		if _, err := c.Get("/controller/rest/applications", nil); err != nil {
			t.Fatal(err)
		}
	}
	if mock.TokenHits != 1 {
		t.Fatalf("token should be cached within its lifetime, hits=%d", mock.TokenHits)
	}
	now = now.Add(4*time.Minute + 45*time.Second)
	if _, err := c.Get("/controller/rest/applications", nil); err != nil {
		t.Fatal(err)
	}
	if mock.TokenHits != 2 {
		t.Fatalf("token should be refreshed before expires_in elapses, hits=%d", mock.TokenHits)
	}
	if c.AuthType() != AuthAPIClient || c.Identity() != "efp-reader@customer1" {
		t.Fatalf("identity=%q type=%q", c.Identity(), c.AuthType())
	}
}

func TestClientNormalizeTypeMovesTokenIntoAPIKey(t *testing.T) {
	mock := testutil.NewMockAppD(t)
	inst := apiClientInstance(mock.Server.URL)
	inst.Auth = config.AuthConfig{Type: "api_client", Username: "efp-reader", Token: "secret-api-key-should-not-appear"}
	c, err := New(inst)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Get("/controller/rest/applications", nil); err != nil {
		t.Fatalf("token stored as auth.token should work as the client secret: %v", err)
	}
}

func TestClientBearerTokenAuth(t *testing.T) {
	mock := testutil.NewMockAppD(t)
	inst := config.InstanceConfig{Name: "local", BaseURL: mock.Server.URL, Auth: config.AuthConfig{Type: "bearer_token", Token: mock.Token}}
	c, err := New(inst)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Get("/controller/rest/applications", nil); err != nil {
		t.Fatal(err)
	}
	if mock.TokenHits != 0 || mock.LastAuth != "Bearer "+mock.Token || c.Identity() != "" {
		t.Fatalf("bearer auth: hits=%d auth=%q identity=%q", mock.TokenHits, mock.LastAuth, c.Identity())
	}
}

func TestNormalizeBaseURL(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"https://appd.example.test:8090", "https://appd.example.test:8090"},
		{"https://appd.example.test:8090/", "https://appd.example.test:8090"},
		{"https://appd.example.test/controller", "https://appd.example.test"},
		{"https://appd.example.test/Controller/", "https://appd.example.test"},
		{" https://proxy.example.test/appd/controller ", "https://proxy.example.test/appd"},
	} {
		got, err := NormalizeBaseURL(tc.in)
		if err != nil || got != tc.want {
			t.Fatalf("NormalizeBaseURL(%q)=%q err=%v want %q", tc.in, got, err, tc.want)
		}
	}
	for _, bad := range []string{"", "appd.example.test", "ftp://appd.example.test", "https://"} {
		if _, err := NormalizeBaseURL(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}

func TestClientRESTPathGuard(t *testing.T) {
	c, err := New(apiClientInstance("https://appd.example.test/appd"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		in        string
		wantPath  string
		wantQuery map[string]string
	}{
		{"/controller/rest/applications", "/controller/rest/applications", map[string]string{}},
		{"controller/rest/applications/ecommerce/tiers?a=1&b=2", "/controller/rest/applications/ecommerce/tiers", map[string]string{"a": "1", "b": "2"}},
		{"/controller/rest/applications/ecommerce/../payments", "/controller/rest/applications/payments", map[string]string{}},
		{"https://appd.example.test/appd/controller/rest/applications?x=y", "/controller/rest/applications", map[string]string{"x": "y"}},
	} {
		p, q, err := c.RESTPath(tc.in)
		if err != nil || p != tc.wantPath || len(q) != len(tc.wantQuery) {
			t.Fatalf("RESTPath(%q)=%q %v err=%v want %q %v", tc.in, p, q, err, tc.wantPath, tc.wantQuery)
		}
		for k, v := range tc.wantQuery {
			if q[k] != v {
				t.Fatalf("RESTPath(%q) query %s=%q want %q", tc.in, k, q[k], v)
			}
		}
	}
	for _, tc := range []struct{ in, code string }{
		{"/controller/api/oauth/access_token", "invalid_args"},
		{"/controller/rest/../api/oauth/access_token", "invalid_args"},
		{"/controller/rest", "invalid_args"},
		{"/api/json", "invalid_args"},
		{"https://appd.example.test/controller/rest/applications", "instance_url_mismatch"},
		{"https://other.example.test/appd/controller/rest/applications", "instance_url_mismatch"},
	} {
		_, _, err := c.RESTPath(tc.in)
		var httpErr *httpclient.HTTPError
		if !errors.As(err, &httpErr) || httpErr.Code != tc.code {
			t.Fatalf("RESTPath(%q) err=%v want code %s", tc.in, err, tc.code)
		}
	}
}

func TestClientResolveURLAddsOutputJSONAndEncodesMetricPath(t *testing.T) {
	c, err := New(apiClientInstance("https://appd.example.test/controller"))
	if err != nil {
		t.Fatal(err)
	}
	u, err := c.ResolveURL("/controller/rest/applications/ecommerce/metric-data-v2", map[string]string{"metric-path": "Overall Application Performance|Average Response Time (ms)"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(u, "https://appd.example.test/controller/rest/applications/ecommerce/metric-data-v2?") || !strings.Contains(u, "output=JSON") || !strings.Contains(u, "metric-path=Overall+Application+Performance%7CAverage+Response+Time+%28ms%29") {
		t.Fatalf("bad url: %s", u)
	}
	u, err = c.ResolveURL("/controller/rest/applications", map[string]string{"output": "XML"})
	if err != nil || !strings.Contains(u, "output=XML") {
		t.Fatalf("explicit output should win: %s err=%v", u, err)
	}
	if _, err := c.ResolveURL("https://evil.example/controller/rest/applications", nil); err == nil {
		t.Fatal("off-instance absolute url must be rejected")
	}
}

func TestClientMapsStatusesAndScrubsSecrets(t *testing.T) {
	status := http.StatusOK
	body := `{"ok":true}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	inst := config.InstanceConfig{Name: "local", BaseURL: srv.URL, Account: "customer1", Auth: config.AuthConfig{Type: AuthBasic, Username: "reader", Password: "secret-password-should-not-appear"}}
	c, err := New(inst)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		status int
		code   string
	}{
		{http.StatusBadRequest, "invalid_args"},
		{http.StatusUnauthorized, "auth_failed"},
		{http.StatusForbidden, "permission_denied"},
		{http.StatusNotFound, "not_found"},
		{http.StatusTooManyRequests, "rate_limited"},
		{http.StatusBadGateway, "server_error"},
	} {
		status = tc.status
		body = `{"error":"echo secret-password-should-not-appear Authorization: Basic abc"}`
		_, err := c.Get("/controller/rest/applications", nil)
		var httpErr *httpclient.HTTPError
		if !errors.As(err, &httpErr) || httpErr.Code != tc.code || httpErr.Status != tc.status {
			t.Fatalf("status %d -> %v, want %s", tc.status, err, tc.code)
		}
		if strings.Contains(httpErr.Message, "secret-password-should-not-appear") || strings.Contains(httpErr.Message, "Basic abc") {
			t.Fatalf("error message leaked a secret: %s", httpErr.Message)
		}
	}
	status = http.StatusOK
	body = `<html>login page</html>`
	_, err = c.Get("/controller/rest/applications", nil)
	var httpErr *httpclient.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Code != "server_error" || !strings.Contains(httpErr.Message, "non-JSON") {
		t.Fatalf("non-json body: %v", err)
	}
}

func TestClientNetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	base := srv.URL
	srv.Close()
	c, err := New(apiClientInstance(base))
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Get("/controller/rest/applications", nil)
	var httpErr *httpclient.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Code != "network_error" {
		t.Fatalf("expected network_error, got %v", err)
	}
	if strings.Contains(httpErr.Message, "secret-api-key-should-not-appear") {
		t.Fatalf("network error leaked the secret: %s", httpErr.Message)
	}
}

func TestClientOAuthFailuresBecomeAuthFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == OAuthTokenPath {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"token_type":"Bearer"}`))
			return
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	c, err := New(apiClientInstance(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Get("/controller/rest/applications", nil)
	var httpErr *httpclient.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Code != "auth_failed" || !strings.Contains(httpErr.Message, "access_token") {
		t.Fatalf("missing access_token should be auth_failed: %v", err)
	}
	if strings.Contains(httpErr.Message, "secret-api-key-should-not-appear") || strings.Contains(httpErr.Hint, "secret-api-key-should-not-appear") {
		t.Fatalf("oauth error leaked the secret: %v", httpErr)
	}
}

func TestClientRejectsBadConfig(t *testing.T) {
	for _, inst := range []config.InstanceConfig{
		{Name: "no-url", Auth: config.AuthConfig{Type: AuthAPIClient, Username: "c", APIKey: "k"}},
		{Name: "no-auth", BaseURL: "https://appd.example.test"},
		{Name: "no-account", BaseURL: "https://appd.example.test", Auth: config.AuthConfig{Type: AuthAPIClient, Username: "c", APIKey: "k"}},
		{Name: "no-secret", BaseURL: "https://appd.example.test", Account: "a", Auth: config.AuthConfig{Type: AuthAPIClient, Username: "c"}},
		{Name: "basic-no-password", BaseURL: "https://appd.example.test", Account: "a", Auth: config.AuthConfig{Type: AuthBasic, Username: "u"}},
		{Name: "basic-api-key", BaseURL: "https://appd.example.test", Account: "a", Auth: config.AuthConfig{Type: "basic_api_key", Username: "u", APIKey: "k"}},
		{Name: "bad-ca", BaseURL: "https://appd.example.test", Account: "a", CACert: "not a pem", Auth: config.AuthConfig{Type: AuthAPIClient, Username: "c", APIKey: "k"}},
	} {
		_, err := New(inst)
		var httpErr *httpclient.HTTPError
		if !errors.As(err, &httpErr) || httpErr.Code != "config_error" {
			t.Fatalf("%s: expected config_error, got %v", inst.Name, err)
		}
	}
}
