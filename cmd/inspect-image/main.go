package main

import (
	"engineering-flow-platform-tools/internal/clihelp"
	"engineering-flow-platform-tools/internal/inspectimage/commands"
	"os"
)

func main() {
	os.Exit(clihelp.Execute(commands.NewRoot(), "inspect-image", os.Args[1:], os.Stdout, os.Stderr))
}
