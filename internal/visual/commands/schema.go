package commands

import (
	"engineering-flow-platform-tools/internal/catalog"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func schemaCmd(o *Opts) *cobra.Command {
	return &cobra.Command{
		Use:   "schema <command>",
		Short: "Show argument and flag schema for a visual command",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			schema, ok := catalog.SchemaFromCobra("visual", args[0], cmd.Root())
			if !ok {
				return print(cmd, o, output.Failure(
					"schema_not_found",
					"visual command schema was not found.",
					"Run visual commands --json and pass a listed command name such as render.",
					404,
				))
			}
			return print(cmd, o, output.Success("", schema))
		},
	}
}
