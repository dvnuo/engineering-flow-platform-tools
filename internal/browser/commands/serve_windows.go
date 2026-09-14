//go:build windows

package commands

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"engineering-flow-platform-tools/internal/browser/automation"
)

const (
	bridgeProtocolRegistryKey = `HKCU\Software\Classes\efp-bridge`
	bridgeProtocolDisplayName = "URL:EFP Bridge"

	windowsBridgeDetachedProcess       = 0x00000008
	windowsBridgeCreateNewProcessGroup = 0x00000200
	windowsBridgeCreateBreakawayJob    = 0x01000000
)

// runRegistryCommand executes reg.exe; tests replace it to capture arguments.
var runRegistryCommand = func(args ...string) ([]byte, error) {
	return exec.Command("reg.exe", args...).CombinedOutput()
}

func bridgeProtocolCommand(executable string) string {
	return fmt.Sprintf(`"%s" bridge-launch "%%1"`, executable)
}

func bridgeProtocolRegistrySteps(executable string) [][]string {
	return [][]string{
		{"add", bridgeProtocolRegistryKey, "/ve", "/t", "REG_SZ", "/d", bridgeProtocolDisplayName, "/f"},
		{"add", bridgeProtocolRegistryKey, "/v", "URL Protocol", "/t", "REG_SZ", "/d", "", "/f"},
		{"add", bridgeProtocolRegistryKey + `\shell\open\command`, "/ve", "/t", "REG_SZ", "/d", bridgeProtocolCommand(executable), "/f"},
	}
}

// registerBridgeProtocol writes HKCU\Software\Classes\efp-bridge so that an
// efp-bridge://start?... link runs `browser.exe bridge-launch "<url>"`.
func registerBridgeProtocol(origin string) (map[string]any, error) {
	executable, err := bridgeExecutablePath()
	if err != nil {
		return nil, err
	}
	for _, args := range bridgeProtocolRegistrySteps(executable) {
		if out, err := runRegistryCommand(args...); err != nil {
			return nil, automation.NewError("protocol_register_failed", strings.TrimSpace(string(out)+" "+err.Error()), "Run the command in a normal user shell; HKCU registration does not need administrator rights.", 500)
		}
	}
	return map[string]any{
		"registered":   true,
		"protocol":     bridgeProtocolScheme,
		"registry_key": bridgeProtocolRegistryKey,
		"command":      bridgeProtocolCommand(executable),
		"executable":   executable,
		"origin":       origin,
	}, nil
}

// unregisterBridgeProtocol deletes the HKCU protocol key when it exists.
func unregisterBridgeProtocol() (map[string]any, error) {
	data := map[string]any{"registered": false, "protocol": bridgeProtocolScheme, "registry_key": bridgeProtocolRegistryKey}
	if _, err := runRegistryCommand("query", bridgeProtocolRegistryKey); err != nil {
		data["removed"] = false
		data["message"] = "The efp-bridge protocol handler was not registered for this user."
		return data, nil
	}
	if out, err := runRegistryCommand("delete", bridgeProtocolRegistryKey, "/f"); err != nil {
		return nil, automation.NewError("protocol_unregister_failed", strings.TrimSpace(string(out)+" "+err.Error()), "Run the command in a normal user shell; HKCU keys do not need administrator rights.", 500)
	}
	data["removed"] = true
	return data, nil
}

// startDetachedBridgeProcess starts `browser serve` so it outlives the
// short-lived bridge-launch process invoked by the protocol handler. It uses
// the same detach flags as the managed browser launch, with a fallback when
// job breakaway is not permitted.
func startDetachedBridgeProcess(executable string, args []string, logFile *os.File) (*exec.Cmd, error) {
	cmd := newDetachedCommand(executable, args, logFile)
	configureWindowsDetachedCommand(cmd, true)
	if err := cmd.Start(); err != nil {
		fallback := newDetachedCommand(executable, args, logFile)
		configureWindowsDetachedCommand(fallback, false)
		if fallbackErr := fallback.Start(); fallbackErr == nil {
			return fallback, nil
		}
		return nil, err
	}
	return cmd, nil
}

func configureWindowsDetachedCommand(cmd *exec.Cmd, breakaway bool) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	flags := uint32(windowsBridgeDetachedProcess | windowsBridgeCreateNewProcessGroup)
	if breakaway {
		flags |= windowsBridgeCreateBreakawayJob
	}
	cmd.SysProcAttr.CreationFlags |= flags
}
