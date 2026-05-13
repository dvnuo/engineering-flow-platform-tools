package app

import (
	"engineering-flow-platform-tools/internal/cli"
	"github.com/spf13/cobra"
)

func NewConfluenceRootCommand(commands []string) *cobra.Command {
	return cli.NewRootCommand(cli.BuilderInput{
		Use:      "confluence",
		Short:    "Atlassian Confluence CLI",
		Long:     "Confluence command line interface for Atlassian.",
		Commands: commands,
		SchemaFor: func(command string) cli.SchemaDoc {
			return cli.SchemaDoc{Command: command, Version: 1, Input: map[string]interface{}{"type": "object"}, Output: map[string]interface{}{"type": "object", "envelope": true}}
		},
	})
}
