package commands

import (
	"engineering-flow-platform-tools/internal/catalog"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func commandsCmd(o *Opts) *cobra.Command {
	return &cobra.Command{
		Use:   "commands",
		Short: "List visual commands with metadata",
		RunE: func(cmd *cobra.Command, args []string) error {
			return print(cmd, o, output.Success("", map[string]any{
				"commands": catalog.CommandsFromCobra("visual", cmd.Root()),
			}))
		},
	}
}
