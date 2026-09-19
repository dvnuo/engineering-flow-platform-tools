package main

import (
	"engineering-flow-platform-tools/internal/clihelp"
	"engineering-flow-platform-tools/internal/splunk/commands"
	"os"
)

func main() {
	os.Exit(clihelp.Execute(commands.NewRoot(), "splunk", os.Args[1:], os.Stdout, os.Stderr))
}
