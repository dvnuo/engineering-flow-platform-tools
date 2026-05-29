package commands

import (
	"engineering-flow-platform-tools/internal/catalog"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func commandsCmd() *cobra.Command {
	return &cobra.Command{Use: "commands", RunE: func(cmd *cobra.Command, args []string) error {
		return output.Print(cmd.OutOrStdout(), "json", output.Success("", map[string]any{"commands": catalog.CommandsFromCobra("confluence", cmd.Root())}))
	}}
}
func schemaCmd() *cobra.Command {
	return &cobra.Command{Use: "schema <command>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		schema, ok := catalog.SchemaFromCobra("confluence", args[0], cmd.Root())
		if !ok {
			return output.Print(cmd.OutOrStdout(), "json", output.Failure("not_found", "command not found", "Run confluence commands --json to list command names.", 404))
		}
		return output.Print(cmd.OutOrStdout(), "json", output.Success("", schema))
	}}
}
