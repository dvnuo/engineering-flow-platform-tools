package main

import (
	"engineering-flow-platform-tools/internal/browser/commands"
	"engineering-flow-platform-tools/internal/clihelp"
	"os"
)

func main() {
	os.Exit(clihelp.Execute(commands.NewRoot(), "browser", os.Args[1:], os.Stdout, os.Stderr))
}
