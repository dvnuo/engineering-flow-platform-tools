package app

import (
	"engineering-flow-platform-tools/internal/cli"
	"engineering-flow-platform-tools/internal/llm"
	"github.com/spf13/cobra"
)

func NewConfluenceRootCommand(commands []string) *cobra.Command {
	r := llm.NewRegistry()
	for _, c := range commands {
		r.Register(llm.CommandMeta{Name: c, Usage: c, Product: "confluence", Risk: "read", Description: "spec placeholder"})
	}
	return cli.NewRootCommand(cli.BuilderInput{Use: "confluence", Short: "Atlassian Confluence CLI", Long: "Confluence command line interface for Atlassian.", Registry: r})
}
