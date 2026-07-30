package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"engineering-flow-platform-tools/internal/browser/probe"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type fakeRunner struct {
	result probe.ProbeResult
	err    error
	got    probe.ProbeOptions
	calls  int
}

func (f *fakeRunner) Probe(ctx context.Context, opts probe.ProbeOptions) (probe.ProbeResult, error) {
	f.calls++
	f.got = opts
	return f.result, f.err
}

func TestCommandsJSONIncludesPersistentOpenAndProbe(t *testing.T) {
	out := run(t, &fakeRunner{}, "commands", "--json")
	data := out["data"].(map[string]any)
	commands := data["commands"].([]any)
	found := map[string]bool{}
	positions := map[string]int{}
	for index, item := range commands {
		m := item.(map[string]any)
		for _, name := range []string{"open", "bookmark.list", "probe"} {
			if m["name"] == name || strings.Contains(m["usage"].(string), "browser "+name) {
				found[name] = true
				positions[name] = index
			}
		}
	}
	for _, name := range []string{"open", "bookmark.list", "probe"} {
		if !found[name] {
			t.Fatalf("commands did not contain %s: %#v", name, commands)
		}
	}
	if positions["open"] != 0 || positions["open"] >= positions["bookmark.list"] || positions["bookmark.list"] >= positions["probe"] {
		t.Fatalf("open, bookmark.list, and probe order is wrong: positions=%#v commands=%#v", positions, commands)
	}
}

