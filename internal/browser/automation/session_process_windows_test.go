//go:build windows

package automation

import "testing"

func TestConfigureWindowsBrowserCommandDetachesFromAgentProcess(t *testing.T) {
	cmd := newBrowserCommand("browser.exe", []string{"--remote-debugging-port=9222"}, nil)
	configureWindowsBrowserCommand(cmd, true)
	if cmd.Stdin != nil {
		t.Fatal("browser command should not inherit stdin")
	}
	if cmd.SysProcAttr == nil {
		t.Fatal("browser command missing SysProcAttr")
	}
	want := uint32(windowsDetachedProcess | windowsCreateNewProcessGroup | windowsCreateBreakawayJob)
	if got := cmd.SysProcAttr.CreationFlags; got&want != want {
		t.Fatalf("creation flags = %#x want all %#x", got, want)
	}
}

func TestConfigureWindowsBrowserCommandFallbackKeepsBasicDetach(t *testing.T) {
	cmd := newBrowserCommand("browser.exe", nil, nil)
	configureWindowsBrowserCommand(cmd, false)
	want := uint32(windowsDetachedProcess | windowsCreateNewProcessGroup)
	if got := cmd.SysProcAttr.CreationFlags; got&want != want {
		t.Fatalf("creation flags = %#x want all %#x", got, want)
	}
	if got := cmd.SysProcAttr.CreationFlags; got&windowsCreateBreakawayJob != 0 {
		t.Fatalf("fallback creation flags unexpectedly include breakaway: %#x", got)
	}
}
