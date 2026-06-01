package main

import (
	"engineering-flow-platform-tools/internal/inspectimage/commands"
	"os"
)

func main() {
	os.Exit(commands.Execute(os.Args[1:], os.Stdout, os.Stderr))
}
