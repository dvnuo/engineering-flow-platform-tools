package clihelp

import (
	"strings"

	"engineering-flow-platform-tools/internal/catalog"
	"engineering-flow-platform-tools/internal/llm"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type ProductHelp struct {
	Product      string
	Binary       string
	Short        string
	Long         string
	Examples     []string
	Instructions string
	Groups       map[string]string
}

func ApplyCatalogHelp(root *cobra.Command, h ProductHelp) {
	if h.Binary == "" {
		h.Binary = h.Product
	}
	if strings.TrimSpace(root.Short) == "" {
		root.Short = h.Short
	}
	if strings.TrimSpace(root.Long) == "" {
		root.Long = h.Long
	}
	if strings.TrimSpace(root.Example) == "" {
		root.Example = strings.Join(h.Examples, "\n")
	}
	describeFlags(root, h.Product, "")
	annotateCommands(root, h)
}

func annotateCommands(cmd *cobra.Command, h ProductHelp) {
	for _, child := range cmd.Commands() {
		if child.Hidden {
			continue
		}
		key := commandKey(child)
		meta, ok := catalog.Find(h.Product, key)
		if ok {
			applyMeta(child, h, meta)
		} else {
			applyGroupFallback(child, h, key)
		}
		describeFlags(child, h.Product, key)
		annotateCommands(child, h)
	}
}

func applyMeta(cmd *cobra.Command, h ProductHelp, meta llm.CommandMeta) {
	if strings.TrimSpace(cmd.Short) == "" {
		cmd.Short = meta.Description
	}
	if strings.TrimSpace(cmd.Long) == "" {
		cmd.Long = commandLong(h, meta)
	}
	if strings.TrimSpace(cmd.Example) == "" {
		cmd.Example = strings.Join(meta.Examples, "\n")
	}
}

func applyGroupFallback(cmd *cobra.Command, h ProductHelp, key string) {
	desc := strings.TrimSpace(h.Groups[key])
	if desc == "" {
		desc = fallbackDescription(h.Product, key, cmd.HasAvailableSubCommands())
	}
	if strings.TrimSpace(cmd.Short) == "" {
		cmd.Short = desc
	}
	if strings.TrimSpace(cmd.Long) == "" {
		cmd.Long = strings.TrimSpace(desc + "\n\nUse `" + commandUsage(cmd) + " --help` to inspect subcommands and flags. For agent workflows, treat `--json` as the default for every command and subcommand, inspect `error.code` and `error.hint`, and use `--dry-run` before write operations when available. On Windows cmd, use double quotes and cmd-native commands such as `where`, `dir`, `cd`, and `type`.")
	}
}

func commandLong(h ProductHelp, meta llm.CommandMeta) string {
	var b strings.Builder
	b.WriteString(meta.Description)
	b.WriteString("\n\nAgent guidance:")
	b.WriteString("\n- Treat `--json` as the default for this command and every subcommand so results and failures use the stable envelope.")
	if len(meta.Required) > 0 {
		b.WriteString("\n- Required input: `")
		b.WriteString(strings.Join(meta.Required, "`, `"))
		b.WriteString("`.")
	}
	if strings.Contains(meta.Risk, "write") {
		b.WriteString("\n- Use `--dry-run` first when the command supports it.")
	}
	if meta.Risk == "delete" {
		b.WriteString("\n- Destructive commands require explicit `--yes` after user confirmation.")
	}
	if strings.TrimSpace(h.Instructions) != "" {
		b.WriteString("\n- VS Code GitHub Copilot guidance: ")
		b.WriteString(h.Instructions)
	}
	b.WriteString("\n- If `ok=false`, read `error.code`, `error.message`, and `error.hint` before retrying.")
	b.WriteString("\n- Command parsing failures return `invalid_args` JSON when `--json` is present.")
	b.WriteString("\n- On Windows cmd, use double quotes and cmd-native commands such as `where`, `dir`, `cd`, and `type`.")
	return b.String()
}

func describeFlags(cmd *cobra.Command, product, key string) {
	describeFlagSet(cmd.LocalFlags(), product, key)
	describeFlagSet(cmd.PersistentFlags(), product, key)
}

func describeFlagSet(flags *pflag.FlagSet, product, key string) {
	if flags == nil {
		return
	}
	flags.VisitAll(func(f *pflag.Flag) {
		if strings.TrimSpace(f.Usage) != "" {
			return
		}
		f.Usage = flagDescription(product, key, f.Name)
	})
}

func flagDescription(product, key, name string) string {
	desc := catalog.FlagDescription(key, name)
	if desc != "Command option." {
		return desc
	}
	switch name {
	case "instance":
		return "Configured " + product + " instance name."
	case "config":
		return "Path to the EFP config file."
	case "json":
		return "Print a JSON envelope to stdout."
	case "format":
		return "Output format: table, json, or yaml."
	case "verbose":
		return "Enable non-secret diagnostics."
	case "dry-run":
		return "Preview a write request without sending it."
	case "yes":
		return "Confirm a destructive operation."
	default:
		return desc
	}
}

func commandKey(cmd *cobra.Command) string {
	var parts []string
	for c := cmd; c != nil && c.HasParent(); c = c.Parent() {
		fields := strings.Fields(c.Use)
		if len(fields) == 0 {
			fields = []string{c.Name()}
		}
		var names []string
		for _, field := range fields {
			if strings.HasPrefix(field, "<") || strings.HasPrefix(field, "[") {
				continue
			}
			names = append(names, field)
		}
		if len(names) == 0 {
			names = append(names, c.Name())
		}
		parts = append(names, parts...)
	}
	return strings.Join(parts, ".")
}

func commandUsage(cmd *cobra.Command) string {
	if cmd == nil {
		return ""
	}
	parts := strings.Fields(cmd.CommandPath())
	if strings.HasPrefix(cmd.Use, "help llm") && len(parts) > 0 && parts[len(parts)-1] == "help" {
		parts = append(parts, "llm")
	}
	return strings.Join(parts, " ")
}

func fallbackDescription(product, key string, group bool) string {
	words := strings.ReplaceAll(key, ".", " ")
	words = strings.TrimSpace(words)
	if words == "" {
		words = "commands"
	}
	if group {
		return "Group for " + productName(product) + " " + words + " commands."
	}
	return "Run " + productName(product) + " " + words + "."
}

func productName(product string) string {
	switch product {
	case "jira":
		return "Jira"
	case "confluence":
		return "Confluence"
	case "browser":
		return "Browser"
	default:
		return product
	}
}
