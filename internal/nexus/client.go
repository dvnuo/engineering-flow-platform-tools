// Package nexus holds the Sonatype Nexus Repository 3 helpers shared by the
// nexus CLI commands: instance resolution with anonymous-read support, REST
// API v1 path defaults, continuationToken paging, search filter mapping, and
// asset download helpers. All REST calls are read-only.
package nexus

import (
	"crypto/sha1"
	"encoding/hex"
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

	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/httpclient"
	"engineering-flow-platform-tools/internal/instance"
)

// DefaultRESTPath is the Nexus Repository 3 REST API v1 prefix used when an
// instance leaves rest_path empty.
const DefaultRESTPath = "/service/rest/v1"

// DownloadTimeout bounds one asset download (the whole exchange, including
// streaming the body to disk).
const DownloadTimeout = 10 * time.Minute

// Paging defaults for search and list commands. Nexus returns fixed-size
// pages (up to 100 items for /components and /assets, up to 50 for /search)
// addressed by an opaque continuationToken; the CLI caps the items it returns
// client-side and optionally follows tokens across pages.
const (
	DefaultLimit    = 100
	MaxLimit        = 500
	DefaultMaxPages = 5
)

// Context is one resolved nexus instance plus the HTTP client bound to it.
// Anonymous reports that the instance has no credentials configured, so
// requests carry no Authorization header (Nexus may still allow anonymous
// reads).
type Context struct {
	Cfg       config.RootConfig
	Inst      config.InstanceConfig
	Client    *httpclient.Client
	Anonymous bool
}

// NewContext resolves the nexus instance (explicit name, default instance, or
// the only configured instance) and builds a client for it. rawURL may be an
// absolute URL to route by; the nexus commands normally pass "".
func NewContext(cfg config.RootConfig, explicit, rawURL string) (*Context, error) {
	res, err := instance.Resolve(cfg.Nexus, explicit, rawURL, "nexus")
	if err != nil {
		return nil, err
	}
	inst := WithDefaults(res.Instance)
	client, anonymous, err := NewClient(inst)
	if err != nil {
		return nil, err
	}
	return &Context{Cfg: cfg, Inst: inst, Client: client, Anonymous: anonymous}, nil
}

// NewClient builds the HTTP client for one instance. Instances without any
// auth block get an anonymous client (no Authorization header); the returned
// bool reports that case. A partially filled auth block (for example a
// username without a secret) is a config_error, never silently anonymous.
func NewClient(inst config.InstanceConfig) (*httpclient.Client, bool, error) {
	inst = WithDefaults(inst)
	if !HasAuth(inst.Auth) {
		c, err := httpclient.NewAnonymous(inst)
		return c, true, err
	}
	c, err := httpclient.New(inst)
	return c, false, err
}

// WithDefaults fills the effective rest_path so relative REST paths resolve
// under /service/rest/v1 when the config leaves the field empty.
func WithDefaults(inst config.InstanceConfig) config.InstanceConfig {
	if strings.TrimSpace(inst.RESTPath) == "" {
		inst.RESTPath = DefaultRESTPath
	}
	return inst
}

// HasAuth reports whether any credential field or auth type is configured.
func HasAuth(a config.AuthConfig) bool {
	return strings.TrimSpace(a.Type) != "" || a.Username != "" || a.Password != "" || a.APIKey != "" || a.Token != ""
}

// AuthLabel is the non-secret description of an instance's auth mode.
func AuthLabel(a config.AuthConfig) string {
	if !HasAuth(a) {
		return "anonymous"
	}
	if t := config.NormalizeAuthType(a); t != "" {
		return t
	}
	return "incomplete"
}

// Page is the shape shared by /search, /search/assets, /components, and
// /assets responses.
type Page struct {
	Items             []any  `json:"items"`
	ContinuationToken string `json:"continuationToken"`
}

// DecodePage decodes one paged Nexus response.
func DecodePage(r io.Reader) (Page, error) {
	var p Page
	if err := json.NewDecoder(r).Decode(&p); err != nil {
		return Page{}, fmt.Errorf("failed to decode Nexus paged response: %s", httpclient.SanitizeErrorText(err.Error()))
	}
	if p.Items == nil {
		p.Items = []any{}
	}
	return p, nil
}

// JSONValue decodes any JSON document; an empty body decodes to nil.
func JSONValue(r io.Reader) (any, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(b)) == "" {
		return nil, nil
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("failed to decode Nexus JSON response: %s", httpclient.SanitizeErrorText(err.Error()))
	}
	return out, nil
}

// JSONMap decodes a JSON object document.
func JSONMap(r io.Reader) (map[string]any, error) {
	v, err := JSONValue(r)
	if err != nil {
		return nil, err
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("Nexus response is not a JSON object")
	}
	return m, nil
}

// SearchFilters holds the query parameters accepted by /search and
// /search/assets. Empty values are omitted, which Nexus treats as "no filter".
type SearchFilters struct {
	Repository       string
	Format           string
	Group            string
	Name             string
	Version          string
	Keyword          string
	Sort             string
	Direction        string
	MavenGroupID     string
	MavenArtifactID  string
	MavenBaseVersion string
	MavenExtension   string
	MavenClassifier  string
	DockerImageName  string
	DockerImageTag   string
}

