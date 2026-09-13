//go:build !windows

package commands

import (
	"os"
	"os/exec"
	"syscall"

	"engineering-flow-platform-tools/internal/browser/automation"
)

func unsupportedProtocolPlatform() error {
	return automation.NewError("unsupported_platform", "The efp-bridge:// protocol handler can only be registered on Windows.", "Start the bridge manually with browser serve --origin <portal-origin>.", 400)
}

func registerBridgeProtocol(origin string) (map[string]any, error) {
	return nil, unsupportedProtocolPlatform()
}

func unregisterBridgeProtocol() (map[string]any, error) {
	return nil, unsupportedProtocolPlatform()
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
