package automation

import (
	"context"
	"testing"
	"time"
)

func (f *fakePersistentSessionManager) ListTabs(_ context.Context, sessionName string) (TabListResult, error) {
	f.listCalls++
	if f.listErr != nil {
		return TabListResult{}, f.listErr
	}
	var tabs []Target
	if len(f.tabLists) > 0 {
		index := f.listCalls - 1
		if index >= len(f.tabLists) {
			index = len(f.tabLists) - 1
		}
		tabs = f.tabLists[index]
	}
	return TabListResult{Session: sessionName, Tabs: tabs}, nil
}

func (f *fakePersistentSessionManager) ActivateTab(_ context.Context, sessionName, targetID string) (TabResult, error) {
	f.activated = append(f.activated, targetID)
	if f.activateErr != nil {
		return TabResult{}, f.activateErr
	}
	for _, tabs := range f.tabLists {
		for _, tab := range tabs {
			if tab.ID == targetID {
				tab.Active = true
				return TabResult{Session: sessionName, Tab: tab}, nil
			}
		}
	}
	return TabResult{}, NewError("target_not_found", "no such tab: "+targetID, "", 404)
}

func shortenEnsureTabWait(t *testing.T) {
	t.Helper()
	wait, poll := ensureTabWait, ensureTabPoll
	ensureTabWait, ensureTabPoll = 300*time.Millisecond, 10*time.Millisecond
	t.Cleanup(func() { ensureTabWait, ensureTabPoll = wait, poll })
}

func TestEnsurePersistentLaunchesOnTheURLAndKeepsThatTabOnly(t *testing.T) {
	fake := &fakePersistentSessionManager{
		ensureSession: Session{Name: "default", Alive: true, BrowserWebSocketURL: "ws://127.0.0.1/devtools/browser/new"},
		tabLists:      [][]Target{{{ID: "page-1", Type: "page", URL: "https://portal.example.test/app"}}},
	}
	result, err := ensurePersistent(context.Background(), fake, StartOptions{Name: "default", URL: "https://portal.example.test/app", Browser: "chrome"})
	if err != nil {
		t.Fatal(err)
	}
	// The launch itself shows the page: no New Tab page and no second tab.
	if fake.ensureOpts.URL != "https://portal.example.test/app" || fake.ensureOpts.Name != "default" || fake.ensureOpts.Browser != "chrome" {
		t.Fatalf("launch options = %#v", fake.ensureOpts)
	}
	if fake.openCalls != 0 || len(fake.activated) != 1 || fake.activated[0] != "page-1" {
		t.Fatalf("open calls=%d activated=%v", fake.openCalls, fake.activated)
	}
	if result.Reused || result.TabOpened || result.Target.ID != "page-1" || !result.Target.Active || result.Session.ActiveTargetID != "page-1" {
		t.Fatalf("result = %#v", result)
	}
}

