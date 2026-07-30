package bookmarks

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

var localStoreMu sync.Mutex

type Store struct {
	Path   string
	Source string
}

type Update struct {
	Name        *string
	Aliases     *[]string
	Description *string
	URL         *string
}

func (s Store) Load() ([]Bookmark, bool, error) {
	if strings.TrimSpace(s.Path) == "" {
		return nil, false, storeError("bookmark_store_error", "The bookmark source file path is empty.", 500)
	}
	source := strings.TrimSpace(s.Source)
	if source == "" {
		return nil, false, storeError("bookmark_store_error", "The bookmark source name is empty.", 500)
	}
	file, err := os.Open(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return []Bookmark{}, false, nil
	}
	if err != nil {
		return nil, false, storeError("bookmark_store_error", "The bookmark source file could not be read.", 500)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, true, storeError("bookmark_store_error", "The bookmark source file could not be inspected.", 500)
	}
	if !info.Mode().IsRegular() {
		return nil, true, &Error{
			Code:    "bookmark_store_invalid",
			Message: "The bookmark source must be a regular file.",
			Hint:    "Configure --source to use a regular JSON or YAML bookmark manifest.",
			Status:  400,
		}
	}
	body, err := io.ReadAll(io.LimitReader(file, maxSourceBytes+1))
	if err != nil {
		return nil, true, storeError("bookmark_store_error", "The bookmark source file could not be read.", 500)
	}
	if len(body) > maxSourceBytes {
		return nil, true, &Error{
			Code:    "bookmark_store_invalid",
			Message: "The bookmark source file exceeds 1 MiB.",
			Hint:    "Reduce the configured local manifest to 1 MiB or less.",
			Status:  400,
		}
	}
	items, err := parseManifest(source, body)
	if err != nil {
		return nil, true, &Error{
			Code:    "bookmark_store_invalid",
			Message: "The bookmark source file is invalid: " + err.Error(),
			Hint:    "Fix the configured local source manifest, or remove and add its bookmarks again.",
			Status:  400,
		}
	}
	return items, true, nil
}

func (s Store) Add(input Bookmark) (Bookmark, error) {
	localStoreMu.Lock()
	defer localStoreMu.Unlock()

	items, _, err := s.Load()
	if err != nil {
		return Bookmark{}, err
	}
	candidate, err := normalizeBookmark(strings.TrimSpace(s.Source), manifestBookmark{
		Name: input.Name, Aliases: input.Aliases, Description: input.Description, URL: input.URL,
	}, len(items))
	if err != nil {
		return Bookmark{}, invalidBookmarkInput(err)
	}
	for _, item := range items {
		if strings.EqualFold(item.Name, candidate.Name) {
			return Bookmark{}, &Error{
				Code:    "bookmark_exists",
				Message: "A bookmark with this name already exists in the selected source.",
				Hint:    fmt.Sprintf("Use browser bookmark update <name> --source %q --json to change the existing bookmark.", strings.TrimSpace(s.Source)),
				Status:  409,
			}
		}
	}
	items = append(items, candidate)
	if err := s.write(items); err != nil {
		return Bookmark{}, err
	}
	return candidate, nil
}

func (s Store) Update(currentName string, patch Update) (Bookmark, error) {
	localStoreMu.Lock()
	defer localStoreMu.Unlock()

	items, _, err := s.Load()
	if err != nil {
		return Bookmark{}, err
	}
	index := findBookmark(items, currentName)
	if index < 0 {
		return Bookmark{}, bookmarkNotFound(currentName, strings.TrimSpace(s.Source))
	}
	candidate := items[index]
	if patch.Name != nil {
		candidate.Name = *patch.Name
	}
	if patch.Aliases != nil {
		candidate.Aliases = append([]string(nil), (*patch.Aliases)...)
	}
	if patch.Description != nil {
		candidate.Description = *patch.Description
	}
	if patch.URL != nil {
		candidate.URL = *patch.URL
	}
	candidate, err = normalizeBookmark(strings.TrimSpace(s.Source), manifestBookmark{
		Name: candidate.Name, Aliases: candidate.Aliases, Description: candidate.Description, URL: candidate.URL,
	}, index)
	if err != nil {
		return Bookmark{}, invalidBookmarkInput(err)
	}
	for i, item := range items {
		if i != index && strings.EqualFold(item.Name, candidate.Name) {
			return Bookmark{}, &Error{
				Code:    "bookmark_exists",
				Message: "Another bookmark in the selected source already uses the requested name.",
				Hint:    "Choose a unique bookmark name within the selected source.",
				Status:  409,
			}
		}
	}
	items[index] = candidate
	if err := s.write(items); err != nil {
		return Bookmark{}, err
	}
	return candidate, nil
}

