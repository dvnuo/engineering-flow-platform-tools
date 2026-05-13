package main

import (
	"os"

	"engineering-flow-platform-tools/internal/app"
)

func main() {
	cmd := app.NewJiraRootCommand(app.JiraCommandList())
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
