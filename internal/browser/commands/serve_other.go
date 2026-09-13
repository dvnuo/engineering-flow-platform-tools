//go:build !windows

package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"engineering-flow-platform-tools/internal/browser/automation"
)

// runProtocolCommand executes one registration step; tests replace it.
var runProtocolCommand = func(command string, args ...string) ([]byte, error) {
	return exec.Command(command, args...).CombinedOutput()
}

func unsupportedProtocolPlatform() error {
	return automation.NewError("unsupported_platform", "The efp-bridge:// protocol handler can be registered on Windows, macOS, and Linux only.", "Start the bridge manually with browser serve --origin <portal-origin>.", 400)
}

func registerBridgeProtocol(origin string) (map[string]any, error) {
	executable, err := bridgeExecutablePath()
	if err != nil {
		return nil, err
	}
	switch runtime.GOOS {
	case "darwin":
		return registerDarwinBridgeProtocol(executable, origin)
	case "linux":
		return registerLinuxBridgeProtocol(executable, origin)
	default:
		return nil, unsupportedProtocolPlatform()
	}
}

func unregisterBridgeProtocol() (map[string]any, error) {
	switch runtime.GOOS {
	case "darwin":
		return unregisterDarwinBridgeProtocol()
	case "linux":
		return unregisterLinuxBridgeProtocol()
	default:
		return nil, unsupportedProtocolPlatform()
	}
}

func runProtocolSteps(steps []protocolStep, code string) error {
	for _, step := range steps {
		out, err := runProtocolCommand(step.Command, step.Args...)
		if err == nil || step.Optional {
			continue
		}
		return automation.NewError(code, step.Name+": "+strings.TrimSpace(string(out)+" "+err.Error()), "Run the command in a normal user shell; registration only writes inside your home directory.", 500)
	}
	return nil
}

// ---- macOS: an AppleScript applet in ~/Applications owns the URL scheme ----

func darwinBridgeAppPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", automation.NewError("automation_failed", err.Error(), "The home directory could not be resolved.", 500)
	}
	return filepath.Join(home, "Applications", darwinBridgeAppName), nil
}

func registerDarwinBridgeProtocol(executable, origin string) (map[string]any, error) {
	appPath, err := darwinBridgeAppPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(appPath), 0o755); err != nil {
		return nil, automation.NewError("protocol_register_failed", err.Error(), "~/Applications could not be created.", 500)
	}
	// Recompile from scratch so a moved binary or an older launcher never lingers.
	_ = os.RemoveAll(appPath)
	scriptDir, err := os.MkdirTemp("", "efp-bridge-launcher-")
	if err != nil {
		return nil, automation.NewError("protocol_register_failed", err.Error(), "A temporary directory could not be created.", 500)
	}
	defer os.RemoveAll(scriptDir)
	scriptPath := filepath.Join(scriptDir, "launcher.applescript")
	if err := os.WriteFile(scriptPath, []byte(darwinBridgeAppleScript(executable)), 0o600); err != nil {
		return nil, automation.NewError("protocol_register_failed", err.Error(), "The launcher script could not be written.", 500)
	}
	if err := runProtocolSteps(darwinRegisterSteps(appPath, scriptPath), "protocol_register_failed"); err != nil {
		return nil, err
	}
	return map[string]any{
		"registered": true,
		"protocol":   bridgeProtocolScheme,
		"app_bundle": appPath,
		"bundle_id":  darwinBridgeBundleID,
		"command":    executable + ` bridge-launch "<url>"`,
		"executable": executable,
		"origin":     origin,
	}, nil
}

func unregisterDarwinBridgeProtocol() (map[string]any, error) {
	appPath, err := darwinBridgeAppPath()
	if err != nil {
		return nil, err
	}
	data := map[string]any{"registered": false, "protocol": bridgeProtocolScheme, "app_bundle": appPath}
	if _, statErr := os.Stat(appPath); statErr != nil {
		data["removed"] = false
		data["message"] = "The efp-bridge protocol handler was not registered for this user."
		return data, nil
	}
	_ = runProtocolSteps(darwinUnregisterSteps(appPath), "protocol_unregister_failed")
	if err := os.RemoveAll(appPath); err != nil {
		return nil, automation.NewError("protocol_unregister_failed", err.Error(), "Delete ~/Applications/EFP Bridge.app manually.", 500)
	}
	data["removed"] = true
	return data, nil
}

// ---- Linux: a freedesktop entry registered through xdg-mime ----------------

func linuxApplicationsDir() (string, error) {
	if base := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); base != "" {
		return filepath.Join(base, "applications"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", automation.NewError("automation_failed", err.Error(), "The home directory could not be resolved.", 500)
	}
	return filepath.Join(home, ".local", "share", "applications"), nil
}

func registerLinuxBridgeProtocol(executable, origin string) (map[string]any, error) {
	applicationsDir, err := linuxApplicationsDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(applicationsDir, 0o755); err != nil {
		return nil, automation.NewError("protocol_register_failed", err.Error(), "The applications directory could not be created.", 500)
	}
	desktopPath := filepath.Join(applicationsDir, linuxBridgeDesktopFile)
	if err := os.WriteFile(desktopPath, []byte(linuxDesktopEntry(executable)), 0o644); err != nil {
		return nil, automation.NewError("protocol_register_failed", err.Error(), "The desktop entry could not be written.", 500)
	}
	if err := runProtocolSteps(linuxRegisterSteps(applicationsDir), "protocol_register_failed"); err != nil {
		return nil, err
	}
	return map[string]any{
		"registered":    true,
		"protocol":      bridgeProtocolScheme,
		"desktop_entry": desktopPath,
		"command":       executable + " bridge-launch %u",
		"executable":    executable,
		"origin":        origin,
	}, nil
}

func unregisterLinuxBridgeProtocol() (map[string]any, error) {
	applicationsDir, err := linuxApplicationsDir()
	if err != nil {
		return nil, err
	}
	desktopPath := filepath.Join(applicationsDir, linuxBridgeDesktopFile)
	data := map[string]any{"registered": false, "protocol": bridgeProtocolScheme, "desktop_entry": desktopPath}
	if _, statErr := os.Stat(desktopPath); statErr != nil {
		data["removed"] = false
		data["message"] = "The efp-bridge protocol handler was not registered for this user."
		return data, nil
	}
	if err := os.Remove(desktopPath); err != nil {
		return nil, automation.NewError("protocol_unregister_failed", err.Error(), "Delete the desktop entry manually.", 500)
	}
	_ = runProtocolSteps(linuxUnregisterSteps(applicationsDir), "protocol_unregister_failed")
	data["removed"] = true
	return data, nil
}

// startDetachedBridgeProcess starts `browser serve` in its own process group
// so it survives the bridge-launch parent exiting.
func startDetachedBridgeProcess(executable string, args []string, logFile *os.File) (*exec.Cmd, error) {
	cmd := newDetachedCommand(executable, args, logFile)
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	return cmd, cmd.Start()
}
