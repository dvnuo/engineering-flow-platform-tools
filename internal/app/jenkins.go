package app

import (
	"engineering-flow-platform-tools/internal/catalog"
	"engineering-flow-platform-tools/internal/cli"
	"engineering-flow-platform-tools/internal/llm"
	"github.com/spf13/cobra"
)

func NewJenkinsRootCommand(commands []string) *cobra.Command {
	r := llm.NewRegistry()
	for _, c := range catalog.Commands("jenkins") {
		r.Register(c)
	}
	return cli.NewRootCommand(cli.BuilderInput{Use: "jenkins", Short: "Jenkins CLI", Long: "Jenkins command line interface.", Registry: r})
}
