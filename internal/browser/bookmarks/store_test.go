package bookmarks

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreAddUpdateRemove(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bookmarks.yaml")
	store := Store{Path: path, Source: "personal"}

	added, err := store.Add(Bookmark{
		Name: "Google", Aliases: []string{"谷歌"}, Description: "Search the public web.", URL: "https://www.google.com/",
	})
	if err != nil {
		t.Fatal(err)
	}
	if added.Source != "personal" {
		t.Fatalf("source = %q", added.Source)
	}
	aliases := []string{"web search"}
	description := "Search public websites."
	updated, err := store.Update("google", Update{Aliases: &aliases, Description: &description})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Description != description || len(updated.Aliases) != 1 || updated.Aliases[0] != "web search" {
		t.Fatalf("updated = %#v", updated)
	}

	reloaded, exists, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !exists || len(reloaded) != 1 || reloaded[0].Description != description {
		t.Fatalf("reloaded = %#v exists=%v", reloaded, exists)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) == "" || strings.Contains(string(body), "source:") {
		t.Fatalf("local manifest must not persist output provenance: %s", body)
	}

	removed, err := store.Remove("GOOGLE")
	if err != nil {
		t.Fatal(err)
	}
	if removed.Name != "Google" {
		t.Fatalf("removed = %#v", removed)
	}
	reloaded, exists, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !exists || len(reloaded) != 0 {
		t.Fatalf("local file should remain as an empty authoritative manifest: %#v exists=%v", reloaded, exists)
	}
}

func TestStoreRejectsDuplicateAndInvalidBookmarks(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "bookmarks.yaml"), Source: "personal"}
	input := Bookmark{Name: "Docs", Description: "Read docs.", URL: "https://docs.example.test/"}
	if _, err := store.Add(input); err != nil {
		t.Fatal(err)
	}
	_, err := store.Add(Bookmark{Name: "docs", Description: "Duplicate.", URL: "https://other.example.test/"})
	assertBookmarkErrorCode(t, err, "bookmark_exists")
	_, err = store.Add(Bookmark{Name: "Bad", Description: "", URL: "file:///tmp/docs"})
	assertBookmarkErrorCode(t, err, "bookmark_invalid")
}

func TestStoreMissingAndMalformedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bookmarks.yaml")
	store := Store{Path: path, Source: "personal"}
	items, exists, err := store.Load()
	if err != nil || exists || len(items) != 0 {
		t.Fatalf("missing load = %#v exists=%v err=%v", items, exists, err)
	}
	if err := os.WriteFile(path, []byte("version: 1\nunknown: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, exists, err = store.Load()
	if !exists {
		t.Fatal("malformed file should still be reported as existing")
	}
	assertBookmarkErrorCode(t, err, "bookmark_store_invalid")
}

func TestStoreReplacesFileWithoutLeavingTemporaryFiles(t *testing.T) {
	dir := t.TempDir()
	store := Store{Path: filepath.Join(dir, "bookmarks.yaml"), Source: "personal"}
	if _, err := store.Add(Bookmark{Name: "One", Description: "First.", URL: "https://one.example.test/"}); err != nil {
		t.Fatal(err)
	}
	description := "Updated."
	if _, err := store.Update("One", Update{Description: &description}); err != nil {
		t.Fatal(err)
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".bookmarks-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files left behind: %#v", matches)
	}
}

func TestValidateSourcesAcceptsLocalNameAndDescription(t *testing.T) {
	sources, err := ValidateSources([]Source{{
		Name: "LOCAL", Description: "Personal websites.", URL: "https://bookmarks.example.test/",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if sources[0].Description != "Personal websites." {
		t.Fatalf("sources = %#v", sources)
	}
}

func TestValidateSourcesRejectsLongDescription(t *testing.T) {
	_, err := ValidateSources([]Source{{
		Name: "personal", Description: strings.Repeat("x", maxDescriptionBytes+1), URL: "https://bookmarks.example.test/",
	}})
	assertBookmarkErrorCode(t, err, "bookmark_config_invalid")
}

func assertBookmarkErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	var bookmarkErr *Error
	if !errors.As(err, &bookmarkErr) || bookmarkErr.Code != code {
		t.Fatalf("err = %#v, want code %q", err, code)
	}
}
