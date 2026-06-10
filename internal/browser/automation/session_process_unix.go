//go:build !windows

package automation

import (
	"os"
	"os/exec"
	"syscall"
)

func startBrowserProcess(browserPath string, args []string, devNull *os.File) (*exec.Cmd, error) {
	cmd := newBrowserCommand(browserPath, args, devNull)
	configureUnixBrowserCommand(cmd)
	return cmd, cmd.Start()
}

func configureUnixBrowserCommand(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}
