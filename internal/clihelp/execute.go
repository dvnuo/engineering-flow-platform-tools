package clihelp

import (
	"io"
	"regexp"
	"strings"

	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func Execute(root *cobra.Command, product string, args []string, stdout, stderr io.Writer) int {
	if root == nil {
		_ = output.Print(stdout, "json", output.Failure("invalid_args", "CLI root command is not configured.", "Reinstall or rebuild the CLI binary.", 500))
		return 1
	}
	if stdout != nil {
		root.SetOut(stdout)
	}
	if stderr != nil {
		root.SetErr(stderr)
	}
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		_ = printExecuteError(root.OutOrStdout(), productNameForExecute(product, root), args, err)
		return 1
	}
	return 0
}

func printExecuteError(w io.Writer, product string, args []string, err error) error {
	return output.Print(w, fallbackFormat(args), output.Failure(
		"invalid_args",
		product+" command could not be executed. "+redactCLIError(err.Error()),
		"Run "+product+" commands --json or "+product+" help llm --json. On Windows cmd, use double quotes, verify PATH with where, and read error.code/error.hint before retrying.",
		400,
	))
}

func fallbackFormat(args []string) string {
	for i, arg := range args {
		switch {
		case arg == "--json":
			return "json"
		case arg == "--format=json" || arg == "-o=json":
			return "json"
		case arg == "--format=yaml" || arg == "-o=yaml":
			return "yaml"
		case arg == "--format=table" || arg == "-o=table":
			return "table"
		case arg == "--format" && i+1 < len(args):
			switch strings.ToLower(strings.TrimSpace(args[i+1])) {
			case "json":
				return "json"
			case "yaml":
				return "yaml"
			case "table":
				return "table"
			}
		}
	}
	return "table"
}

func productNameForExecute(product string, root *cobra.Command) string {
	product = strings.TrimSpace(product)
	if product != "" {
		return product
	}
	if root != nil {
		if fields := strings.Fields(root.Use); len(fields) > 0 {
			return fields[0]
		}
	}
	return "cli"
}

var cliSecretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)Authorization\s*:\s*Bearer\s+[A-Za-z0-9._~+/=-]+`),
	regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9._~+/=-]+`),
	regexp.MustCompile(`(?i)(?:gho|ghp|ghu|ghs|github_pat)_[A-Za-z0-9_]+`),
	regexp.MustCompile(`(?i)"?(?:password|api_key|api-key|token|access_token|github_access_token|copilot_token)"?\s*[:=]\s*"?[^",;&\s}]+`),
	regexp.MustCompile(`(?i)(data:image/[a-z0-9.+-]+;base64,)[A-Za-z0-9+/=_-]+`),
}

func redactCLIError(s string) string {
	s = strings.TrimSpace(s)
	for _, re := range cliSecretPatterns {
		s = re.ReplaceAllString(s, "[REDACTED]")
	}
	if len(s) > 1000 {
		s = s[:1000] + "...(truncated)"
	}
	if s == "" {
		return "unknown error"
	}
	return s
}