// Query maps the filters onto the Nexus search query parameter names.
func (f SearchFilters) Query() map[string]string {
	q := map[string]string{}
	set := func(key, value string) {
		if value = strings.TrimSpace(value); value != "" {
			q[key] = value
		}
	}
	set("repository", f.Repository)
	set("format", f.Format)
	set("group", f.Group)
	set("name", f.Name)
	set("version", f.Version)
	set("q", f.Keyword)
	set("sort", f.Sort)
	set("direction", f.Direction)
	set("maven.groupId", f.MavenGroupID)
	set("maven.artifactId", f.MavenArtifactID)
	set("maven.baseVersion", f.MavenBaseVersion)
	set("maven.extension", f.MavenExtension)
	set("maven.classifier", f.MavenClassifier)
	set("docker.imageName", f.DockerImageName)
	set("docker.imageTag", f.DockerImageTag)
	return q
}

// PageOptions controls client-side paging over continuationToken lists.
type PageOptions struct {
	Continuation string
	Limit        int
	All          bool
	MaxPages     int
}

// Validate normalizes the paging options and rejects out-of-range values.
func (p *PageOptions) Validate() error {
	if p.Limit == 0 {
		p.Limit = DefaultLimit
	}
	if p.Limit < 1 || p.Limit > MaxLimit {
		return fmt.Errorf("--limit must be between 1 and %d", MaxLimit)
	}
	if p.MaxPages == 0 {
		p.MaxPages = DefaultMaxPages
	}
	if p.MaxPages < 1 {
		return errors.New("--max-pages must be at least 1")
	}
	return nil
}

// PageResult is the envelope data of every paged command. ContinuationToken is
// the token to resume from (empty when the server reported no further page);
// Truncated is true when more results exist beyond Items, either because the
// server has another page or because --limit dropped items from a fetched
// page. Dropped counts the items of the last fetched page that --limit cut
// off: resuming from ContinuationToken skips them, so callers that need a
// gap-free walk should page with the default limit instead of a small one.
type PageResult struct {
	Items             []any  `json:"items"`
	CountReturned     int    `json:"count_returned"`
	ContinuationToken string `json:"continuation_token"`
	Truncated         bool   `json:"truncated"`
	PagesFetched      int    `json:"pages_fetched"`
	Dropped           int    `json:"dropped"`
}

// FetchPages GETs path with query, following continuationToken across pages
// when opts.All is set, and caps the returned items at opts.Limit.
func FetchPages(client *httpclient.Client, path string, query map[string]string, opts PageOptions) (PageResult, error) {
	out := PageResult{Items: []any{}}
	if err := opts.Validate(); err != nil {
		return out, err
	}
	maxPages := 1
	if opts.All {
		maxPages = opts.MaxPages
	}
	token := strings.TrimSpace(opts.Continuation)
	for page := 0; page < maxPages; page++ {
		q := make(map[string]string, len(query)+1)
		for k, v := range query {
			q[k] = v
		}
		if token != "" {
			q["continuationToken"] = token
		}
		resp, err := client.Do(httpclient.Request{Method: http.MethodGet, Path: path, Query: q})
		if err != nil {
			return out, err
		}
		pg, err := DecodePage(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return out, &httpclient.HTTPError{Code: "server_error", Message: err.Error(), Hint: "Check that base_url and rest_path point at the Nexus REST API v1.", Status: 502}
		}
		out.PagesFetched++
		for i, item := range pg.Items {
			if len(out.Items) >= opts.Limit {
				out.Truncated = true
				out.Dropped = len(pg.Items) - i
				break
			}
			out.Items = append(out.Items, item)
		}
		token = pg.ContinuationToken
		out.ContinuationToken = token
		if token == "" || len(out.Items) >= opts.Limit {
			break
		}
	}
	out.CountReturned = len(out.Items)
	if out.ContinuationToken != "" {
		out.Truncated = true
	}
	return out, nil
}

// ParseKeyValue parses repeated key=value flags into a query map.
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

// StringField returns m[key] when it is a string.
func StringField(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}

// Checksum returns asset.checksum[algorithm] when present.
func Checksum(asset map[string]any, algorithm string) string {
	sums, _ := asset["checksum"].(map[string]any)
	return StringField(sums, algorithm)
}

// URLBelongsToBase reports whether raw shares scheme, host, and base path with
// the instance base URL. Asset downloadUrl values must pass this guard before
// credentials are sent to them.
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

// DownloadTarget picks the local file for an asset download: the explicit
// --output path, else the asset's file name, else asset.bin.
func DownloadTarget(out, assetPath string) string {
	if strings.TrimSpace(out) != "" {
		return out
	}
	name := path.Base(strings.ReplaceAll(assetPath, "\\", "/"))
	if name == "." || name == "/" || name == "" {
		name = "asset.bin"
	}
	return name
}

// SaveBody streams body to target, creating parent directories, and returns
// the byte count and the SHA-1 of the written content.
func SaveBody(body io.Reader, target string) (int64, string, error) {
	if strings.TrimSpace(target) == "" {
		return 0, "", errors.New("output path is required")
	}
	_ = os.MkdirAll(filepath.Dir(target), 0o700)
	f, err := os.Create(target)
	if err != nil {
		return 0, "", err
	}
	defer f.Close()
	h := sha1.New()
	n, err := io.Copy(io.MultiWriter(f, h), body)
	if err != nil {
		return n, "", err
	}
	return n, hex.EncodeToString(h.Sum(nil)), nil
}

// ResponseName returns the Content-Disposition file name when present.
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
