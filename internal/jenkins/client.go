package jenkins

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/auth"
	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/httpclient"
	"engineering-flow-platform-tools/internal/instance"
)

type Context struct {
	Cfg      config.RootConfig
	Inst     config.InstanceConfig
	Client   *Client
	DryRun   bool
	Instance string
}

type Client struct {
	instance  config.InstanceConfig
	http      *http.Client
	headers   map[string]string
	crumb     *Crumb
	crumbMode string
}

type Request struct {
	Method      string
	Path        string
	Query       map[string]string
	Body        io.Reader
	ContentType string
	Headers     map[string]string
	NeedCrumb   bool
}

type Crumb struct {
	RequestField string `json:"crumbRequestField" yaml:"crumbRequestField"`
	Value        string `json:"crumb" yaml:"crumb"`
}

const maxErrorBodySnippet = 2048

var stateChangingMethods = map[string]bool{
	http.MethodPost:   true,
	http.MethodPut:    true,
	http.MethodDelete: true,
	http.MethodPatch:  true,
}

func NewContext(cfg config.RootConfig, explicit, entity string, dry bool) (*Context, error) {
	res, err := instance.Resolve(cfg.Jenkins, explicit, entity, "jenkins")
	if err != nil {
		return nil, err
	}
	cl, err := New(res.Instance)
	if err != nil {
		return nil, err
	}
	return &Context{Cfg: cfg, Inst: res.Instance, Client: cl, DryRun: dry, Instance: res.Instance.Name}, nil
}

func New(instance config.InstanceConfig) (*Client, error) {
	headers, err := auth.AuthHeaders(instance.Auth)
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
	crumbMode := strings.ToLower(strings.TrimSpace(instance.CrumbMode))
	if crumbMode == "" {
		crumbMode = "auto"
	}
	return &Client{instance: instance, http: &http.Client{Timeout: 60 * time.Second, Transport: tr}, headers: headers, crumbMode: crumbMode}, nil
}

func (c *Client) Do(r Request) (*http.Response, error) {
	if r.Method == "" {
		r.Method = http.MethodGet
	}
	method := strings.ToUpper(r.Method)
	needCrumb := r.NeedCrumb || stateChangingMethods[method]
	if needCrumb && c.crumbMode != "never" && !strings.Contains(r.Path, "crumbIssuer/") {
		crumb, err := c.GetCrumb()
		if err != nil {
			return nil, err
		}
		if crumb != nil {
			if r.Headers == nil {
				r.Headers = map[string]string{}
			}
			r.Headers[crumb.RequestField] = crumb.Value
		}
	}
	u, err := c.ResolveURL(r.Path, r.Query)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, u, r.Body)
	if err != nil {
		return nil, &httpclient.HTTPError{Code: "invalid_args", Message: err.Error(), Hint: "Check the Jenkins request path.", Status: 400}
	}
	if r.ContentType != "" {
		req.Header.Set("Content-Type", r.ContentType)
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		message := "request failed"
		if detail := httpclient.SanitizeErrorText(err.Error()); detail != "" {
			message += ": " + detail
		}
		return nil, &httpclient.HTTPError{Code: "network_error", Message: message, Hint: "Check network connectivity, proxy settings, and the selected Jenkins base_url."}
	}
	if resp.StatusCode >= 400 {
		return resp, statusError(resp, c.headers)
	}
	return resp, nil
}

func (c *Client) GetCrumb() (*Crumb, error) {
	if c.crumb != nil {
		return c.crumb, nil
	}
	if c.crumbMode == "never" {
		return nil, nil
	}
	u, err := c.ResolveURL("/crumbIssuer/api/json", nil)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		message := "request failed"
		if detail := httpclient.SanitizeErrorText(err.Error()); detail != "" {
			message += ": " + detail
		}
		return nil, &httpclient.HTTPError{Code: "network_error", Message: message, Hint: "Check network connectivity before fetching Jenkins crumb."}
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound && c.crumbMode == "auto" {
		return nil, nil
	}
	if resp.StatusCode >= 400 {
		return nil, statusError(resp, c.headers)
	}
	var crumb Crumb
	if err := json.NewDecoder(resp.Body).Decode(&crumb); err != nil {
		return nil, &httpclient.HTTPError{Code: "server_error", Message: "failed to decode Jenkins crumb response", Hint: "Check whether crumbIssuer returns JSON.", Status: 500}
	}
	if crumb.RequestField == "" || crumb.Value == "" {
		if c.crumbMode == "auto" {
			return nil, nil
		}
		return nil, &httpclient.HTTPError{Code: "server_error", Message: "Jenkins crumb response did not include crumbRequestField and crumb", Status: 500}
	}
	c.crumb = &crumb
	return &crumb, nil
}

