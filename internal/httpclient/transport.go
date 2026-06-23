package httpclient

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type ProxySettings struct {
	ProxyHost             string
	ProxyPort             int
	ProxyUser             string
	ProxyPass             string
	NoProxyHosts          []string
	DisableProxyDiscovery bool
	ForceProxy            bool
}

type TransportOptions struct {
	BaseURL               string
	VerifySSL             bool
	CACert                string
	TLSHandshakeTimeout   time.Duration
	ResponseHeaderTimeout time.Duration
	IdleConnTimeout       time.Duration
	Proxy                 ProxySettings
}

type ProxyDiagnostic struct {
	Enabled               bool     `json:"enabled"`
	Source                string   `json:"source"`
	Host                  string   `json:"host,omitempty"`
	Port                  string   `json:"port,omitempty"`
	NoProxy               []string `json:"no_proxy,omitempty"`
	ForceProxy            bool     `json:"force_proxy,omitempty"`
	DisableProxyDiscovery bool     `json:"disable_proxy_discovery,omitempty"`
	TransportMode         string   `json:"transport_mode"`
}

type TransportConfigError struct {
	Message string
}

func (e *TransportConfigError) Error() string {
	return e.Message
}

func NewTransport(opts TransportOptions) (*http.Transport, ProxyDiagnostic, error) {
	tr := cloneDefaultTransport()
	tr.Proxy = proxyFunc(opts.Proxy)
	tr.TLSClientConfig = &tls.Config{}
	tr.TLSHandshakeTimeout = durationOrDefault(opts.TLSHandshakeTimeout, 10*time.Second)
	tr.ResponseHeaderTimeout = opts.ResponseHeaderTimeout
	tr.IdleConnTimeout = durationOrDefault(opts.IdleConnTimeout, 90*time.Second)
	if !opts.VerifySSL {
		tr.TLSClientConfig.InsecureSkipVerify = true
	}
	if strings.TrimSpace(opts.CACert) != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(opts.CACert)) {
			return nil, ProxyDiagnostic{}, errors.New("invalid CA certificate")
		}
		tr.TLSClientConfig.RootCAs = pool
	}
	diag, err := EffectiveProxyDiagnostic(opts.BaseURL, opts.Proxy)
	if err != nil {
		return nil, diag, err
	}
	return tr, diag, nil
}

func cloneDefaultTransport() *http.Transport {
	baseTransport, ok := http.DefaultTransport.(*http.Transport)
	if ok && baseTransport != nil {
		return baseTransport.Clone()
	}
	return &http.Transport{}
}

func durationOrDefault(value, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
}

func proxyFunc(settings ProxySettings) func(*http.Request) (*url.URL, error) {
	return func(req *http.Request) (*url.URL, error) {
		proxy, _, err := proxyForURL(req.URL, settings)
		return proxy, err
	}
}

func EffectiveProxyDiagnostic(rawTarget string, settings ProxySettings) (ProxyDiagnostic, error) {
	target, err := url.Parse(strings.TrimSpace(rawTarget))
	if err != nil || target.Scheme == "" || target.Host == "" {
		return ProxyDiagnostic{}, &TransportConfigError{Message: "invalid proxy target URL"}
	}
	proxy, source, err := proxyForURL(target, settings)
	diag := ProxyDiagnostic{
		Enabled:               proxy != nil,
		Source:                source,
		NoProxy:               append([]string{}, settings.NoProxyHosts...),
		ForceProxy:            settings.ForceProxy,
		DisableProxyDiscovery: settings.DisableProxyDiscovery,
		TransportMode:         "http1",
	}
	if proxy != nil {
		diag.Host = proxy.Hostname()
		diag.Port = proxy.Port()
	}
	return diag, err
}

func proxyForURL(target *url.URL, settings ProxySettings) (*url.URL, string, error) {
	if target == nil {
		return nil, "none", nil
	}
	if explicitProxyConfigured(settings) {
		proxy, err := explicitProxyURL(settings)
		if err != nil {
			return nil, "config", err
		}
		if !settings.ForceProxy && bypassProxy(target, settings.NoProxyHosts) {
			return nil, "config_no_proxy", nil
		}
		return proxy, "config", nil
	}
	if settings.ForceProxy {
		return nil, "config", &TransportConfigError{Message: "force_proxy requires explicit proxy_host and proxy_port"}
	}
	if settings.DisableProxyDiscovery {
		return nil, "none", nil
	}
	proxy, source, err := envProxyURL(target, settings.ForceProxy)
	if err != nil {
		return nil, source, err
	}
	return proxy, source, nil
}

func explicitProxyConfigured(settings ProxySettings) bool {
	return strings.TrimSpace(settings.ProxyHost) != "" || settings.ProxyPort != 0 || strings.TrimSpace(settings.ProxyUser) != "" || strings.TrimSpace(settings.ProxyPass) != ""
}

