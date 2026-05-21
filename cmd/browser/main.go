package main

import (
	"engineering-flow-platform-tools/internal/browser/commands"
	"os"
)

func main() {
	if err := commands.NewRoot().Execute(); err != nil {
		os.Exit(1)
	}
}
