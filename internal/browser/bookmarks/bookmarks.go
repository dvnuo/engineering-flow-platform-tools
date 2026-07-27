package bookmarks

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	maxSources          = 32
	maxBookmarks        = 1000
	maxAliases          = 20
	maxSourceBytes      = 1 << 20
	maxNameBytes        = 128
	maxAliasBytes       = 128
	maxDescriptionBytes = 1000
)

type Source struct {
	Name string `json:"name" yaml:"name"`
	URL  string `json:"url" yaml:"url"`
}

type Bookmark struct {
	Source      string   `json:"source" yaml:"source"`
	Name        string   `json:"name" yaml:"name"`
	Aliases     []string `json:"aliases,omitempty" yaml:"aliases,omitempty"`
	Description string   `json:"description" yaml:"description"`
	URL         string   `json:"url" yaml:"url"`
}

type SourceStatus struct {
	Name  string `json:"name" yaml:"name"`
	OK    bool   `json:"ok" yaml:"ok"`
	Count int    `json:"count" yaml:"count"`
}

type Warning struct {
	Source  string `json:"source" yaml:"source"`
	Code    string `json:"code" yaml:"code"`
	Message string `json:"message" yaml:"message"`
}

type Result struct {
	Bookmarks []Bookmark     `json:"bookmarks" yaml:"bookmarks"`
	Sources   []SourceStatus `json:"sources" yaml:"sources"`
	Warnings  []Warning      `json:"warnings" yaml:"warnings"`
}

type Error struct {
	Code    string
	Message string
	Hint    string
	Status  int
}

func (e *Error) Error() string {
	return e.Code + ": " + e.Message
}

type manifest struct {
	Version   int                `json:"version" yaml:"version"`
	Bookmarks []manifestBookmark `json:"bookmarks" yaml:"bookmarks"`
}

type manifestBookmark struct {
	Name        string   `json:"name" yaml:"name"`
	Aliases     []string `json:"aliases,omitempty" yaml:"aliases,omitempty"`
	Description string   `json:"description" yaml:"description"`
	URL         string   `json:"url" yaml:"url"`
}

type sourceResult struct {
	bookmarks []Bookmark
	warning   *Warning
}

type Lister struct {
	client *http.Client
}

func NewLister() *Lister {
	return &Lister{client: &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("too many redirects")
			}
			return validateHTTPURL(req.URL)
		},
	}}
}

func NewListerWithClient(client *http.Client) *Lister {
	if client == nil {
		return NewLister()
	}
	return &Lister{client: client}
}

func (l *Lister) List(ctx context.Context, sources []Source) (Result, error) {
	result := Result{
		Bookmarks: []Bookmark{},
		Sources:   []SourceStatus{},
		Warnings:  []Warning{},
	}
	normalized, err := ValidateSources(sources)
	if err != nil {
		return result, err
	}
	if len(normalized) == 0 {
		return result, nil
	}

	results := make([]sourceResult, len(normalized))
	var wg sync.WaitGroup
	for i, source := range normalized {
		wg.Add(1)
		go func(index int, item Source) {
			defer wg.Done()
			bookmarks, warning := l.fetch(ctx, item)
			results[index] = sourceResult{bookmarks: bookmarks, warning: warning}
		}(i, source)
	}
	wg.Wait()

	successes := 0
	for i, fetched := range results {
		status := SourceStatus{Name: normalized[i].Name}
		if fetched.warning != nil {
			result.Warnings = append(result.Warnings, *fetched.warning)
		} else {
			successes++
			status.OK = true
			status.Count = len(fetched.bookmarks)
			result.Bookmarks = append(result.Bookmarks, fetched.bookmarks...)
		}
		result.Sources = append(result.Sources, status)
	}
	if successes == 0 {
		return result, &Error{
			Code:    "bookmark_sources_unavailable",
			Message: "No configured bookmark source could be loaded.",
			Hint:    "Inspect data.warnings, verify the source URLs and manifest format, then retry.",
			Status:  502,
		}
	}
	return result, nil
}

func (l *Lister) fetch(ctx context.Context, source Source) ([]Bookmark, *Warning) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)
	if err != nil {
		return nil, sourceWarning(source.Name, "bookmark_source_request_failed", "The bookmark source request could not be created.")
	}
	req.Header.Set("Accept", "application/json, application/yaml, text/yaml")
	req.Header.Set("User-Agent", "efp-browser-bookmarks/1")
	resp, err := l.client.Do(req)
	if err != nil {
		return nil, sourceWarning(source.Name, "bookmark_source_unavailable", "The bookmark source could not be fetched.")
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, sourceWarning(source.Name, "bookmark_source_http_error", fmt.Sprintf("The bookmark source returned HTTP %d.", resp.StatusCode))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSourceBytes+1))
	if err != nil {
		return nil, sourceWarning(source.Name, "bookmark_source_read_failed", "The bookmark source response could not be read.")
	}
	if len(body) > maxSourceBytes {
		return nil, sourceWarning(source.Name, "bookmark_source_too_large", "The bookmark source response exceeds 1 MiB.")
	}
	items, err := parseManifest(source.Name, body)
	if err != nil {
		return nil, sourceWarning(source.Name, "bookmark_manifest_invalid", err.Error())
	}
	return items, nil
}

