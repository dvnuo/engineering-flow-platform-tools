package httpclient

import (
	"net/http"
	"net/url"
	"testing"
)

func TestEffectiveProxyDiagnosticUsesExplicitProxy(t *testing.T) {
	diag, err := EffectiveProxyDiagnostic("https://api-cloud.browserstack.com", ProxySettings{
		ProxyHost: "proxy.internal",
		ProxyPort: 8080,
		ProxyUser: "user",
		ProxyPass: "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !diag.Enabled || diag.Source != "config" || diag.Host != "proxy.internal" || diag.Port != "8080" {
		t.Fatalf("diag=%#v", diag)
	}
	if text := ProxyDiagnosticText("https://api-cloud.browserstack.com", diag); text == "" || containsAny(text, "user", "secret") {
		t.Fatalf("diagnostic leaked secret or was empty: %q", text)
	}
}

func TestExplicitProxyHonorsNoProxyUnlessForced(t *testing.T) {
	settings := ProxySettings{ProxyHost: "proxy.internal", ProxyPort: 8080, NoProxyHosts: []string{"api-cloud.browserstack.com"}}
	target, _ := url.Parse("https://api-cloud.browserstack.com")
	proxy, source, err := proxyForURL(target, settings)
	if err != nil {
		t.Fatal(err)
	}
	if proxy != nil || source != "config_no_proxy" {
		t.Fatalf("proxy=%v source=%s", proxy, source)
	}
	settings.ForceProxy = true
	proxy, source, err = proxyForURL(target, settings)
	if err != nil {
		t.Fatal(err)
	}
	if proxy == nil || source != "config" {
		t.Fatalf("proxy=%v source=%s", proxy, source)
	}
}

func TestEnvProxyIgnoresEmptyValues(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("https_proxy", "")
	t.Setenv("ALL_PROXY", "")
	t.Setenv("all_proxy", "")
	diag, err := EffectiveProxyDiagnostic("https://api-cloud.browserstack.com", ProxySettings{})
	if err != nil {
		t.Fatal(err)
	}
	if diag.Enabled || diag.Source != "none" {
		t.Fatalf("diag=%#v", diag)
	}
}

func TestRejectsZeroPortProxy(t *testing.T) {
	_, _, err := NewTransport(TransportOptions{
		BaseURL: "https://api-cloud.browserstack.com",
		Proxy:   ProxySettings{ProxyHost: "proxy.internal", ProxyPort: 0, ForceProxy: true},
	})
	if err == nil {
		t.Fatal("expected proxy config error")
	}
	t.Setenv("HTTPS_PROXY", "http://proxy.internal:0")
	_, _, err = NewTransport(TransportOptions{BaseURL: "https://api-cloud.browserstack.com"})
	if err == nil {
		t.Fatal("expected env proxy config error")
	}
}

func TestTransportProxyFunctionUsesExplicitProxy(t *testing.T) {
	tr, _, err := NewTransport(TransportOptions{
		BaseURL: "https://api-cloud.browserstack.com",
		Proxy:   ProxySettings{ProxyHost: "proxy.internal", ProxyPort: 8080},
	})
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, "https://api-cloud.browserstack.com/app-automate/plan.json", nil)
	proxy, err := tr.Proxy(req)
	if err != nil {
		t.Fatal(err)
	}
	if proxy == nil || proxy.Host != "proxy.internal:8080" {
		t.Fatalf("proxy=%v", proxy)
	}
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if needle != "" && len(value) >= len(needle) {
			for i := 0; i+len(needle) <= len(value); i++ {
				if value[i:i+len(needle)] == needle {
					return true
				}
			}
		}
	}
	return false
}