func TestEnsurePersistentWaitsForTheLaunchTabBeforeOpeningOne(t *testing.T) {
	shortenEnsureTabWait(t)
	fake := &fakePersistentSessionManager{
		ensureSession: Session{Name: "default", Alive: true, BrowserWebSocketURL: "ws://127.0.0.1/devtools/browser/new"},
		// Chrome lists its first tab a moment after DevTools answers, still blank.
		tabLists: [][]Target{{}, {}, {{ID: "page-1", Type: "page", URL: "about:blank"}}},
	}
	result, err := ensurePersistent(context.Background(), fake, StartOptions{Name: "default", URL: "https://portal.example.test/app"})
	if err != nil {
		t.Fatal(err)
	}
	if fake.listCalls < 3 || fake.openCalls != 0 || len(fake.activated) != 1 || fake.activated[0] != "page-1" || result.TabOpened {
		t.Fatalf("list calls=%d open calls=%d activated=%v result=%#v", fake.listCalls, fake.openCalls, fake.activated, result)
	}

	// A transient blank target listed before the launch tab does not win: the
	// wait continues until a tab at the origin shows up.
	fake = &fakePersistentSessionManager{
		ensureSession: Session{Name: "default", Alive: true, BrowserWebSocketURL: "ws://127.0.0.1/devtools/browser/new"},
		tabLists: [][]Target{
			{{ID: "blank", Type: "page", URL: "about:blank"}},
			{{ID: "blank", Type: "page", URL: "about:blank"}, {ID: "page-2", Type: "page", URL: "https://portal.example.test/app"}},
		},
	}
	result, err = ensurePersistent(context.Background(), fake, StartOptions{Name: "default", URL: "https://portal.example.test/app"})
	if err != nil {
		t.Fatal(err)
	}
	if fake.listCalls != 2 || fake.openCalls != 0 || len(fake.activated) != 1 || fake.activated[0] != "page-2" || result.Target.ID != "page-2" {
		t.Fatalf("list calls=%d open calls=%d activated=%v result=%#v", fake.listCalls, fake.openCalls, fake.activated, result)
	}

	// A window that stays empty gets the tab opened once the wait is over.
	fake = &fakePersistentSessionManager{
		ensureSession: Session{Name: "default", Alive: true, BrowserWebSocketURL: "ws://127.0.0.1/devtools/browser/new"},
		tabLists:      [][]Target{{}},
		tabResult:     TabResult{Session: "default", Tab: Target{ID: "page-new", Type: "page", URL: "https://portal.example.test/app", Active: true}},
	}
	result, err = ensurePersistent(context.Background(), fake, StartOptions{Name: "default", URL: "https://portal.example.test/app"})
	if err != nil {
		t.Fatal(err)
	}
	if fake.listCalls < 2 || fake.openCalls != 1 || fake.openURL != "https://portal.example.test/app" || len(fake.activated) != 0 {
		t.Fatalf("list calls=%d open calls=%d url=%q activated=%v", fake.listCalls, fake.openCalls, fake.openURL, fake.activated)
	}
	if !result.TabOpened || result.Target.ID != "page-new" || result.Session.ActiveTargetID != "page-new" {
		t.Fatalf("result = %#v", result)
	}
}

func TestEnsurePersistentReusesTheRunningWindowWithoutAddingTabs(t *testing.T) {
	const portal = "https://portal.example.test/app#/chat"
	tests := []struct {
		name string
		tabs []Target
		want string
	}{
		{name: "front tab at the origin wins", tabs: []Target{{ID: "jira", URL: "https://jira.example.test/browse/X"}, {ID: "docs", URL: "https://portal.example.test/docs"}, {ID: "front", URL: "https://portal.example.test/app?code=REDACTED", Active: true}}, want: "front"},
		{name: "a tab at the origin beats the front tab", tabs: []Target{{ID: "jira", URL: "https://jira.example.test/", Active: true}, {ID: "portal", URL: "https://portal.example.test/other"}}, want: "portal"},
		{name: "origin matching ignores case default port and path", tabs: []Target{{ID: "jira", URL: "https://jira.example.test/"}, {ID: "portal", URL: "HTTPS://Portal.Example.Test:443/x/y?z=1"}}, want: "portal"},
		{name: "front tab mid-redirect to a login provider is kept", tabs: []Target{{ID: "sso", URL: "https://sso.example.test/login?next=portal", Active: true}, {ID: "jira", URL: "https://jira.example.test/"}}, want: "sso"},
		{name: "first tab when none is in front", tabs: []Target{{ID: "jira", URL: "https://jira.example.test/"}, {ID: "wiki", URL: "https://wiki.example.test/"}}, want: "jira"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakePersistentSessionManager{
				ensureSession: Session{Name: "default", Alive: true, BrowserWebSocketURL: "ws://127.0.0.1/devtools/browser/existing", ActiveTargetID: "old"},
				ensureReused:  true,
				tabLists:      [][]Target{tc.tabs},
			}
			result, err := ensurePersistent(context.Background(), fake, StartOptions{Name: "default", URL: portal})
			if err != nil {
				t.Fatal(err)
			}
			if fake.openCalls != 0 || fake.listCalls != 1 || len(fake.activated) != 1 || fake.activated[0] != tc.want {
				t.Fatalf("open calls=%d list calls=%d activated=%v want %s", fake.openCalls, fake.listCalls, fake.activated, tc.want)
			}
			if !result.Reused || result.TabOpened || result.Target.ID != tc.want || result.Session.ActiveTargetID != tc.want {
				t.Fatalf("result = %#v", result)
			}
		})
	}

	// Only a window without page tabs (macOS keeps Chrome running after the
	// last window closes) gets a tab, and a running browser is not waited for.
	fake := &fakePersistentSessionManager{
		ensureSession: Session{Name: "default", Alive: true, BrowserWebSocketURL: "ws://127.0.0.1/devtools/browser/existing"},
		ensureReused:  true,
		tabLists:      [][]Target{{}},
		tabResult:     TabResult{Session: "default", Tab: Target{ID: "page-new", Type: "page", URL: portal, Active: true}},
	}
	result, err := ensurePersistent(context.Background(), fake, StartOptions{Name: "default", URL: portal})
	if err != nil {
		t.Fatal(err)
	}
	if fake.listCalls != 1 || fake.openCalls != 1 || fake.openURL != portal || !result.TabOpened || result.Target.ID != "page-new" {
		t.Fatalf("list calls=%d open calls=%d url=%q result=%#v", fake.listCalls, fake.openCalls, fake.openURL, result)
	}
}

