package commands

import (
	"strings"

	"engineering-flow-platform-tools/internal/logtool"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func windowCmd(o *Opts) *cobra.Command {
	var runDir, entryID, file string
	var line, before, after int
	before, after = 20, 20
	c := &cobra.Command{
		Use:   "window",
		Short: "Return redacted source lines around an entry or file line",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(runDir) == "" {
				return print(cmd, o, output.Failure("invalid_args", "--run is required.", "Pass the run directory created by log analyze.", 400))
			}
			hasEntry := strings.TrimSpace(entryID) != ""
			hasFileLine := strings.TrimSpace(file) != "" || line > 0
			if hasEntry == hasFileLine {
				return print(cmd, o, output.Failure("invalid_args", "Use either --entry-id or --file plus --line.", "Run log schema window --json.", 400))
			}
			var (
				result logtool.WindowResult
				err    error
			)
			if hasEntry {
				result, err = logtool.WindowByEntry(runDir, entryID, before, after)
			} else {
				result, err = logtool.WindowByFileLine(file, line, before, after)
			}
			if err != nil {
				return printErr(cmd, o, err)
			}
			return print(cmd, o, output.Success("", result))
		},
	}
	c.Flags().StringVar(&runDir, "run", "", "Run directory produced by log analyze; required when using --entry-id.")
	c.Flags().StringVar(&entryID, "entry-id", "", "Entry id from entries, search, templates, or extract output.")
	c.Flags().StringVar(&file, "file", "", "Source file path for direct file/line window lookup.")
	c.Flags().IntVar(&line, "line", 0, "One-based source line number for direct file/line lookup.")
	c.Flags().IntVar(&before, "before", 20, "Number of lines before the target to return, up to 200.")
	c.Flags().IntVar(&after, "after", 20, "Number of lines after the target to return, up to 200.")
	return c
}
