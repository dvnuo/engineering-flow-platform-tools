//go:build windows

package automation

import (
	"os"
	"os/exec"
	"syscall"
)

const (
	windowsDetachedProcess       = 0x00000008
	windowsCreateNewProcessGroup = 0x00000200
	windowsCreateBreakawayJob    = 0x01000000
)

func startBrowserProcess(browserPath string, args []string, devNull *os.File) (*exec.Cmd, error) {
	cmd := newBrowserCommand(browserPath, args, devNull)
	configureWindowsBrowserCommand(cmd, true)
	if err := cmd.Start(); err != nil {
		fallback := newBrowserCommand(browserPath, args, devNull)
		configureWindowsBrowserCommand(fallback, false)
		if fallbackErr := fallback.Start(); fallbackErr == nil {
			return fallback, nil
		}
		return nil, err
	}
	return cmd, nil
}

func configureWindowsBrowserCommand(cmd *exec.Cmd, breakaway bool) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	flags := uint32(windowsDetachedProcess | windowsCreateNewProcessGroup)
	if breakaway {
		flags |= windowsCreateBreakawayJob
	}
	cmd.SysProcAttr.CreationFlags |= flags
}