func TestEnsurePersistentWithoutURLOnlyEnsuresTheSession(t *testing.T) {
	fake := &fakePersistentSessionManager{
		ensureSession: Session{Name: "default", Alive: true, BrowserWebSocketURL: "ws://127.0.0.1/devtools/browser/existing"},
		ensureReused:  true,
	}
	result, err := ensurePersistent(context.Background(), fake, StartOptions{Name: "default"})
	if err != nil {
		t.Fatal(err)
	}
	if fake.ensureCalls != 1 || fake.ensureOpts.URL != "" || fake.listCalls != 0 || fake.openCalls != 0 || len(fake.activated) != 0 {
		t.Fatalf("fake = %#v", fake)
	}
	if !result.Reused || result.Target.ID != "" || result.TabOpened {
		t.Fatalf("result = %#v", result)
	}
}

func TestEnsurePersistentRejectsInvalidURLBeforeSessionLookup(t *testing.T) {
	fake := &fakePersistentSessionManager{}
	_, err := ensurePersistent(context.Background(), fake, StartOptions{URL: "file:///tmp/private"})
	if automationErr, ok := err.(*Error); !ok || automationErr.Code != "invalid_args" {
		t.Fatalf("error = %#v", err)
	}
	if fake.ensureCalls != 0 || fake.listCalls != 0 || fake.openCalls != 0 {
		t.Fatalf("manager was called: %#v", fake)
	}
	mgr := NewManager(NewStore(t.TempDir()), nil)
	_, err = mgr.EnsurePersistent(context.Background(), StartOptions{Name: "default", URL: "ftp://portal.example.test"})
	if automationErr, ok := err.(*Error); !ok || automationErr.Code != "invalid_args" {
		t.Fatalf("Manager error = %#v", err)
	}
}

func TestOriginOf(t *testing.T) {
	tests := map[string]string{
		"https://Portal.Example.Test:443/app?x=1#y": "https://portal.example.test",
		"http://localhost:8010/":                    "http://localhost:8010",
		"http://localhost:80/app":                   "http://localhost",
		"https://portal.example.test":               "https://portal.example.test",
		"about:blank":                               "",
		"chrome://newtab/":                          "",
		"":                                          "",
		"portal.example.test/app":                   "",
	}
	for raw, want := range tests {
		if got := originOf(raw); got != want {
			t.Fatalf("originOf(%q) = %q want %q", raw, got, want)
		}
	}
}
