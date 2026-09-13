package commands

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Protocol-handler registration plans for macOS and Linux. They are pure
// functions (no I/O) so the exact command sequence can be unit-tested on any
// platform; serve_other.go executes them, serve_windows.go has the reg.exe
// equivalent.

const (
	bridgeProtocolScheme = "efp-bridge"

	darwinBridgeAppName  = "EFP Bridge.app"
	darwinBridgeBundleID = "com.efp.browser-bridge"
	darwinPlistBuddyPath = "/usr/libexec/PlistBuddy"
	darwinLSRegisterPath = "/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"

	linuxBridgeDesktopFile = "efp-bridge.desktop"
	linuxBridgeMimeType    = "x-scheme-handler/" + bridgeProtocolScheme
)

// protocolStep is one external command of a registration plan. Optional steps
// are attempted but their failure does not abort the plan (for example a
// PlistBuddy Delete of a key that does not exist yet).
type protocolStep struct {
	Name     string
	Command  string
	Args     []string
	Optional bool
}

// darwinBridgeAppleScript is the source of the launcher app. macOS delivers a
// custom-scheme URL to an application as an Apple event, not as a command-line
// argument, so a tiny AppleScript applet receives it and hands it to
// `browser bridge-launch`, which exits immediately after starting the bridge.
func darwinBridgeAppleScript(executable string) string {
	quoted := strings.ReplaceAll(executable, `"`, `\"`)
	return fmt.Sprintf(`on open location theURL
	do shell script quoted form of "%s" & " bridge-launch " & quoted form of theURL & " >/dev/null 2>&1 &"
end open location

on run
	do shell script quoted form of "%s" & " bridge-launch efp-bridge://start >/dev/null 2>&1 &"
end run
`, quoted, quoted)
}

// darwinRegisterSteps compiles the applet, declares the URL scheme in its
// Info.plist, hides it from the Dock, and registers it with Launch Services.
func darwinRegisterSteps(appPath, scriptPath string) []protocolStep {
	plist := filepath.Join(appPath, "Contents", "Info.plist")
	return []protocolStep{
		{Name: "compile launcher app", Command: "osacompile", Args: []string{"-o", appPath, scriptPath}},
		{Name: "reset bundle identifier", Command: darwinPlistBuddyPath, Args: []string{"-c", "Delete :CFBundleIdentifier", plist}, Optional: true},
		{Name: "set bundle identifier", Command: darwinPlistBuddyPath, Args: []string{"-c", "Add :CFBundleIdentifier string " + darwinBridgeBundleID, plist}},
		{Name: "declare url types", Command: darwinPlistBuddyPath, Args: []string{"-c", "Add :CFBundleURLTypes array", plist}},
		{Name: "declare url type entry", Command: darwinPlistBuddyPath, Args: []string{"-c", "Add :CFBundleURLTypes:0 dict", plist}},
		{Name: "name url type", Command: darwinPlistBuddyPath, Args: []string{"-c", "Add :CFBundleURLTypes:0:CFBundleURLName string EFP Bridge", plist}},
		{Name: "declare url schemes", Command: darwinPlistBuddyPath, Args: []string{"-c", "Add :CFBundleURLTypes:0:CFBundleURLSchemes array", plist}},
		{Name: "declare efp-bridge scheme", Command: darwinPlistBuddyPath, Args: []string{"-c", "Add :CFBundleURLTypes:0:CFBundleURLSchemes:0 string " + bridgeProtocolScheme, plist}},
		{Name: "hide from dock", Command: darwinPlistBuddyPath, Args: []string{"-c", "Add :LSUIElement bool true", plist}, Optional: true},
		{Name: "register with launch services", Command: darwinLSRegisterPath, Args: []string{"-f", appPath}},
	}
}

func darwinUnregisterSteps(appPath string) []protocolStep {
	return []protocolStep{
		{Name: "unregister from launch services", Command: darwinLSRegisterPath, Args: []string{"-u", appPath}, Optional: true},
	}
}

// linuxDesktopEntry is the freedesktop entry that owns the scheme; %u carries
// the URL the browser opened.
func linuxDesktopEntry(executable string) string {
	quoted := strings.ReplaceAll(executable, `"`, `\"`)
	return "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=EFP Bridge\n" +
		"Comment=Starts the EFP Portal local browser bridge\n" +
		"NoDisplay=true\n" +
		"Terminal=false\n" +
		fmt.Sprintf("Exec=\"%s\" bridge-launch %%u\n", quoted) +
		"MimeType=" + linuxBridgeMimeType + ";\n"
}

func linuxRegisterSteps(applicationsDir string) []protocolStep {
	return []protocolStep{
		{Name: "set default handler", Command: "xdg-mime", Args: []string{"default", linuxBridgeDesktopFile, linuxBridgeMimeType}},
		{Name: "refresh desktop database", Command: "update-desktop-database", Args: []string{applicationsDir}, Optional: true},
	}
}

func linuxUnregisterSteps(applicationsDir string) []protocolStep {
	return []protocolStep{
		{Name: "refresh desktop database", Command: "update-desktop-database", Args: []string{applicationsDir}, Optional: true},
	}
}
