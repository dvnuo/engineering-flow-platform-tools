package main

import (
	"os"

	"engineering-flow-platform-tools/internal/clihelp"
	logcommands "engineering-flow-platform-tools/internal/logtool/commands"
)

func main() {
	os.Exit(clihelp.Execute(logcommands.NewRoot(), "log", os.Args[1:], os.Stdout, os.Stderr))
}
