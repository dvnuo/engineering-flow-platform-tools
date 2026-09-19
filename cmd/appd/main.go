package main

import (
	"engineering-flow-platform-tools/internal/appd/commands"
	"engineering-flow-platform-tools/internal/clihelp"
	"os"
)

func main() {
	os.Exit(clihelp.Execute(commands.NewRoot(), "appd", os.Args[1:], os.Stdout, os.Stderr))
}
