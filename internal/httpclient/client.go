package httpclient

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
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

func New(instance config.InstanceConfig) (*Client, error) {
	h, err := auth.AuthHeaders(instance.Auth)
	if err != nil {
		return nil, err
	}
	tr := &http.Transport{TLSClientConfig: &tls.Config{}}
	if instance.VerifySSL != nil && !*instance.VerifySSL {
		tr.TLSClientConfig.InsecureSkipVerify = true
	}
	if instance.CACert != "" {
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
		return resp, &HTTPError{Code: mapStatus(resp.StatusCode), Message: "request failed", Status: resp.StatusCode}
	}
	return resp, nil
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
