package commands

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDarwinBridgeAppleScriptHandsURLToBridgeLaunch(t *testing.T) {
	script := darwinBridgeAppleScript("/Users/me/efp/browser")
	for _, want := range []string{
		"on open location theURL",
		`quoted form of "/Users/me/efp/browser" & " bridge-launch " & quoted form of theURL`,
		"end open location",
		"on run",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("applescript missing %q:\n%s", want, script)
		}
	}
	escaped := darwinBridgeAppleScript(`/Users/me/"odd"/browser`)
	if !strings.Contains(escaped, `\"odd\"`) {
		t.Fatalf("executable quotes were not escaped:\n%s", escaped)
	}
}

func TestDarwinRegisterStepsDeclareSchemeAndRegister(t *testing.T) {
	app := "/Users/me/Applications/EFP Bridge.app"
	steps := darwinRegisterSteps(app, "/tmp/launcher.applescript")
	if steps[0].Command != "osacompile" || steps[0].Args[0] != "-o" || steps[0].Args[1] != app || steps[0].Args[2] != "/tmp/launcher.applescript" {
		t.Fatalf("first step must compile the applet: %+v", steps[0])
	}
	plist := filepath.Join(app, "Contents", "Info.plist")
	var sawScheme, sawBundleID bool
	for _, step := range steps[1 : len(steps)-1] {
		if step.Command != darwinPlistBuddyPath {
			t.Fatalf("plist edits must go through PlistBuddy: %+v", step)
		}
		if step.Args[len(step.Args)-1] != plist {
			t.Fatalf("plist edit targets the wrong file: %+v", step)
		}
		if strings.Contains(step.Args[1], "CFBundleURLSchemes:0 string efp-bridge") {
			sawScheme = true
		}
		if strings.Contains(step.Args[1], "CFBundleIdentifier string "+darwinBridgeBundleID) {
			sawBundleID = true
		}
	}
	if !sawScheme || !sawBundleID {
		t.Fatalf("scheme=%v bundle id=%v in %+v", sawScheme, sawBundleID, steps)
	}
	last := steps[len(steps)-1]
	if last.Command != darwinLSRegisterPath || last.Args[0] != "-f" || last.Args[1] != app || last.Optional {
		t.Fatalf("last step must register the app with Launch Services: %+v", last)
	}
	if !steps[1].Optional {
		t.Fatalf("deleting a not-yet-existing CFBundleIdentifier must be optional: %+v", steps[1])
	}
	unregister := darwinUnregisterSteps(app)
	if len(unregister) != 1 || unregister[0].Args[0] != "-u" || !unregister[0].Optional {
		t.Fatalf("unregister steps = %+v", unregister)
	}
}

func TestLinuxDesktopEntryOwnsTheScheme(t *testing.T) {
	entry := linuxDesktopEntry("/home/me/efp/browser")
	for _, want := range []string{
		"[Desktop Entry]",
		"Type=Application",
		`Exec="/home/me/efp/browser" bridge-launch %u`,
		"MimeType=x-scheme-handler/efp-bridge;",
		"NoDisplay=true",
	} {
		if !strings.Contains(entry, want) {
			t.Fatalf("desktop entry missing %q:\n%s", want, entry)
		}
	}
	steps := linuxRegisterSteps("/home/me/.local/share/applications")
	if steps[0].Command != "xdg-mime" || strings.Join(steps[0].Args, " ") != "default efp-bridge.desktop x-scheme-handler/efp-bridge" || steps[0].Optional {
		t.Fatalf("xdg-mime step = %+v", steps[0])
	}
	if steps[1].Command != "update-desktop-database" || !steps[1].Optional {
		t.Fatalf("database refresh must be optional: %+v", steps[1])
	}
}
