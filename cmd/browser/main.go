package main

import (
	"engineering-flow-platform-tools/internal/browser/commands"
	"engineering-flow-platform-tools/internal/clihelp"
	"os"
)

func main() {
	// Before anything else: the efp-bridge:// protocol handler is started by
	// the shell, which gives this console program a window that would otherwise
	// sit on screen until the launch finishes.
	commands.HideConsoleForProtocolLaunch(os.Args[1:])
	os.Exit(clihelp.Execute(commands.NewRoot(), "browser", os.Args[1:], os.Stdout, os.Stderr))
}
