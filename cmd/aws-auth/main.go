package main

import (
	"os"

	"engineering-flow-platform-tools/internal/awsauth/commands"
	"engineering-flow-platform-tools/internal/clihelp"
)

func main() {
	os.Exit(clihelp.Execute(commands.NewRoot(), "aws-auth", os.Args[1:], os.Stdout, os.Stderr))
}
