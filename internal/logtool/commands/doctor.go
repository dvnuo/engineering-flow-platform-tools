package commands

import (
	"engineering-flow-platform-tools/internal/logtool"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func doctorCmd(o *Opts) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check local log CLI defaults and safety assumptions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return print(cmd, o, output.Success("", logtool.Doctor()))
		},
	}
}
