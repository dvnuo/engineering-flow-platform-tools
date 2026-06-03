package main

import (
	"engineering-flow-platform-tools/internal/clihelp"
	"engineering-flow-platform-tools/internal/jenkins/commands"
	"os"
)

func main() {
	os.Exit(clihelp.Execute(commands.NewRoot(), "jenkins", os.Args[1:], os.Stdout, os.Stderr))
}
