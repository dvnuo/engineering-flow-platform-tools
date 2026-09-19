package commands

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/output"
	"engineering-flow-platform-tools/internal/pgsql"
	"github.com/spf13/cobra"
)

func addSQLFlags(cmd *cobra.Command) {
	cmd.Flags().String("sql", "", "Statement text (SELECT, WITH, TABLE, VALUES, SHOW).")
	cmd.Flags().String("sql-file", "", "Read the statement from a file instead of --sql.")
	cmd.Flags().StringArray("param", nil, "Positional parameter value for $1, $2, ... (repeatable; sent as text and coerced by the server, cast explicitly such as $1::int).")
	cmd.Flags().Int("timeout-sec", 0, "Statement timeout in seconds; capped by the instance statement_timeout_seconds (default: the instance value).")
}

// readSQL returns the statement from --sql or --sql-file (exactly one).
func readSQL(cmd *cobra.Command) (string, error) {
	inline := mustS(cmd, "sql")
	file := strings.TrimSpace(mustS(cmd, "sql-file"))
	switch {
	case strings.TrimSpace(inline) != "" && file != "":
		return "", errors.New("pass either --sql or --sql-file, not both")
	case file != "":
		b, err := os.ReadFile(file)
		if err != nil {
			return "", errors.New("cannot read --sql-file: " + err.Error())
		}
		return string(b), nil
	case strings.TrimSpace(inline) != "":
		return inline, nil
	}
	return "", errors.New("--sql or --sql-file required")
}

func effectiveLimit(flag, maxRows int) (int, bool, error) {
	if flag < 1 {
		return 0, false, errors.New("--limit must be at least 1")
	}
	if maxRows > 0 && flag > maxRows {
		return maxRows, true, nil
	}
	return flag, false, nil
}

func effectiveTimeout(flagSec int, instance time.Duration) time.Duration {
	if flagSec <= 0 {
		return instance
	}
	d := time.Duration(flagSec) * time.Second
	if d > instance {
		return instance
	}
	return d
}

func queryCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "query", RunE: func(cmd *cobra.Command, args []string) error {
		text, err := readSQL(cmd)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", err.Error(), "Run pgsql schema query --json for the flag list.", 400))
		}
		stmt, err := pgsql.Prepare(text)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		s, err := loadSession(o)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		limit, capped, err := effectiveLimit(mustI(cmd, "limit"), s.inst.EffectiveMaxRows())
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", err.Error(), "", 400))
		}
		s.conn.StatementTimeout = effectiveTimeout(mustI(cmd, "timeout-sec"), s.conn.StatementTimeout)
		params := toArgs(mustSA(cmd, "param"))
		meta := map[string]any{
			"limit":                     limit,
			"limit_capped":              capped,
			"statement_timeout_seconds": int(s.conn.StatementTimeout.Seconds()),
		}
		if o.DryRun {
			data := dryRunData(s, dryStatement{SQL: stmt.SQL, Params: params})
			data["kind"] = stmt.Kind
			for k, v := range meta {
				data[k] = v
			}
			return print(cmd, o, output.Success(s.inst.Name, data))
		}
		out, err := pgsql.Execute(context.Background(), s.exec, s.conn, stmt.SQL, params, limit)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		if path := strings.TrimSpace(mustS(cmd, "output")); path != "" {
			file, err := pgsql.WriteFile(out, path)
			if err != nil {
				return print(cmd, o, envelopeError(err))
			}
			data := map[string]any{"path": file.Path, "bytes": file.Bytes, "format": file.Format, "row_count": file.RowCount, "rows_truncated": file.RowsTruncated, "elapsed_ms": out.ElapsedMs, "notices": out.Notices}
			for k, v := range meta {
				data[k] = v
			}
			return print(cmd, o, output.Success(s.inst.Name, data))
		}
		out.TruncateCells(mustI(cmd, "max-cell-chars"))
		data := out.Data()
		for k, v := range meta {
			data[k] = v
		}
		if o.Verbose {
			data["connection"] = s.target()
		}
		return print(cmd, o, output.Success(s.inst.Name, data))
	}}
	addSQLFlags(c)
	c.Flags().Int("limit", pgsql.DefaultLimit, "Maximum rows to return; capped by the instance max_rows.")
	c.Flags().String("output", "", "Write the full result to this file (CSV when the name ends in .csv, JSON otherwise) and return only file metadata.")
	c.Flags().Int("max-cell-chars", pgsql.DefaultMaxCellChars, "Truncate cells longer than this many characters in envelope output (0 disables).")
	return c
}

func explainCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "explain", RunE: func(cmd *cobra.Command, args []string) error {
		text, err := readSQL(cmd)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", err.Error(), "Run pgsql schema explain --json for the flag list.", 400))
		}
		format := strings.ToLower(strings.TrimSpace(mustS(cmd, "plan-format")))
		if format != "text" && format != "json" {
			return print(cmd, o, output.Failure("invalid_args", "--plan-format must be text or json", "", 400))
		}
		inner, err := pgsql.Prepare(text)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		analyze := mustB(cmd, "analyze")
		full := pgsql.ExplainSQL(inner.SQL, analyze, format)
		if _, err := pgsql.Prepare(full); err != nil {
			return print(cmd, o, envelopeError(err))
		}
		s, err := loadSession(o)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		s.conn.StatementTimeout = effectiveTimeout(mustI(cmd, "timeout-sec"), s.conn.StatementTimeout)
		params := toArgs(mustSA(cmd, "param"))
		if o.DryRun {
			data := dryRunData(s, dryStatement{SQL: full, Params: params})
			data["analyze"] = analyze
			data["plan_format"] = format
			return print(cmd, o, output.Success(s.inst.Name, data))
		}
		out, err := pgsql.Execute(context.Background(), s.exec, s.conn, full, params, 10000)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		data := map[string]any{"analyze": analyze, "plan_format": format, "elapsed_ms": out.ElapsedMs, "notices": out.Notices, "statement_timeout_seconds": int(s.conn.StatementTimeout.Seconds())}
		if analyze {
			data["executed"] = true
			data["note"] = "EXPLAIN ANALYZE executed the statement inside the READ ONLY transaction; it was rolled back."
		}
		var plan any
		if len(out.Columns) > 0 {
			values := out.Column(out.Columns[0].Name)
			if format == "json" && len(values) > 0 {
				plan = values[0]
			} else {
				lines := make([]string, 0, len(values))
				for _, v := range values {
					lines = append(lines, pgsql.CellString(v))
				}
				plan = strings.Join(lines, "\n")
				data["plan_lines"] = lines
			}
		}
		data["plan"] = plan
		if o.Verbose {
			data["connection"] = s.target()
		}
		return print(cmd, o, output.Success(s.inst.Name, data))
	}}
	addSQLFlags(c)
	c.Flags().Bool("analyze", false, "Run EXPLAIN ANALYZE (executes the statement inside the READ ONLY transaction, then rolls back).")
	c.Flags().String("plan-format", "text", "Plan format: text or json.")
	return c
}