func (s Store) Remove(name string) (Bookmark, error) {
	localStoreMu.Lock()
	defer localStoreMu.Unlock()

	items, _, err := s.Load()
	if err != nil {
		return Bookmark{}, err
	}
	index := findBookmark(items, name)
	if index < 0 {
		return Bookmark{}, bookmarkNotFound(name, strings.TrimSpace(s.Source))
	}
	removed := items[index]
	items = append(items[:index], items[index+1:]...)
	if err := s.write(items); err != nil {
		return Bookmark{}, err
	}
	return removed, nil
}

func (s Store) write(items []Bookmark) error {
	if len(items) > maxBookmarks {
		return &Error{
			Code:    "bookmark_store_full",
			Message: fmt.Sprintf("A bookmark source file supports at most %d bookmarks.", maxBookmarks),
			Hint:    "Remove unused bookmarks or split them across configured source manifests.",
			Status:  409,
		}
	}
	doc := manifest{Version: 1, Bookmarks: make([]manifestBookmark, 0, len(items))}
	for i, item := range items {
		normalized, err := normalizeBookmark(strings.TrimSpace(s.Source), manifestBookmark{
			Name: item.Name, Aliases: item.Aliases, Description: item.Description, URL: item.URL,
		}, i)
		if err != nil {
			return invalidBookmarkInput(err)
		}
		doc.Bookmarks = append(doc.Bookmarks, manifestBookmark{
			Name: normalized.Name, Aliases: normalized.Aliases, Description: normalized.Description, URL: normalized.URL,
		})
	}
	body, err := yaml.Marshal(doc)
	if err != nil {
		return storeError("bookmark_store_error", "The bookmark source file could not be encoded.", 500)
	}
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return storeError("bookmark_store_error", "The bookmark source directory could not be created.", 500)
	}
	tmp, err := os.CreateTemp(dir, ".bookmarks-*.tmp")
	if err != nil {
		return storeError("bookmark_store_error", "A temporary bookmark source file could not be created.", 500)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return storeError("bookmark_store_error", "The bookmark source file permissions could not be set.", 500)
	}
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return storeError("bookmark_store_error", "The bookmark source file could not be written.", 500)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return storeError("bookmark_store_error", "The bookmark source file could not be synchronized.", 500)
	}
	if err := tmp.Close(); err != nil {
		return storeError("bookmark_store_error", "The bookmark source file could not be closed.", 500)
	}
	if err := os.Rename(tmpPath, s.Path); err != nil {
		return storeError("bookmark_store_error", "The bookmark source file could not be replaced.", 500)
	}
	return nil
}

func findBookmark(items []Bookmark, name string) int {
	name = strings.TrimSpace(name)
	for i, item := range items {
		if strings.EqualFold(item.Name, name) {
			return i
		}
	}
	return -1
}

func invalidBookmarkInput(err error) error {
	return &Error{
		Code:    "bookmark_invalid",
		Message: err.Error(),
		Hint:    "Provide a unique name, optional aliases, a required description, and an absolute HTTP or HTTPS URL.",
		Status:  400,
	}
}

func bookmarkNotFound(name, source string) error {
	return &Error{
		Code:    "bookmark_not_found",
		Message: "The bookmark was not found in source " + strings.TrimSpace(source) + ": " + strings.TrimSpace(name),
		Hint:    "Run browser bookmark list --source " + strings.TrimSpace(source) + " --json and choose an existing bookmark.",
		Status:  404,
	}
}

func storeError(code, message string, status int) error {
	return &Error{
		Code:    code,
		Message: message,
		Hint:    "Check the configured source path, directory permissions, and free disk space.",
		Status:  status,
	}
}
