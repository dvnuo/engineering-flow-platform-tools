package main

import (
	"engineering-flow-platform-tools/internal/clihelp"
	"engineering-flow-platform-tools/internal/pgsql/commands"
	"os"
)

func main() {
	os.Exit(clihelp.Execute(commands.NewRoot(), "pgsql", os.Args[1:], os.Stdout, os.Stderr))
}
