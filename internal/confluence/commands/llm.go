package commands

import (
	"strings"

	"engineering-flow-platform-tools/internal/app"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func commandsCmd() *cobra.Command {
	return &cobra.Command{Use: "commands", RunE: func(cmd *cobra.Command, args []string) error {
		return output.Print(cmd.OutOrStdout(), "json", output.Success("", map[string]any{"commands": app.ConfluenceCommandList()}))
	}}
}
func schemaCmd() *cobra.Command {
	return &cobra.Command{Use: "schema <command>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		required := map[string][]string{"page.create": {"space", "title", "body"}, "page.update": {"id|url"}, "content.create": {"type", "title", "body"}, "content.update": {"content-id"}, "blog.create": {"space", "title", "body"}, "blog.update": {"blog-id-or-url"}}
		meta := map[string]any{"usage": args[0], "risk": "read", "examples": []string{args[0] + " --json"}}
		if strings.Contains(args[0], "delete") || strings.Contains(args[0], "update") || strings.Contains(args[0], "create") {
			meta["risk"] = "write"
		}
		return output.Print(cmd.OutOrStdout(), "json", output.Success("", map[string]any{"command": args[0], "required": required[args[0]], "available": app.ConfluenceCommandList(), "metadata": meta}))
	}}
}