func (c *Client) ResolveURL(rawPath string, query map[string]string) (string, error) {
	var base string
	if strings.HasPrefix(rawPath, "http://") || strings.HasPrefix(rawPath, "https://") {
		if !URLBelongsToBase(rawPath, c.instance.BaseURL) {
			return "", &httpclient.HTTPError{Code: "instance_url_mismatch", Message: "off-instance url", Hint: "Use a URL that belongs to the selected Jenkins instance.", Status: 400}
		}
		base = rawPath
	} else if strings.HasPrefix(rawPath, "/") {
		base = strings.TrimRight(c.instance.BaseURL, "/") + rawPath
	} else {
		base = strings.TrimRight(c.instance.BaseURL, "/") + "/" + strings.TrimLeft(rawPath, "/")
	}
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	q := u.Query()
	for k, v := range query {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func JSONMap(resp io.Reader) map[string]any {
	var out map[string]any
	if err := json.NewDecoder(resp).Decode(&out); err != nil {
		return map[string]any{"raw": ""}
	}
	return out
}

func JSONValue(resp io.Reader) any {
	var out any
	if err := json.NewDecoder(resp).Decode(&out); err != nil {
		return map[string]any{"raw": ""}
	}
	return out
}

func Text(resp io.Reader) (string, error) {
	b, err := io.ReadAll(resp)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func JobPath(job string) string {
	return JobPathFromInput(job)
}

func BuildPath(job, build string) string {
	build = strings.TrimSpace(build)
	if build == "" {
		build = "lastBuild"
	}
	return JobPath(job) + "/" + pathEscapeSegment(build)
}

func ArtifactPath(job, build, artifact string) string {
	return BuildPath(job, build) + "/artifact/" + pathEscapeSlashPath(artifact)
}

func QueueIDFromLocation(location string) string {
	location = strings.TrimRight(strings.TrimSpace(location), "/")
	if location == "" {
		return ""
	}
	parts := strings.Split(location, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] == "" || strings.EqualFold(parts[i], "queue") || strings.EqualFold(parts[i], "item") {
			continue
		}
		return parts[i]
	}
	return ""
}

func DownloadPath(out, artifact string) string {
	if strings.TrimSpace(out) != "" {
		return out
	}
	name := path.Base(strings.ReplaceAll(artifact, "\\", "/"))
	if name == "." || name == "/" || name == "" {
		name = "artifact.bin"
	}
	return name
}

func SaveResponseBody(resp *http.Response, target string) (int64, string, error) {
	if target == "" {
		return 0, "", errors.New("output path is required")
	}
	_ = os.MkdirAll(filepath.Dir(target), 0o700)
	f, err := os.Create(target)
	if err != nil {
		return 0, "", err
	}
	defer f.Close()
	n, err := io.Copy(f, resp.Body)
	if err != nil {
		return n, target, err
	}
	return n, target, nil
}

func ResponseName(resp *http.Response, fallback string) string {
	if resp == nil {
		return fallback
	}
	_, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition"))
	if err == nil {
		if name := strings.TrimSpace(params["filename"]); name != "" {
			return name
		}
	}
	return fallback
}

func FormBody(params map[string]string) (io.Reader, string) {
	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}
	return strings.NewReader(values.Encode()), "application/x-www-form-urlencoded"
}

func ParseKeyValue(items []string) (map[string]string, error) {
	out := map[string]string{}
	for _, item := range items {
		k, v, ok := strings.Cut(item, "=")
		if !ok || strings.TrimSpace(k) == "" {
			return nil, fmt.Errorf("invalid key=value parameter %q", item)
		}
		out[strings.TrimSpace(k)] = v
	}
	return out, nil
}

func QueryMap(items []string) (map[string]string, error) {
	return ParseKeyValue(items)
}

func URLBelongsToBase(raw, base string) bool {
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

func JobPathFromInput(job string) string {
	job = strings.TrimSpace(job)
	if job == "" {
		return "/job/"
	}
	if strings.HasPrefix(job, "http://") || strings.HasPrefix(job, "https://") {
		if u, err := url.Parse(job); err == nil {
			job = u.Path
		}
	}
	parts := splitJobSegments(job)
	if len(parts) == 0 {
		return "/job/" + pathEscapeSegment(strings.Trim(job, "/"))
	}
	encoded := make([]string, 0, len(parts)*2)
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		encoded = append(encoded, "job", pathEscapeSegment(part))
	}
	if len(encoded) == 0 {
		return "/job/"
	}
	return "/" + strings.Join(encoded, "/")
}

func splitJobSegments(job string) []string {
	job = strings.Trim(job, "/")
	if job == "" {
		return nil
	}
	raw := strings.Split(job, "/")
	hasJobMarker := false
	for _, part := range raw {
		if part == "job" {
			hasJobMarker = true
			break
		}
	}
	if hasJobMarker {
		var out []string
		for i := 0; i+1 < len(raw); i++ {
			if raw[i] == "job" {
				out = append(out, raw[i+1])
				i++
			}
		}
		return out
	}
	return raw
}

func pathEscapeSlashPath(v string) string {
	parts := strings.Split(strings.Trim(v, "/"), "/")
	for i := range parts {
		parts[i] = pathEscapeSegment(parts[i])
	}
	return strings.Join(parts, "/")
}

func pathEscapeSegment(v string) string {
	if decoded, err := url.PathUnescape(v); err == nil {
		v = decoded
	}
	return url.PathEscape(v)
}

func statusError(resp *http.Response, headers map[string]string) *httpclient.HTTPError {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodySnippet+1))
	resp.Body = io.NopCloser(bytes.NewReader(body))
	message := fmt.Sprintf("request failed with HTTP %d", resp.StatusCode)
	if snippet := httpclient.SanitizeErrorText(string(body)); snippet != "" {
		message += ": " + snippet
	}
	for _, secret := range headers {
		if strings.TrimSpace(secret) != "" {
			message = strings.ReplaceAll(message, secret, "***REDACTED***")
		}
	}
	return &httpclient.HTTPError{Code: mapStatus(resp.StatusCode), Message: message, Status: resp.StatusCode}
}

func mapStatus(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "auth_failed"
	case http.StatusForbidden:
		return "permission_denied"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusConflict:
		return "invalid_args"
	default:
		if status >= 500 {
			return "server_error"
		}
		return "server_error"
	}
}