func explicitProxyURL(settings ProxySettings) (*url.URL, error) {
	host := strings.TrimSpace(settings.ProxyHost)
	if host == "" {
		return nil, &TransportConfigError{Message: "proxy_host is required when explicit proxy settings are used"}
	}
	if settings.ProxyPort < 0 {
		return nil, &TransportConfigError{Message: "proxy_port must be greater than zero"}
	}
	proxyURL, err := parseProxyHost(host)
	if err != nil {
		return nil, err
	}
	if settings.ProxyPort > 0 {
		proxyURL.Host = net.JoinHostPort(proxyURL.Hostname(), strconv.Itoa(settings.ProxyPort))
	}
	if proxyURL.Port() == "" {
		return nil, &TransportConfigError{Message: "proxy_port is required when explicit proxy settings are used"}
	}
	if proxyURL.Port() == "0" {
		return nil, &TransportConfigError{Message: "proxy_port must be greater than zero"}
	}
	if strings.TrimSpace(settings.ProxyUser) != "" || strings.TrimSpace(settings.ProxyPass) != "" {
		proxyURL.User = url.UserPassword(strings.TrimSpace(settings.ProxyUser), strings.TrimSpace(settings.ProxyPass))
	}
	return proxyURL, nil
}

func parseProxyHost(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, &TransportConfigError{Message: "invalid proxy_host"}
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, &TransportConfigError{Message: "proxy_host must use http or https"}
	}
	if u.User != nil {
		u.User = nil
	}
	u.Path = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u, nil
}

func envProxyURL(target *url.URL, force bool) (*url.URL, string, error) {
	if !force && bypassProxy(target, splitEnvNoProxy()) {
		return nil, "env_no_proxy", nil
	}
	source, raw := envProxyCandidate(target.Scheme)
	if strings.TrimSpace(raw) == "" {
		return nil, "none", nil
	}
	u, err := parseEnvProxy(raw)
	if err != nil {
		return nil, source, err
	}
	return u, source, nil
}

func envProxyCandidate(scheme string) (string, string) {
	keys := []string{}
	if strings.EqualFold(scheme, "https") {
		keys = append(keys, "HTTPS_PROXY", "https_proxy")
	} else {
		keys = append(keys, "HTTP_PROXY", "http_proxy")
	}
	keys = append(keys, "ALL_PROXY", "all_proxy")
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
			return "env:" + key, strings.TrimSpace(value)
		}
	}
	return "none", ""
}

func parseEnvProxy(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, &TransportConfigError{Message: "invalid proxy environment variable"}
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, &TransportConfigError{Message: "proxy environment variable must use http or https"}
	}
	if u.Port() == "0" || u.Hostname() == "" {
		return nil, &TransportConfigError{Message: "invalid proxy environment variable host or port"}
	}
	return u, nil
}

func splitEnvNoProxy() []string {
	values := []string{}
	for _, key := range []string{"NO_PROXY", "no_proxy"} {
		if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
			values = append(values, strings.Split(value, ",")...)
		}
	}
	return values
}

func bypassProxy(target *url.URL, hosts []string) bool {
	if target == nil {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(target.Hostname()))
	if host == "" {
		return false
	}
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	for _, raw := range hosts {
		pattern := strings.ToLower(strings.TrimSpace(raw))
		if pattern == "" {
			continue
		}
		pattern = strings.TrimPrefix(pattern, "http://")
		pattern = strings.TrimPrefix(pattern, "https://")
		if strings.Contains(pattern, ":") {
			patternHost, _, err := net.SplitHostPort(pattern)
			if err == nil {
				pattern = patternHost
			}
		}
		switch {
		case pattern == "*":
			return true
		case strings.HasPrefix(pattern, "*."):
			if strings.HasSuffix(host, strings.TrimPrefix(pattern, "*")) {
				return true
			}
		case strings.HasPrefix(pattern, "."):
			if host == strings.TrimPrefix(pattern, ".") || strings.HasSuffix(host, pattern) {
				return true
			}
		case host == pattern:
			return true
		}
	}
	return false
}

func ProxyDiagnosticText(target string, diag ProxyDiagnostic) string {
	targetURL, _ := url.Parse(strings.TrimSpace(target))
	targetHost := ""
	if targetURL != nil {
		targetHost = targetURL.Hostname()
	}
	if !diag.Enabled {
		return fmt.Sprintf("target=%s proxy=%s transport=%s", targetHost, diag.Source, diag.TransportMode)
	}
	return fmt.Sprintf("target=%s proxy=%s://%s:%s transport=%s", targetHost, diag.Source, diag.Host, diag.Port, diag.TransportMode)
}
