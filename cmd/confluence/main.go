package main

import (
	"engineering-flow-platform-tools/internal/clihelp"
	"engineering-flow-platform-tools/internal/confluence/commands"
	"os"
)

func main() {
	os.Exit(clihelp.Execute(commands.NewRoot(), "confluence", os.Args[1:], os.Stdout, os.Stderr))
}
