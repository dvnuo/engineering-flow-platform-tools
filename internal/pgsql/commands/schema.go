package commands

import (
	"strings"

	"engineering-flow-platform-tools/internal/output"
	"engineering-flow-platform-tools/internal/pgsql"
	"github.com/spf13/cobra"
)

// schemaCmd serves two roles: `pgsql schema <command>` describes a pgsql
// command (catalog schema) while `pgsql schema tables|describe|indexes`
// inspect the database schema.
func schemaCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "schema <command>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return describeCommand(o, cmd, args[0])
	}}
	tables := &cobra.Command{Use: "tables", RunE: func(cmd *cobra.Command, args []string) error {
		schema := strings.TrimSpace(mustS(cmd, "schema"))
		if mustB(cmd, "all-schemas") {
			schema = ""
		}
		limit := mustI(cmd, "limit")
		if limit < 1 {
			return print(cmd, o, output.Failure("invalid_args", "--limit must be at least 1", "", 400))
		}
		return runPreset(o, cmd, pgsql.PresetSchemaTables, []any{schema}, limit, func(*session, *pgsql.Output) map[string]any {
			label := schema
			if label == "" {
				label = "*"
			}
			return map[string]any{"schema": label}
		})
	}}
	tables.Flags().String("schema", "public", "Schema to list.")
	tables.Flags().Bool("all-schemas", false, "List every non-system schema instead of --schema.")
	tables.Flags().Int("limit", 500, "Maximum relations to return.")
	c.AddCommand(tables)
	c.AddCommand(&cobra.Command{Use: "describe <table>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		table := strings.TrimSpace(args[0])
		s, err := loadSession(o)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		params := []any{table}
		if o.DryRun {
			return print(cmd, o, output.Success(s.inst.Name, dryRunData(s,
				dryStatement{SQL: pgsql.PresetRelation, Params: params},
				dryStatement{SQL: pgsql.PresetColumns, Params: params},
				dryStatement{SQL: pgsql.PresetIndexes, Params: params},
				dryStatement{SQL: pgsql.PresetConstraints, Params: params})))
		}
		rel, err := resolveRelation(s, table)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		cols, err := s.query(pgsql.PresetColumns, params, 5000)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		idx, err := s.query(pgsql.PresetIndexes, params, 1000)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		cons, err := s.query(pgsql.PresetConstraints, params, 1000)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		primaryKey := []any{}
		for _, row := range cols.Rows {
			if pk, _ := row["primary_key"].(bool); pk {
				primaryKey = append(primaryKey, row["name"])
			}
		}
		data := map[string]any{
			"table":        rel,
			"columns":      cols.Rows,
			"column_count": cols.RowCount,
			"primary_key":  primaryKey,
			"indexes":      idx.Rows,
			"constraints":  cons.Rows,
			"elapsed_ms":   cols.ElapsedMs + idx.ElapsedMs + cons.ElapsedMs,
		}
		if o.Verbose {
			data["connection"] = s.target()
		}
		return print(cmd, o, output.Success(s.inst.Name, data))
	}})
	c.AddCommand(&cobra.Command{Use: "indexes <table>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		table := strings.TrimSpace(args[0])
		s, err := loadSession(o)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		params := []any{table}
		if o.DryRun {
			return print(cmd, o, output.Success(s.inst.Name, dryRunData(s,
				dryStatement{SQL: pgsql.PresetRelation, Params: params},
				dryStatement{SQL: pgsql.PresetIndexes, Params: params})))
		}
		rel, err := resolveRelation(s, table)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		idx, err := s.query(pgsql.PresetIndexes, params, 1000)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		data := idx.Data()
		data["table"] = rel
		data["indexes"] = idx.Rows
		if o.Verbose {
			data["connection"] = s.target()
		}
		return print(cmd, o, output.Success(s.inst.Name, data))
	}})
	return c
}

// resolveRelation returns the relation row for table or a not_found error.
func resolveRelation(s *session, table string) (map[string]any, error) {
	if table == "" {
		return nil, &pgsql.Error{Code: pgsql.CodeInvalidArgs, Message: "table name is empty", Hint: "Pass a table name, optionally schema-qualified such as public.orders.", Status: 400}
	}
	rel, err := s.query(pgsql.PresetRelation, []any{table}, 2)
	if err != nil {
		return nil, err
	}
	if rel.RowCount == 0 {
		return nil, &pgsql.Error{Code: "not_found", Message: "relation " + table + " not found", Hint: "Run pgsql schema tables --json (or --all-schemas) to list relation names; quote mixed-case names such as public.\"MyTable\".", Status: 404}
	}
	return rel.Rows[0], nil
}
