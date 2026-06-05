package commands

import (
	"errors"
	"strings"

	"engineering-flow-platform-tools/internal/clihelp"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

type Opts struct {
	TemplateDir   string
	Config        string
	Format        string
	JSON          bool
	Verbose       bool
	DryRun        bool
	OfflineStrict bool
}

type codedError interface {
	error
	Code() string
	Message() string
	Hint() string
	Status() int
}

type templateIDError interface {
	TemplateID() string
}

type fileError interface {
	File() string
}

type missingFilesError interface {
	MissingFiles() []string
}

type orphanTemplateDirsError interface {
	OrphanTemplateDirs() []string
}

func NewRoot() *cobra.Command {
	cobra.EnableCommandSorting = false
	o := &Opts{Format: "table", OfflineStrict: true}
	cmd := &cobra.Command{
		Use:           "visual",
		Short:         "Generate offline visual artifacts from local templates",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	cmd.PersistentFlags().StringVar(&o.TemplateDir, "template-dir", "", "Path to visual templates; defaults to ~/.efp/template/visual, then ./templates/visual")
	cmd.PersistentFlags().StringVar(&o.Config, "config", "", "Path to EFP config file")
	cmd.PersistentFlags().BoolVar(&o.JSON, "json", false, "Output stable JSON envelope")
	cmd.PersistentFlags().StringVar(&o.Format, "format", "table", "Output format: table|json|yaml")
	cmd.PersistentFlags().BoolVar(&o.Verbose, "verbose", false, "Enable non-secret diagnostics")
	cmd.PersistentFlags().BoolVar(&o.DryRun, "dry-run", false, "Plan the operation without writing files")
	cmd.PersistentFlags().BoolVar(&o.OfflineStrict, "offline-strict", true, "Reject remote URLs, network APIs, and absolute asset references")

	cmd.AddCommand(renderCmd(o), inspectInputCmd(o), inspectPlanCmd(o), validateCmd(o), templateCmd(o), inspectOutputCmd(o), commandsCmd(o), schemaCmd(o), helpLLMCmd(o), versionCmd(o))
	clihelp.ApplyCatalogHelp(cmd, clihelp.ProductHelp{
		Product: "visual",
		Binary:  "visual",
		Short:   "Generate complete offline static visualization artifacts from local templates",
		Long: strings.TrimSpace(`visual is a terminal-invoked CLI for agents and scripts that need deterministic offline HTML/SVG artifacts.

It reads local templates from ~/.efp/template/visual by default, with checkout and release fallbacks, validates JSON input, copies local assets, and writes index.html, manifest.json, manifest.js, data.js, and assets/** to an output directory. It does not start a server, call Portal, call MCP, use Node/npm, download assets, or generate arbitrary JavaScript.`),
		Examples: []string{
			`visual commands --json`,
			`visual schema render --json`,
			`visual template list --template-dir ./templates/visual --json`,
			`visual template list --template-dir ./templates/visual --category uml --json`,
			`visual template schema uml.sequence_3d --template-dir ./templates/visual --json`,
			`visual template guide uml.sequence_3d --template-dir ./templates/visual --json`,
			`visual inspect-input --template uml.sequence_3d --template-dir ./templates/visual --input ./templates/visual/uml.sequence_3d/examples/basic.input.json --json`,
			`visual inspect-plan --template uml.sequence_3d --template-dir ./templates/visual --input ./templates/visual/uml.sequence_3d/examples/game-session-flow.input.json --out ./out/sequence --json`,
			`visual render --template uml.sequence_3d --template-dir ./templates/visual --input ./templates/visual/uml.sequence_3d/examples/game-session-flow.input.json --out ./out/sequence --title "Checkout Sequence" --json`,
		},
		Instructions: "copy cmd/visual/visual-cli.instructions.md to ~/.copilot/instructions/visual-cli.instructions.md.",
	})
	return cmd
}

func fmtOut(o *Opts) string {
	if o.JSON {
		return "json"
	}
	if o.Format != "" {
		return strings.ToLower(o.Format)
	}
	return "table"
}

func print(cmd *cobra.Command, o *Opts, env output.Envelope) error {
	return output.Print(cmd.OutOrStdout(), fmtOut(o), env)
}

func failureFromError(err error, fallbackCode string) output.Envelope {
	var ce codedError
	if errors.As(err, &ce) {
		detail := &output.ErrorDetail{Code: ce.Code(), Message: ce.Message(), Hint: ce.Hint(), Status: ce.Status()}
		var te templateIDError
		if errors.As(err, &te) {
			detail.TemplateID = te.TemplateID()
		}
		var fe fileError
		if errors.As(err, &fe) {
			detail.File = fe.File()
		}
		var me missingFilesError
		if errors.As(err, &me) {
			detail.MissingFiles = me.MissingFiles()
		}
		var oe orphanTemplateDirsError
		if errors.As(err, &oe) {
			detail.OrphanTemplateDirs = oe.OrphanTemplateDirs()
		}
		return output.Envelope{OK: false, Error: detail}
	}
	if fallbackCode == "" {
		fallbackCode = "output_write_failed"
	}
	return output.Failure(fallbackCode, err.Error(), "Inspect the command arguments and retry with --json for a stable envelope.", 500)
}

func invalidArgs(cmd *cobra.Command, o *Opts, message, hint string) error {
	return print(cmd, o, output.Failure("invalid_args", message, hint, 400))
}
