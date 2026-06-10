package automation

import (
	"os"
	"os/exec"
)

func newBrowserCommand(browserPath string, args []string, devNull *os.File) *exec.Cmd {
	cmd := exec.Command(browserPath, args...)
	cmd.Stdin = nil
	if devNull != nil {
		cmd.Stdout = devNull
		cmd.Stderr = devNull
	}
	return cmd
}