func TestBookmarkListFetchesConfiguredExternalManifest(t *testing.T) {
	setBookmarkTestHome(t)
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"version":1,"bookmarks":[{"name":"Google","aliases":["谷歌"],"description":"Search the public web.","url":"https://www.google.com/"}]}`))
	}))
	defer server.Close()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configBody := "browser:\n  bookmarks:\n    sources:\n      - name: public\n        url: " + server.URL + "\n"
	if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
		t.Fatal(err)
	}

	out := run(t, &fakeRunner{}, "bookmark", "list", "--config", configPath, "--json")
	if out["ok"] != true {
		t.Fatalf("bookmark list failed: %#v", out)
	}
	data := out["data"].(map[string]any)
	items := data["bookmarks"].([]any)
	if len(items) != 1 {
		t.Fatalf("bookmarks = %#v", items)
	}
	item := items[0].(map[string]any)
	if item["name"] != "Google" || item["description"] != "Search the public web." || item["url"] != "https://www.google.com/" {
		t.Fatalf("bookmark metadata missing: %#v", item)
	}
	if calls != 1 {
		t.Fatalf("source calls = %d want 1", calls)
	}
}

func TestBookmarkSourceAddLocalFileAndListBookmarks(t *testing.T) {
	setBookmarkTestHome(t)
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	manifestPath := filepath.Join(dir, "team-bookmarks.yaml")
	manifest := "version: 1\nbookmarks:\n  - name: Runbooks\n    description: Read team runbooks.\n    url: https://runbooks.example.test/\n"
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	added := run(t, &fakeRunner{},
		"bookmark", "source", "add",
		"--name", "team",
		"--description", "Engineering runbooks.",
		"--url", manifestPath,
		"--config", configPath,
		"--json",
	)
	if added["ok"] != true {
		t.Fatalf("local source add failed: %#v", added)
	}
	source := added["data"].(map[string]any)["source"].(map[string]any)
	if source["url"] != manifestPath || source["description"] != "Engineering runbooks." {
		t.Fatalf("local source path was not preserved: %#v", source)
	}

	listed := run(t, &fakeRunner{}, "bookmark", "list", "--config", configPath, "--json")
	if listed["ok"] != true {
		t.Fatalf("bookmark list failed: %#v", listed)
	}
	items := listed["data"].(map[string]any)["bookmarks"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["name"] != "Runbooks" || items[0].(map[string]any)["source"] != "team" {
		t.Fatalf("local source bookmarks = %#v", items)
	}
}

func TestBookmarkListFiltersConfiguredSources(t *testing.T) {
	setBookmarkTestHome(t)
	dir := t.TempDir()
	firstPath := filepath.Join(dir, "first.yaml")
	secondPath := filepath.Join(dir, "second.yaml")
	for path, name := range map[string]string{firstPath: "First", secondPath: "Second"} {
		manifest := "version: 1\nbookmarks:\n  - name: " + name + "\n    description: " + name + " website.\n    url: https://" + strings.ToLower(name) + ".example.test/\n"
		if err := os.WriteFile(path, []byte(manifest), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	configPath := filepath.Join(dir, "config.yaml")
	configBody := "browser:\n  bookmarks:\n    sources:\n      - name: first\n        url: " + firstPath + "\n      - name: \" second \"\n        description: Secondary websites.\n        url: " + secondPath + "\n"
	configBody += "      - name: broken\n        url: relative-path.yaml\n"
	if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
		t.Fatal(err)
	}

	filtered := run(t, &fakeRunner{}, "bookmark", "list", "--source", "SECOND", "--config", configPath, "--json")
	if filtered["ok"] != true {
		t.Fatalf("filtered list failed: %#v", filtered)
	}
	data := filtered["data"].(map[string]any)
	items := data["bookmarks"].([]any)
	statuses := data["sources"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["source"] != "second" || len(statuses) != 1 {
		t.Fatalf("filtered result = %#v", data)
	}
	if statuses[0].(map[string]any)["description"] != "Secondary websites." {
		t.Fatalf("source description missing from list status: %#v", statuses)
	}

	both := run(t, &fakeRunner{}, "bookmark", "list", "--source", "second", "--source", "first", "--config", configPath, "--json")
	bothItems := both["data"].(map[string]any)["bookmarks"].([]any)
	if len(bothItems) != 2 || bothItems[0].(map[string]any)["source"] != "first" || bothItems[1].(map[string]any)["source"] != "second" {
		t.Fatalf("repeatable filter did not preserve configured order: %#v", both)
	}

	missing := run(t, &fakeRunner{}, "bookmark", "list", "--source", "missing", "--config", configPath, "--json")
	if missing["ok"] != false || missing["error"].(map[string]any)["code"] != "bookmark_source_not_found" {
		t.Fatalf("unknown source filter = %#v", missing)
	}

	empty := run(t, &fakeRunner{}, "bookmark", "list", "--source", "", "--config", configPath, "--json")
	if empty["ok"] != false || empty["error"].(map[string]any)["code"] != "invalid_args" ||
		empty["error"].(map[string]any)["status"] != float64(400) {
		t.Fatalf("empty source filter = %#v", empty)
	}
}

func TestBookmarkListReturnsEmptyWhenDefaultConfigIsMissing(t *testing.T) {
	setBookmarkTestHome(t)
	out := run(t, &fakeRunner{}, "bookmark", "list", "--json")
	if out["ok"] != true {
		t.Fatalf("missing default config should return an empty list: %#v", out)
	}
	data := out["data"].(map[string]any)
	if len(data["bookmarks"].([]any)) != 0 {
		t.Fatalf("bookmarks = %#v", data["bookmarks"])
	}
}

func TestBookmarkListDoesNotReadLegacyDefaultManifest(t *testing.T) {
	setBookmarkTestHome(t)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	legacyPath := filepath.Join(home, ".efp", "bookmarks.yaml")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o700); err != nil {
		t.Fatal(err)
	}
	manifest := "version: 1\nbookmarks:\n  - name: Legacy\n    description: Must not be loaded implicitly.\n    url: https://legacy.example.test/\n"
	if err := os.WriteFile(legacyPath, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	out := run(t, &fakeRunner{}, "bookmark", "list", "--json")
	if out["ok"] != true {
		t.Fatalf("bookmark list failed: %#v", out)
	}
	if len(out["data"].(map[string]any)["bookmarks"].([]any)) != 0 {
		t.Fatalf("legacy manifest was loaded implicitly: %#v", out)
	}
}

func TestBookmarkCRUDRequiresConfiguredSource(t *testing.T) {
	setBookmarkTestHome(t)
	missing := run(t, &fakeRunner{},
		"bookmark", "add",
		"--name", "Docs",
		"--description", "Read documentation.",
		"--url", "https://docs.example.test/",
		"--json",
	)
	if missing["ok"] != false || missing["error"].(map[string]any)["code"] != "bookmark_source_required" {
		t.Fatalf("missing source = %#v", missing)
	}

	unknown := run(t, &fakeRunner{},
		"bookmark", "add",
		"--source", "personal",
		"--name", "Docs",
		"--description", "Read documentation.",
		"--url", "https://docs.example.test/",
		"--json",
	)
	if unknown["ok"] != false || unknown["error"].(map[string]any)["code"] != "bookmark_source_not_found" {
		t.Fatalf("unregistered source = %#v", unknown)
	}
}

func TestBookmarkListReportsAllSourceFailures(t *testing.T) {
	setBookmarkTestHome(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configBody := "browser:\n  bookmarks:\n    sources:\n      - name: broken\n        url: " + server.URL + "\n"
	if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
		t.Fatal(err)
	}

	out := run(t, &fakeRunner{}, "bookmark", "list", "--config", configPath, "--json")
	if out["ok"] != false {
		t.Fatalf("all failures should fail: %#v", out)
	}
	errObj := out["error"].(map[string]any)
	if errObj["code"] != "bookmark_sources_unavailable" {
		t.Fatalf("error = %#v", errObj)
	}
	data := out["data"].(map[string]any)
	if len(data["warnings"].([]any)) != 1 {
		t.Fatalf("warnings = %#v", data["warnings"])
	}
}

func TestBookmarkLocalCRUDAndMergedList(t *testing.T) {
	setBookmarkTestHome(t)
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	manifestPath := filepath.Join(dir, "browser", "bookmarks", "personal.yaml")
	sourceAdded := run(t, &fakeRunner{},
		"bookmark", "source", "add",
		"--name", "personal",
		"--description", "Personal websites.",
		"--url", manifestPath,
		"--config", configPath,
		"--json",
	)
	if sourceAdded["ok"] != true {
		t.Fatalf("bookmark source add failed: %#v", sourceAdded)
	}
	if _, err := os.Stat(manifestPath); !os.IsNotExist(err) {
		t.Fatalf("source registration unexpectedly created the manifest: %v", err)
	}

	added := run(t, &fakeRunner{},
		"bookmark", "add",
		"--source", "personal",
		"--name", "Google",
		"--alias", "谷歌",
		"--alias", "web search",
		"--description", "Search the public web.",
		"--url", "https://www.google.com/",
		"--config", configPath,
		"--json",
	)
	if added["ok"] != true {
		t.Fatalf("bookmark add failed: %#v", added)
	}
	addedBookmark := added["data"].(map[string]any)["bookmark"].(map[string]any)
	if addedBookmark["source"] != "personal" || addedBookmark["description"] != "Search the public web." {
		t.Fatalf("added bookmark = %#v", addedBookmark)
	}
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("bookmark add did not create the configured manifest: %v", err)
	}

	listed := run(t, &fakeRunner{}, "bookmark", "list", "--source", "PERSONAL", "--config", configPath, "--json")
	if listed["ok"] != true {
		t.Fatalf("bookmark list failed: %#v", listed)
	}
	listData := listed["data"].(map[string]any)
	items := listData["bookmarks"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["source"] != "personal" {
		t.Fatalf("local bookmark not merged: %#v", listData)
	}

	updated := run(t, &fakeRunner{},
		"bookmark", "update", "google",
		"--source", "personal",
		"--description", "Search public websites.",
		"--clear-aliases",
		"--config", configPath,
		"--json",
	)
	if updated["ok"] != true {
		t.Fatalf("bookmark update failed: %#v", updated)
	}
	updatedBookmark := updated["data"].(map[string]any)["bookmark"].(map[string]any)
	if updatedBookmark["description"] != "Search public websites." {
		t.Fatalf("updated bookmark = %#v", updatedBookmark)
	}
	if aliases, ok := updatedBookmark["aliases"]; ok && len(aliases.([]any)) != 0 {
		t.Fatalf("aliases were not cleared: %#v", updatedBookmark)
	}

	unconfirmed := run(t, &fakeRunner{}, "bookmark", "remove", "Google", "--source", "personal", "--config", configPath, "--json")
	if unconfirmed["ok"] != false || unconfirmed["error"].(map[string]any)["code"] != "invalid_args" {
		t.Fatalf("unconfirmed remove = %#v", unconfirmed)
	}
	removed := run(t, &fakeRunner{}, "bookmark", "remove", "Google", "--source", "personal", "--yes", "--config", configPath, "--json")
	if removed["ok"] != true {
		t.Fatalf("bookmark remove failed: %#v", removed)
	}
}

func TestBookmarkExternalSourcesAreReadOnly(t *testing.T) {
	setBookmarkTestHome(t)
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configBody := "browser:\n  bookmarks:\n    sources:\n      - name: company\n        url: https://bookmarks.example.test/company.yaml\n"
	if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
		t.Fatal(err)
	}
	out := run(t, &fakeRunner{},
		"bookmark", "update", "Google",
		"--source", "company",
		"--description", "Changed.",
		"--config", configPath,
		"--json",
	)
	if out["ok"] != false || out["error"].(map[string]any)["code"] != "bookmark_source_read_only" {
		t.Fatalf("external update = %#v", out)
	}
}

func TestBookmarkListKeepsLocalResultsWhenExternalSourcesFail(t *testing.T) {
	setBookmarkTestHome(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	manifestPath := filepath.Join(dir, "personal.yaml")
	configBody := "browser:\n  bookmarks:\n    sources:\n      - name: personal\n        url: " + manifestPath + "\n      - name: broken\n        url: " + server.URL + "\n"
	if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
		t.Fatal(err)
	}
	added := run(t, &fakeRunner{},
		"bookmark", "add",
		"--source", "personal",
		"--name", "Local docs",
		"--description", "Read local team documentation.",
		"--url", "https://docs.example.test/",
		"--config", configPath,
		"--json",
	)
	if added["ok"] != true {
		t.Fatalf("bookmark add failed: %#v", added)
	}

	out := run(t, &fakeRunner{}, "bookmark", "list", "--config", configPath, "--json")
	if out["ok"] != true {
		t.Fatalf("valid local source should keep the merged list usable: %#v", out)
	}
	data := out["data"].(map[string]any)
	items := data["bookmarks"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["source"] != "personal" {
		t.Fatalf("local bookmarks = %#v", items)
	}
	if len(data["warnings"].([]any)) != 1 {
		t.Fatalf("external warning missing: %#v", data)
	}
}

func TestBookmarkSourceCRUD(t *testing.T) {
	setBookmarkTestHome(t)
	configPath := filepath.Join(t.TempDir(), "config.yaml")

	added := run(t, &fakeRunner{},
		"bookmark", "source", "add",
		"--name", "company",
		"--description", "Company websites.",
		"--url", "https://bookmarks.example.test/company.yaml",
		"--config", configPath,
		"--json",
	)
	if added["ok"] != true {
		t.Fatalf("source add failed: %#v", added)
	}
	listed := run(t, &fakeRunner{}, "bookmark", "source", "list", "--config", configPath, "--json")
	if listed["ok"] != true {
		t.Fatalf("source list failed: %#v", listed)
	}
	sources := listed["data"].(map[string]any)["sources"].([]any)
	if len(sources) != 1 || sources[0].(map[string]any)["name"] != "company" ||
		sources[0].(map[string]any)["description"] != "Company websites." {
		t.Fatalf("sources = %#v", sources)
	}

	updated := run(t, &fakeRunner{},
		"bookmark", "source", "update", "company",
		"--name", "engineering",
		"--description", "Engineering websites.",
		"--url", "https://bookmarks.example.test/engineering.yaml",
		"--config", configPath,
		"--json",
	)
	if updated["ok"] != true {
		t.Fatalf("source update failed: %#v", updated)
	}
	source := updated["data"].(map[string]any)["source"].(map[string]any)
	if source["name"] != "engineering" || source["description"] != "Engineering websites." ||
		source["url"] != "https://bookmarks.example.test/engineering.yaml" {
		t.Fatalf("updated source = %#v", source)
	}
	cleared := run(t, &fakeRunner{},
		"bookmark", "source", "update", "engineering",
		"--description", "",
		"--config", configPath,
		"--json",
	)
	if cleared["ok"] != true {
		t.Fatalf("source description clear failed: %#v", cleared)
	}
	if _, exists := cleared["data"].(map[string]any)["source"].(map[string]any)["description"]; exists {
		t.Fatalf("source description was not cleared: %#v", cleared)
	}

	unconfirmed := run(t, &fakeRunner{}, "bookmark", "source", "remove", "engineering", "--config", configPath, "--json")
	if unconfirmed["ok"] != false || unconfirmed["error"].(map[string]any)["code"] != "invalid_args" {
		t.Fatalf("unconfirmed source remove = %#v", unconfirmed)
	}
	removed := run(t, &fakeRunner{}, "bookmark", "source", "remove", "engineering", "--yes", "--config", configPath, "--json")
	if removed["ok"] != true {
		t.Fatalf("source remove failed: %#v", removed)
	}
	listed = run(t, &fakeRunner{}, "bookmark", "source", "list", "--config", configPath, "--json")
	if len(listed["data"].(map[string]any)["sources"].([]any)) != 0 {
		t.Fatalf("source was not removed: %#v", listed)
	}
}

func TestBookmarkSourceWriteRefusesEnvironmentManagedConfig(t *testing.T) {
	setBookmarkTestHome(t)
	t.Setenv("EFP_BROWSER_BOOKMARKS_SOURCES_0_NAME", "company")
	t.Setenv("EFP_BROWSER_BOOKMARKS_SOURCES_0_URL", "https://bookmarks.example.test/company.yaml")
	out := run(t, &fakeRunner{},
		"bookmark", "source", "add",
		"--name", "public",
		"--url", "https://bookmarks.example.test/public.yaml",
		"--json",
	)
	if out["ok"] != false || out["error"].(map[string]any)["code"] != "config_env_managed" {
		t.Fatalf("environment-managed source write = %#v", out)
	}
}

func TestSchemaProbeRequiresURL(t *testing.T) {
	out := run(t, &fakeRunner{}, "schema", "probe", "--json")
	data := out["data"].(map[string]any)
	required := data["required"].([]any)
	for _, item := range required {
		if item == "url" {
			return
		}
	}
	t.Fatalf("schema did not require url: %#v", data)
}

func TestSchemaBrowserDefaultsToChrome(t *testing.T) {
	for _, command := range []string{"open", "probe", "session.start"} {
		out := run(t, &fakeRunner{}, "schema", command, "--json")
		data := out["data"].(map[string]any)
		flags := data["flags"].([]any)
		found := false
		for _, raw := range flags {
			flag := raw.(map[string]any)
			if flag["name"] == "browser" {
				found = true
				if flag["default"] != "chrome" {
					t.Fatalf("%s --browser default = %v want chrome", command, flag["default"])
				}
				if description, _ := flag["description"].(string); strings.Contains(description, "REDACTED") {
					t.Fatalf("%s --browser description was unexpectedly redacted: %q", command, description)
				}
			}
		}
		if !found {
			t.Fatalf("%s missing --browser flag in %#v", command, data)
		}
	}
}

func TestSessionStartURLIsDocumentedAsCompatibilityOnly(t *testing.T) {
	out := run(t, &fakeRunner{}, "schema", "session.start", "--json")
	data := out["data"].(map[string]any)
	for _, raw := range data["flags"].([]any) {
		flag := raw.(map[string]any)
		if flag["name"] != "url" {
			continue
		}
		description, _ := flag["description"].(string)
		for _, want := range []string{"Deprecated compatibility", "browser open", "do not use in new workflows"} {
			if !strings.Contains(description, want) {
				t.Fatalf("session.start --url description missing %q: %q", want, description)
			}
		}
		return
	}
	t.Fatal("session.start schema missing compatibility --url flag")
}

func TestSchemaIncludesUploadAndDownloadFlags(t *testing.T) {
	cases := map[string][]string{
		"bookmark.add":           {"source", "name", "alias", "description", "url"},
		"bookmark.update":        {"source", "name", "alias", "clear-aliases", "description", "url"},
		"bookmark.remove":        {"source", "yes"},
		"bookmark.source.add":    {"name", "url"},
		"bookmark.source.update": {"name", "url"},
		"bookmark.source.remove": {"yes"},
		"session.start":          {"download-dir"},
		"page.ax":                {"limit", "include-hidden", "pierce", "session", "target-id", "timeout"},
		"page.find":              {"selector", "role", "name", "text", "label", "placeholder", "near-text", "nth", "limit", "include-hidden", "session", "target-id", "timeout"},
		"page.click":             {"selector", "ref", "yes", "session", "target-id", "timeout"},
		"page.type":              {"selector", "ref", "text", "clear", "session", "target-id", "timeout"},
		"page.select":            {"selector", "ref", "value", "label", "index", "session", "target-id", "timeout"},
		"page.check":             {"selector", "ref", "session", "target-id", "timeout"},
		"page.uncheck":           {"selector", "ref", "session", "target-id", "timeout"},
		"page.press":             {"selector", "ref", "key", "session", "target-id", "timeout"},
		"page.upload":            {"selector", "file", "clear", "session", "target-id", "timeout"},
		"page.console":           {"level", "limit", "session", "target-id", "timeout"},
		"page.errors":            {"limit", "session", "target-id", "timeout"},
		"page.console-clear":     {"session", "target-id", "timeout"},
		"page.extract":           {"selector", "limit", "include-html", "pierce", "max-html-bytes", "session", "target-id", "timeout"},
		"page.outline":           {"limit", "include-hidden", "pierce", "session", "target-id", "timeout"},
		"page.metrics":           {"limit-resources", "filter", "session", "target-id", "timeout"},
		"page.table-export":      {"selector", "out", "format", "limit-rows", "limit-cells", "session", "target-id", "timeout"},
		"page.list-export":       {"selector", "out", "format", "limit-items", "session", "target-id", "timeout"},
		"page.scroll-collect":    {"selector", "item-selector", "out", "format", "limit", "max-scrolls", "scroll-step", "interval-ms", "session", "target-id", "timeout"},
		"page.diff":              {"before", "after", "out", "limit"},
		"assert.visible":         {"selector", "ref", "not", "session", "target-id", "timeout"},
		"assert.text":            {"contains", "selector", "ref", "not", "session", "target-id", "timeout"},
		"assert.url":             {"contains", "not", "session", "target-id", "timeout"},
		"assert.count":           {"selector", "equals", "min", "max", "session", "target-id", "timeout"},
		"workflow.run":           {"file", "dry-run", "session", "target-id", "timeout", "continue-on-error", "var", "report-out", "evidence-dir", "allow-human", "yes"},
		"frame.list":             {"session", "target-id", "timeout"},
		"frame.snapshot":         {"frame-id", "include-html", "max-text-bytes", "max-html-bytes", "session", "target-id", "timeout"},
		"network.start":          {"session", "target-id", "timeout", "limit", "filter", "body", "max-body-bytes"},
		"network.stop":           {"session", "target-id", "timeout"},
		"network.list":           {"session", "target-id", "timeout", "filter", "limit", "method", "status", "body", "max-body-bytes"},
		"network.wait":           {"session", "target-id", "timeout", "url-contains", "method", "status", "limit", "body", "max-body-bytes"},
		"network.export":         {"session", "target-id", "timeout", "out", "format", "filter", "limit"},
		"network.clear":          {"session", "target-id", "timeout"},
		"download.list":          {"session"},
		"download.wait":          {"session", "filename-contains", "timeout"},
	}
	for command, flags := range cases {
		out := run(t, &fakeRunner{}, "schema", command, "--json")
		data := out["data"].(map[string]any)
		have := map[string]bool{}
		for _, raw := range data["flags"].([]any) {
			flag := raw.(map[string]any)
			have[flag["name"].(string)] = true
		}
		for _, flag := range flags {
			if !have[flag] {
				t.Fatalf("%s missing --%s in %#v", command, flag, data)
			}
		}
	}
}

func TestVersionJSON(t *testing.T) {
	out := run(t, &fakeRunner{}, "version", "--json")
	if out["ok"] != true {
		t.Fatalf("version failed: %#v", out)
	}
}

func TestProbeRequiresURL(t *testing.T) {
	out := run(t, &fakeRunner{}, "probe", "--json")
	if out["ok"] != false {
		t.Fatalf("missing url should fail: %#v", out)
	}
	errObj := out["error"].(map[string]any)
	if errObj["code"] != "invalid_args" {
		t.Fatalf("code = %#v", errObj)
	}
}

func TestRequireSelectorRequiresSelector(t *testing.T) {
	fake := &fakeRunner{}
	out := run(t, fake, "probe", "--url", "https://intranet.test", "--require-selector", "--json")
	if out["ok"] != false {
		t.Fatalf("missing selector should fail: %#v", out)
	}
	errObj := out["error"].(map[string]any)
	if errObj["code"] != "invalid_args" {
		t.Fatalf("code = %#v", errObj)
	}
	if fake.calls != 0 {
		t.Fatalf("runner should not be called")
	}
}

func TestProbeUsesRunner(t *testing.T) {
	fake := &fakeRunner{result: probe.ProbeResult{Selector: ".user", SelectorFound: true}}
	out := run(t, fake, "probe", "--url", "https://intranet.test", "--selector", ".user", "--json")
	if out["ok"] != true {
		t.Fatalf("probe failed: %#v", out)
	}
	if fake.calls != 1 || fake.got.URL != "https://intranet.test" || fake.got.Selector != ".user" {
		t.Fatalf("runner not called with flags: calls=%d opts=%#v", fake.calls, fake.got)
	}
	data := out["data"].(map[string]any)
	if data["selector_found"] != true {
		t.Fatalf("selector_found = %#v", data)
	}
}

func TestProbeErrorEnvelope(t *testing.T) {
	fake := &fakeRunner{err: &probe.ProbeError{Code: "browser_not_found", Message: "missing", Hint: "install browser", Status: 404}}
	out := run(t, fake, "probe", "--url", "https://intranet.test", "--json")
	if out["ok"] != false {
		t.Fatalf("probe should fail: %#v", out)
	}
	errObj := out["error"].(map[string]any)
	if errObj["code"] != "browser_not_found" {
		t.Fatalf("code = %#v", errObj)
	}
}

func TestHelpIsAnnotatedForVisibleCommands(t *testing.T) {
	cmd := NewRootWithRunner(&fakeRunner{})
	assertHelpAnnotated(t, cmd)
	help := runText(t, &fakeRunner{}, "probe", "--help")
	for _, want := range []string{"one-shot", "closes", "--url", "CSS selector"} {
		if !strings.Contains(help, want) {
			t.Fatalf("probe help missing %q\n%s", want, help)
		}
	}
	help = runText(t, &fakeRunner{}, "open", "--help")
	for _, want := range []string{"persistent", "manual login", "--url", "--session"} {
		if !strings.Contains(help, want) {
			t.Fatalf("open help missing %q\n%s", want, help)
		}
	}
	help = runText(t, &fakeRunner{}, "session", "start", "--help")
	for _, want := range []string{"lower-level lifecycle", "browser open", "Deprecated compatibility"} {
		if !strings.Contains(help, want) {
			t.Fatalf("session start help missing %q\n%s", want, help)
		}
	}
	rootHelp := runText(t, &fakeRunner{}, "--help")
	if strings.Contains(rootHelp, "session start --name default --url") {
		t.Fatalf("root help must not recommend deprecated session start --url\n%s", rootHelp)
	}
	if !strings.Contains(rootHelp, "session start --name default --browser chrome") {
		t.Fatalf("root help missing lifecycle-only session start example\n%s", rootHelp)
	}
}

func TestHelpLLMIncludesWindowsAndFallbackGuidance(t *testing.T) {
	out := run(t, &fakeRunner{}, "help", "llm", "--json")
	tips := out["data"].(map[string]any)["tips"].([]any)
	joined := ""
	for _, tip := range tips {
		joined += tip.(string) + "\n"
	}
	for _, want := range []string{"default way to use every browser command", "Command parsing failures", "Windows cmd", "where browser", "file-read tool"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("help llm missing %q\n%s", want, joined)
		}
	}
}

func TestHelpLLMExplainsPersistentRoutingAndHumanHandoff(t *testing.T) {
	out := run(t, &fakeRunner{}, "help", "llm", "--json")
	tips := out["data"].(map[string]any)["tips"].([]any)
	joined := ""
	for _, tip := range tips {
		joined += tip.(string) + "\n"
	}
	for _, want := range []string{
		"browser bookmark list --json",
		"name, aliases, and required description",
		"returned URL unchanged",
		"already supplied an explicit URL",
		"browser bookmark add/update/remove",
		"require an explicit source",
		"~/.efp/browser/bookmarks/",
		"not read implicitly",
		"Repeat --source",
		"browser bookmark source list/add/update/remove",
		"HTTP/HTTPS or local file source registrations",
		"does not use or write a cache",
		"Default requests to open",
		"Manual login",
		"must not use browser probe",
		"one-shot diagnostic",
		"closes when the command returns",
		"report the session name",
		"do not stop the session",
		"reacquire state",
		"only recommended user-level page-open entry point",
		"Do not generate browser session start --url",
		"explicitly configure or ensure the browser lifecycle without opening a page",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("help llm missing %q\n%s", want, joined)
		}
	}
}

func TestRequireSelectorFailsWhenRunnerDoesNot(t *testing.T) {
	fake := &fakeRunner{result: probe.ProbeResult{Selector: ".user", SelectorFound: false}}
	out := run(t, fake, "probe", "--url", "https://intranet.test", "--selector", ".user", "--require-selector", "--json")
	if out["ok"] != false {
		t.Fatalf("probe should fail: %#v", out)
	}
	errObj := out["error"].(map[string]any)
	if errObj["code"] != "selector_not_found" {
		t.Fatalf("code = %#v", errObj)
	}
}

func run(t *testing.T, r probe.Runner, args ...string) map[string]any {
	t.Helper()
	cmd := NewRootWithRunner(r)
	var b bytes.Buffer
	cmd.SetOut(&b)
	cmd.SetErr(&b)
	cmd.SetArgs(args)
	err := cmd.Execute()
	var out map[string]any
	if uerr := json.Unmarshal(b.Bytes(), &out); uerr != nil {
		t.Fatalf("invalid json err=%v execErr=%v out=%s", uerr, err, b.String())
	}
	return out
}

func runText(t *testing.T, r probe.Runner, args ...string) string {
	t.Helper()
	cmd := NewRootWithRunner(r)
	var b bytes.Buffer
	cmd.SetOut(&b)
	cmd.SetErr(&b)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v out=%s", err, b.String())
	}
	return b.String()
}

func setBookmarkTestHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("EFP_CONFIG", "")
	t.Setenv("ATLASSIAN_CONFIG", "")
	t.Setenv("EFP_VERSION", "")
}

func assertHelpAnnotated(t *testing.T, cmd *cobra.Command) {
	t.Helper()
	if !cmd.Hidden {
		if strings.TrimSpace(cmd.Short) == "" {
			t.Fatalf("%s missing Short", cmd.CommandPath())
		}
		if strings.TrimSpace(cmd.Long) == "" {
			t.Fatalf("%s missing Long", cmd.CommandPath())
		}
		cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
			if strings.TrimSpace(f.Usage) == "" {
				t.Fatalf("%s flag --%s missing usage", cmd.CommandPath(), f.Name)
			}
		})
		cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
			if strings.TrimSpace(f.Usage) == "" {
				t.Fatalf("%s persistent flag --%s missing usage", cmd.CommandPath(), f.Name)
			}
		})
	}
	for _, child := range cmd.Commands() {
		if child.Hidden {
			continue
		}
		assertHelpAnnotated(t, child)
	}
}
