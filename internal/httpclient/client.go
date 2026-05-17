package httpclient

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/auth"
	"engineering-flow-platform-tools/internal/config"
)

type Client struct {
	instance config.InstanceConfig
	http     *http.Client
	headers  map[string]string
}

const maxErrorBodySnippet = 2048

var (
	authCredentialPattern = regexp.MustCompile(`(?i)\b(?:bearer|basic)\s+[A-Za-z0-9._~+/\-=]+`)
	sensitiveFieldPattern = regexp.MustCompile(`(?i)((?:"?(?:password|token|api[_-]?key|authorization)"?)\s*[:=]\s*)("[^"]*"|[^\s,}]+)`)
)

func New(instance config.InstanceConfig) (*Client, error) {
	h, err := auth.AuthHeaders(instance.Auth)
	if err != nil {
		return nil, err
	}
	baseTransport, ok := http.DefaultTransport.(*http.Transport)
	var tr *http.Transport
	if ok && baseTransport != nil {
		tr = baseTransport.Clone()
	} else {
		tr = &http.Transport{}
	}
	tr.Proxy = http.ProxyFromEnvironment
	if tr.TLSClientConfig != nil {
		tr.TLSClientConfig = tr.TLSClientConfig.Clone()
	} else {
		tr.TLSClientConfig = &tls.Config{}
	}
	if instance.VerifySSL != nil && !*instance.VerifySSL {
		tr.TLSClientConfig.InsecureSkipVerify = true
	}
	if strings.TrimSpace(instance.CACert) != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(instance.CACert)) {
			return nil, errors.New("config_error")
		}
		tr.TLSClientConfig.RootCAs = pool
	}
	return &Client{instance: instance, http: &http.Client{Timeout: 30 * time.Second, Transport: tr}, headers: h}, nil
}

func (c *Client) Do(r Request) (*http.Response, error) {
	u, err := c.resolveURL(r.Path)
	if err != nil {
		return nil, err
	}
	var body io.Reader
	req, _ := http.NewRequest(r.Method, u, nil)
	q := req.URL.Query()
	for k, v := range r.Query {
		q.Set(k, v)
	}
	req.URL.RawQuery = q.Encode()
	if r.JSONBody != nil {
		b, _ := json.Marshal(r.JSONBody)
		body = bytes.NewReader(b)
		req.Header.Set("Content-Type", "application/json")
	}
	if r.Multipart != nil {
		buf := &bytes.Buffer{}
		mw := multipart.NewWriter(buf)
		fw, _ := mw.CreateFormFile(r.MultipartField, r.MultipartName)
		_, _ = io.Copy(fw, r.Multipart)
		_ = mw.Close()
		body = buf
		req.Header.Set("Content-Type", mw.FormDataContentType())
	}
	if body != nil {
		req.Body = io.NopCloser(body)
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, &HTTPError{Code: "network_error", Message: "request failed", Hint: err.Error()}
	}
	if resp.StatusCode >= 400 {
		return resp, c.statusError(resp)
	}
	return resp, nil
}

func (c *Client) statusError(resp *http.Response) *HTTPError {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodySnippet+1))
	resp.Body = io.NopCloser(bytes.NewReader(body))
	message := fmt.Sprintf("request failed with HTTP %d", resp.StatusCode)
	if snippet := sanitizeErrorSnippet(body, c.headers); snippet != "" {
		message += ": " + snippet
	}
	return &HTTPError{Code: mapStatus(resp.StatusCode), Message: message, Status: resp.StatusCode}
}

func sanitizeErrorSnippet(body []byte, headers map[string]string) string {
	truncated := len(body) > maxErrorBodySnippet
	if truncated {
		body = body[:maxErrorBodySnippet]
	}
	snippet := strings.TrimSpace(string(body))
	if snippet == "" {
		return ""
	}
	snippet = strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t':
			return ' '
		}
		if r < 32 {
			return -1
		}
		return r
	}, snippet)
	for _, secret := range headers {
		if strings.TrimSpace(secret) != "" {
			snippet = strings.ReplaceAll(snippet, secret, "***REDACTED***")
		}
	}
	snippet = authCredentialPattern.ReplaceAllString(snippet, "***REDACTED***")
	snippet = sensitiveFieldPattern.ReplaceAllString(snippet, `${1}"***REDACTED***"`)
	if truncated {
		snippet += "..."
	}
	return snippet
}

func (c *Client) resolveURL(path string) (string, error) {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		if !urlBelongsToBase(path, c.instance.BaseURL) {
			return "", &HTTPError{Code: "instance_url_mismatch", Message: "off-instance url", Hint: "Use a URL that belongs to the selected instance.", Status: 400}
		}
		return path, nil
	}
	if strings.HasPrefix(path, "/") {
		u := strings.TrimRight(c.instance.BaseURL, "/") + path
		_, err := url.Parse(u)
		return u, err
	}
	base := strings.TrimRight(c.instance.BaseURL, "/") + "/" + strings.Trim(c.instance.RESTPath, "/") + "/" + strings.TrimLeft(path, "/")
	_, err := url.Parse(base)
	return base, err
}

func urlBelongsToBase(raw, base string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	b, err := url.Parse(base)
	if err != nil {
		return false
	}
	if !strings.EqualFold(u.Scheme, b.Scheme) || !strings.EqualFold(u.Host, b.Host) {
		return false
	}
	basePath := "/" + strings.Trim(strings.ToLower(b.Path), "/")
	rawPath := "/" + strings.Trim(strings.ToLower(u.Path), "/")
	if basePath == "/" {
		return true
	}
	return rawPath == basePath || strings.HasPrefix(rawPath, strings.TrimRight(basePath, "/")+"/")
}
