package main

import (
	"engineering-flow-platform-tools/internal/clihelp"
	"engineering-flow-platform-tools/internal/nexus/commands"
	"os"
)

func main() {
	os.Exit(clihelp.Execute(commands.NewRoot(), "nexus", os.Args[1:], os.Stdout, os.Stderr))
}
