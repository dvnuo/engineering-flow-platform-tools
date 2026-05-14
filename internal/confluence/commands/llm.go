package commands

import (
	"engineering-flow-platform-tools/internal/catalog"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func commandsCmd() *cobra.Command {
	return &cobra.Command{Use: "commands", RunE: func(cmd *cobra.Command, args []string) error {
		return output.Print(cmd.OutOrStdout(), "json", output.Success("", map[string]any{"commands": catalog.Commands("confluence")}))
	}}
}
func schemaCmd() *cobra.Command {
	return &cobra.Command{Use: "schema <command>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return output.Print(cmd.OutOrStdout(), "json", output.Success("", catalog.Schema("confluence", args[0])))
	}}
}
