package app

import (
	"engineering-flow-platform-tools/internal/cli"
	"engineering-flow-platform-tools/internal/llm"
	"github.com/spf13/cobra"
)

func NewJiraRootCommand(commands []string) *cobra.Command {
	r := llm.NewRegistry()
	for _, c := range commands {
		r.Register(llm.CommandMeta{Name: c, Usage: c, Product: "jira", Risk: "read", Description: "spec placeholder"})
	}
	return cli.NewRootCommand(cli.BuilderInput{Use: "jira", Short: "Atlassian Jira CLI", Long: "Jira command line interface for Atlassian.", Registry: r})
}