func ValidateSources(sources []Source) ([]Source, error) {
	if len(sources) > maxSources {
		return nil, &Error{
			Code:    "bookmark_config_invalid",
			Message: fmt.Sprintf("browser.bookmarks.sources supports at most %d entries.", maxSources),
			Hint:    "Reduce the number of configured external bookmark sources.",
			Status:  400,
		}
	}
	seen := map[string]struct{}{}
	out := make([]Source, 0, len(sources))
	for i, source := range sources {
		source.Name = strings.TrimSpace(source.Name)
		source.URL = strings.TrimSpace(source.URL)
		if source.Name == "" || len(source.Name) > maxNameBytes {
			return nil, invalidSource(i, "name is required and must be at most 128 bytes")
		}
		key := strings.ToLower(source.Name)
		if key == LocalSourceName {
			return nil, invalidSource(i, `name "local" is reserved for the managed local bookmark file`)
		}
		if _, ok := seen[key]; ok {
			return nil, invalidSource(i, "name must be unique (case-insensitive)")
		}
		seen[key] = struct{}{}
		u, err := url.Parse(source.URL)
		if err != nil || validateHTTPURL(u) != nil {
			return nil, invalidSource(i, "url must be an absolute HTTP or HTTPS URL without embedded credentials")
		}
		out = append(out, source)
	}
	return out, nil
}

func invalidSource(index int, message string) error {
	return &Error{
		Code:    "bookmark_config_invalid",
		Message: fmt.Sprintf("browser.bookmarks.sources[%d] %s.", index, message),
		Hint:    "Fix the bookmark source entry in ~/.efp/config.yaml or the file passed with --config.",
		Status:  400,
	}
}

func validateHTTPURL(u *url.URL) error {
	if u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return errors.New("invalid HTTP URL")
	}
	return nil
}

func parseManifest(sourceName string, body []byte) ([]Bookmark, error) {
	var doc manifest
	decoder := yaml.NewDecoder(bytes.NewReader(body))
	decoder.KnownFields(true)
	if err := decoder.Decode(&doc); err != nil {
		return nil, fmt.Errorf("The manifest is not valid JSON/YAML: %v", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("The manifest must contain exactly one document.")
		}
		return nil, fmt.Errorf("The manifest contains trailing invalid data: %v", err)
	}
	if doc.Version != 1 {
		return nil, errors.New("The manifest version must be 1.")
	}
	if len(doc.Bookmarks) > maxBookmarks {
		return nil, fmt.Errorf("The manifest supports at most %d bookmarks.", maxBookmarks)
	}
	seen := map[string]struct{}{}
	out := make([]Bookmark, 0, len(doc.Bookmarks))
	for i, item := range doc.Bookmarks {
		normalized, err := normalizeBookmark(sourceName, item, i)
		if err != nil {
			return nil, err
		}
		key := strings.ToLower(normalized.Name)
		if _, ok := seen[key]; ok {
			return nil, manifestFieldError(i, "name", "must be unique within its source (case-insensitive)")
		}
		seen[key] = struct{}{}
		out = append(out, normalized)
	}
	return out, nil
}

func normalizeBookmark(sourceName string, item manifestBookmark, index int) (Bookmark, error) {
	item.Name = strings.TrimSpace(item.Name)
	item.Description = strings.TrimSpace(item.Description)
	item.URL = strings.TrimSpace(item.URL)
	if item.Name == "" || len(item.Name) > maxNameBytes {
		return Bookmark{}, manifestFieldError(index, "name", "is required and must be at most 128 bytes")
	}
	if item.Description == "" || len(item.Description) > maxDescriptionBytes {
		return Bookmark{}, manifestFieldError(index, "description", "is required and must be at most 1000 bytes")
	}
	u, err := url.Parse(item.URL)
	if err != nil || validateHTTPURL(u) != nil {
		return Bookmark{}, manifestFieldError(index, "url", "must be an absolute HTTP or HTTPS URL without embedded credentials")
	}
	if len(item.Aliases) > maxAliases {
		return Bookmark{}, manifestFieldError(index, "aliases", "supports at most 20 entries")
	}
	aliasSeen := map[string]struct{}{}
	aliases := make([]string, 0, len(item.Aliases))
	for aliasIndex, alias := range item.Aliases {
		alias = strings.TrimSpace(alias)
		if alias == "" || len(alias) > maxAliasBytes {
			return Bookmark{}, manifestFieldError(index, fmt.Sprintf("aliases[%d]", aliasIndex), "must be non-empty and at most 128 bytes")
		}
		aliasKey := strings.ToLower(alias)
		if _, ok := aliasSeen[aliasKey]; ok {
			return Bookmark{}, manifestFieldError(index, "aliases", "must not contain duplicates (case-insensitive)")
		}
		aliasSeen[aliasKey] = struct{}{}
		aliases = append(aliases, alias)
	}
	return Bookmark{
		Source:      sourceName,
		Name:        item.Name,
		Aliases:     aliases,
		Description: item.Description,
		URL:         item.URL,
	}, nil
}

func manifestFieldError(index int, field, message string) error {
	return fmt.Errorf("bookmarks[%d].%s %s.", index, field, message)
}

func sourceWarning(source, code, message string) *Warning {
	const maxWarningRunes = 500
	runes := []rune(message)
	if len(runes) > maxWarningRunes {
		message = string(runes[:maxWarningRunes]) + "..."
	}
	return &Warning{Source: source, Code: code, Message: message}
}
