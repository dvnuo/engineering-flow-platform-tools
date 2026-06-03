package main

import (
	"engineering-flow-platform-tools/internal/clihelp"
	"engineering-flow-platform-tools/internal/visual/commands"
	"os"
)

func main() {
	os.Exit(clihelp.Execute(commands.NewRoot(), "visual", os.Args[1:], os.Stdout, os.Stderr))
}
