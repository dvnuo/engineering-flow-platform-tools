package commands

import (
	"fmt"
	"strings"

	"engineering-flow-platform-tools/internal/catalog"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func helpLLMCmd(o *Opts) *cobra.Command {
	return &cobra.Command{
		Use:   "help llm",
		Short: "Show visual CLI usage guidance for LLM agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			tips := visualLLMTips()
			if fmtOut(o) == "json" {
				return print(cmd, o, output.Success("", map[string]any{
					"tips":     tips,
					"commands": catalog.CommandsFromCobra("visual", cmd.Root()),
				}))
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), visualLLMMarkdown(tips))
			return err
		},
	}
}

func visualLLMTips() []string {
	return []string{
		"visual is a terminal-invoked CLI binary, not a browser UI, Portal tool, MCP tool, or HTTP server.",
		"Always use --json for agent workflows so success and failure use the stable ok/data/error envelope.",
		"Start with visual commands --json and visual schema render --json.",
		"Prefer Mermaid .mmd input for user-authored diagrams; pure official Mermaid is accepted and inferred to the closest template.",
		"Use EFP frontmatter only when you need higher-quality layout, camera, route, or renderHints control.",
		"Inspect templates with visual template list and visual template get before rendering JSON or when choosing an explicit Mermaid target.",
		"Use --template-dir when templates live in the current workspace or release artifact.",
		"Generate JSON only for compatibility or internal IR workflows; it must match the selected template input_schema_kind.",
		"Render to a new workspace output directory and return data.artifact.entrypoint to the user.",
		"Use --dry-run to preview planned files without creating the output directory.",
		"Do not generate JavaScript code for visual; data goes into data.js as JSON.",
		"Do not use remote assets, CDN URLs, Node/npm, fetch, or any network dependency.",
		"Generated artifacts are safe to open through file:// and through a Portal/runtime static proxy at any subpath because resource links are relative.",
	}
}

func visualLLMMarkdown(tips []string) string {
	var b strings.Builder
	b.WriteString("# visual CLI usage for agents\n\n")
	for _, tip := range tips {
		b.WriteString("- ")
		b.WriteString(tip)
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
