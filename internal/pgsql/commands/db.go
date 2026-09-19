package commands

import (
	"engineering-flow-platform-tools/internal/output"
	"engineering-flow-platform-tools/internal/pgsql"
	"github.com/spf13/cobra"
)

func dbCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "db"}
	size := &cobra.Command{Use: "size", RunE: func(cmd *cobra.Command, args []string) error {
		top := mustI(cmd, "top")
		if top < 0 {
			return print(cmd, o, output.Failure("invalid_args", "--top must be at least 0", "", 400))
		}
		s, err := loadSession(o)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		params := []any{itoa(top)}
		if o.DryRun {
			return print(cmd, o, output.Success(s.inst.Name, dryRunData(s, dryStatement{SQL: pgsql.PresetDBSize}, dryStatement{SQL: pgsql.PresetLargestRelations, Params: params})))
		}
		main, err := s.query(pgsql.PresetDBSize, nil, 2)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		data := map[string]any{}
		if main.RowCount > 0 {
			for k, v := range main.Rows[0] {
				data[k] = v
			}
		}
		data["elapsed_ms"] = main.ElapsedMs
		data["largest_relations"] = []map[string]any{}
		if top > 0 {
			largest, err := s.query(pgsql.PresetLargestRelations, params, top)
			if err != nil {
				return print(cmd, o, envelopeError(err))
			}
			data["largest_relations"] = largest.Rows
			data["elapsed_ms"] = main.ElapsedMs + largest.ElapsedMs
		}
		if o.Verbose {
			data["connection"] = s.target()
		}
		return print(cmd, o, output.Success(s.inst.Name, data))
	}}
	size.Flags().Int("top", 10, "Number of largest relations to include (0 to skip).")
	c.AddCommand(size)
	return c
}
