package commands

import (
	"errors"
	"strconv"
	"strings"

	"engineering-flow-platform-tools/internal/output"
	"engineering-flow-platform-tools/internal/pgsql"
	"github.com/spf13/cobra"
)

func statCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "stat"}
	activity := &cobra.Command{Use: "activity", RunE: func(cmd *cobra.Command, args []string) error {
		state := strings.TrimSpace(mustS(cmd, "state"))
		minDuration := mustI(cmd, "min-duration-sec")
		limit := mustI(cmd, "limit")
		if limit < 1 || minDuration < 0 {
			return print(cmd, o, output.Failure("invalid_args", "--limit must be at least 1 and --min-duration-sec at least 0", "", 400))
		}
		return runPreset(o, cmd, pgsql.PresetStatActivity, []any{state, itoa(minDuration)}, limit, func(*session, *pgsql.Output) map[string]any {
			return map[string]any{"filters": map[string]any{"state": state, "min_duration_sec": minDuration}}
		})
	}}
	activity.Flags().String("state", "", "Only sessions in this state: active, idle, 'idle in transaction', 'idle in transaction (aborted)', 'fastpath function call', disabled.")
	activity.Flags().Int("min-duration-sec", 0, "Only non-idle sessions whose current statement has run at least this many seconds.")
	activity.Flags().Int("limit", 100, "Maximum sessions to return.")
	c.AddCommand(activity)

	locks := &cobra.Command{Use: "locks", RunE: func(cmd *cobra.Command, args []string) error {
		blocked := mustB(cmd, "blocked-only")
		limit := mustI(cmd, "limit")
		if limit < 1 {
			return print(cmd, o, output.Failure("invalid_args", "--limit must be at least 1", "", 400))
		}
		return runPreset(o, cmd, pgsql.PresetStatLocks, []any{strconv.FormatBool(blocked)}, limit, func(*session, *pgsql.Output) map[string]any {
			return map[string]any{"blocked_only": blocked}
		})
	}}
	locks.Flags().Bool("blocked-only", false, "Only locks that are waiting plus the sessions holding what they wait for.")
	locks.Flags().Int("limit", 200, "Maximum lock rows to return.")
	c.AddCommand(locks)

	slow := &cobra.Command{Use: "slow", RunE: func(cmd *cobra.Command, args []string) error {
		limit := mustI(cmd, "limit")
		if limit < 1 {
			return print(cmd, o, output.Failure("invalid_args", "--limit must be at least 1", "", 400))
		}
		sortBy := strings.ToLower(strings.TrimSpace(mustS(cmd, "sort")))
		sql, err := pgsql.StatSlowSQL(sortBy, false)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		s, err := loadSession(o)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		params := []any{itoa(limit)}
		if o.DryRun {
			return print(cmd, o, output.Success(s.inst.Name, dryRunData(s, dryStatement{SQL: pgsql.PresetStatStatementsExtension}, dryStatement{SQL: sql, Params: params})))
		}
		ext, err := s.query(pgsql.PresetStatStatementsExtension, nil, 2)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		noReport := func(reason string) error {
			return print(cmd, o, output.Success(s.inst.Name, map[string]any{"has_report": false, "reason": reason, "rows": []any{}, "row_count": 0, "sort": sortBy, "limit": limit}))
		}
		if ext.RowCount == 0 {
			return noReport("pg_stat_statements extension is not installed in this database")
		}
		out, err := s.query(sql, params, limit)
		var pe *pgsql.Error
		if errors.As(err, &pe) {
			switch pe.SQLState {
			case "42703":
				legacy, _ := pgsql.StatSlowSQL(sortBy, true)
				out, err = s.query(legacy, params, limit)
			case "42P01":
				schema, _ := ext.Rows[0]["schema"].(string)
				return noReport("pg_stat_statements view is not visible on the search_path (installed in schema " + schema + ")")
			}
		}
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		data := out.Data()
		data["has_report"] = true
		data["extension_version"] = ext.Rows[0]["version"]
		data["extension_schema"] = ext.Rows[0]["schema"]
		data["sort"] = sortBy
		data["limit"] = limit
		if o.Verbose {
			data["connection"] = s.target()
		}
		return print(cmd, o, output.Success(s.inst.Name, data))
	}}
	slow.Flags().Int("limit", 20, "Maximum statements to return.")
	slow.Flags().String("sort", "total", "Order by: "+strings.Join(pgsql.StatSlowSorts(), ", ")+".")
	c.AddCommand(slow)

	c.AddCommand(&cobra.Command{Use: "replication", RunE: func(cmd *cobra.Command, args []string) error {
		s, err := loadSession(o)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		if o.DryRun {
			return print(cmd, o, output.Success(s.inst.Name, dryRunData(s, dryStatement{SQL: pgsql.PresetStatReplicationStatus}, dryStatement{SQL: pgsql.PresetStatReplication})))
		}
		status, err := s.query(pgsql.PresetStatReplicationStatus, nil, 2)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		replicas, err := s.query(pgsql.PresetStatReplication, nil, 500)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		data := replicas.Data()
		data["elapsed_ms"] = status.ElapsedMs + replicas.ElapsedMs
		if status.RowCount > 0 {
			for k, v := range status.Rows[0] {
				data[k] = v
			}
		}
		if o.Verbose {
			data["connection"] = s.target()
		}
		return print(cmd, o, output.Success(s.inst.Name, data))
	}})

	tables := &cobra.Command{Use: "tables", RunE: func(cmd *cobra.Command, args []string) error {
		schema := strings.TrimSpace(mustS(cmd, "schema"))
		limit := mustI(cmd, "limit")
		if limit < 1 {
			return print(cmd, o, output.Failure("invalid_args", "--limit must be at least 1", "", 400))
		}
		sortBy := strings.ToLower(strings.TrimSpace(mustS(cmd, "sort")))
		sql, err := pgsql.StatTablesSQL(sortBy)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		return runPreset(o, cmd, sql, []any{schema, itoa(limit)}, limit, func(*session, *pgsql.Output) map[string]any {
			label := schema
			if label == "" {
				label = "*"
			}
			return map[string]any{"schema": label, "sort": sortBy, "limit": limit}
		})
	}}
	tables.Flags().String("schema", "", "Only tables in this schema (default: every user schema).")
	tables.Flags().String("sort", "n_dead_tup", "Order by: "+strings.Join(pgsql.StatTablesSorts(), ", ")+".")
	tables.Flags().Int("limit", 50, "Maximum tables to return.")
	c.AddCommand(tables)
	return c
}
