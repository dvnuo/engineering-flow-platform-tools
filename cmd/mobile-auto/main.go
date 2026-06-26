package main

import (
	"engineering-flow-platform-tools/internal/clihelp"
	"engineering-flow-platform-tools/internal/mobile/commands"
	"os"
)

func main() {
	os.Exit(clihelp.Execute(commands.NewRoot(), "mobile-auto", os.Args[1:], os.Stdout, os.Stderr))
}
