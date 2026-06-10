//go:build !windows

package automation

import "testing"

func TestConfigureUnixBrowserCommandUsesSeparateProcessGroup(t *testing.T) {
	cmd := newBrowserCommand("browser", []string{"--remote-debugging-port=9222"}, nil)
	configureUnixBrowserCommand(cmd)
	if cmd.Stdin != nil {
		t.Fatal("browser command should not inherit stdin")
	}
	if cmd.SysProcAttr == nil {
		t.Fatal("browser command missing SysProcAttr")
	}
	if !cmd.SysProcAttr.Setpgid {
		t.Fatal("browser command should start in a separate process group")
	}
}
