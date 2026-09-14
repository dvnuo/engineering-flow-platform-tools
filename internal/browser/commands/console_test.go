package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHideConsoleOnlyForProtocolLaunch(t *testing.T) {
	// The call must be inert for every other command: hiding the console of a
	// normal CLI run would blank the terminal the member is reading the JSON
	// envelope in. On non-Windows it is inert everywhere.
	for _, args := range [][]string{
		nil,
		{},
		{"version"},
		{"serve", "--origin", "https://portal.example.test"},
		{"page", "snapshot"},
		{"--json", BridgeLaunchCommandName},
	} {
		HideConsoleForProtocolLaunch(args)
	}
	HideConsoleForProtocolLaunch([]string{BridgeLaunchCommandName, "efp-bridge://start?origin=https%3A%2F%2Fportal.example.test"})
}

func TestBridgeLaunchCommandNameMatchesTheCobraCommand(t *testing.T) {
	root := NewRootWithRunner(nil)
	for _, c := range root.Commands() {
		if c.Name() == BridgeLaunchCommandName {
			if !c.Hidden {
				t.Fatalf("%s must stay hidden", BridgeLaunchCommandName)
			}
			return
		}
	}
	t.Fatalf("no %s command on the root; the console-hide guard would never fire", BridgeLaunchCommandName)
}

func TestLogBridgeLaunchAppendsToTheBridgeLog(t *testing.T) {
	home := t.TempDir()
	t.Setenv("EFP_BROWSER_HOME", home)

	logBridgeLaunch("started pid %d", 4242)
	logBridgeLaunch("rejected %s", "efp-bridge://nope")

	path := filepath.Join(home, "logs", "bridge-serve.log")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("bridge log was not written: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "bridge-launch: ") || !strings.Contains(text, "started pid 4242") || !strings.Contains(text, "rejected efp-bridge://nope") {
		t.Fatalf("bridge log = %q", text)
	}
	if lines := strings.Count(strings.TrimSpace(text), "\n") + 1; lines != 2 {
		t.Fatalf("expected one line per call, got %d in %q", lines, text)
	}
}

func TestBridgePingOriginReadsTheRunningBridgesOrigin(t *testing.T) {
	if got := bridgePingOrigin(map[string]any{"origin": " https://portal.example.test "}); got != "https://portal.example.test" {
		t.Fatalf("origin = %q", got)
	}
	// A bridge older than this build sends no origin; treating that as unknown
	// keeps the launch working instead of demanding an upgrade first.
	for _, ping := range []map[string]any{{}, {"origin": ""}, {"origin": 42}, nil} {
		if got := bridgePingOrigin(ping); got != "" {
			t.Fatalf("ping %#v gave origin %q; want empty", ping, got)
		}
	}
}
