package pgsql

import (
	"bytes"
	"context"
	"database/sql/driver"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

const (
	// ApplicationName is reported to the server so DBAs can spot CLI sessions.
	ApplicationName = "efp-pgsql"
	// ConnectTimeout bounds the TCP/TLS/auth handshake.
	ConnectTimeout = 10 * time.Second
	// DefaultStatementTimeout applies when an instance sets none.
	DefaultStatementTimeout = 30 * time.Second
	// DefaultLimit is the row cap when a caller passes none.
	DefaultLimit = 200
	// DefaultMaxCellChars caps a single cell in envelope output.
	DefaultMaxCellChars = 2000
)

// ConnParams is everything needed to open one read-only session.
type ConnParams struct {
	Host             string
	Port             int
	Database         string
	Username         string
	Password         string
	SSLMode          string
	CACert           string // PEM; written to a 0600 temp file for sslrootcert
	StatementTimeout time.Duration
}

// EffectiveTimeout returns the statement timeout or the package default.
func (p ConnParams) EffectiveTimeout() time.Duration {
	if p.StatementTimeout <= 0 {
		return DefaultStatementTimeout
	}
	return p.StatementTimeout
}

// Column describes one result column.
type Column struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// RawResult is what an Executor hands back: at most limit+1 unshaped rows.
type RawResult struct {
	Columns []Column
	Rows    [][]any
	Notices []string
	Elapsed time.Duration
}

// Executor runs one guarded statement inside a READ ONLY transaction and
// returns at most limit+1 rows so the caller can detect truncation. The pgx
// implementation lives in pgx_executor.go; tests use FakeExecutor.
type Executor interface {
	Query(ctx context.Context, conn ConnParams, sql string, args []any, limit int) (*RawResult, error)
}

// Output is the shaped, envelope-ready result of a statement.
type Output struct {
	Columns        []Column         `json:"columns"`
	Rows           []map[string]any `json:"rows"`
	RowCount       int              `json:"row_count"`
	RowsTruncated  bool             `json:"rows_truncated"`
	CellsTruncated bool             `json:"cells_truncated"`
	ElapsedMs      int64            `json:"elapsed_ms"`
	Notices        []string         `json:"notices"`
}

// Execute guards sql, runs it through exec, and shapes the result: rows are
// objects keyed by (de-duplicated) column name, values are JSON-safe, at most
// limit rows are kept, and rows_truncated says whether more existed.
func Execute(ctx context.Context, exec Executor, conn ConnParams, sql string, args []any, limit int) (*Output, error) {
	stmt, err := Prepare(sql)
	if err != nil {
		return nil, err
	}
	if limit < 1 {
		limit = DefaultLimit
	}
	raw, err := exec.Query(ctx, conn, stmt.SQL, args, limit)
	if err != nil {
		return nil, err
	}
	return Shape(raw, limit), nil
}

// Shape converts a raw result into envelope-ready output.
func Shape(raw *RawResult, limit int) *Output {
	out := &Output{Columns: []Column{}, Rows: []map[string]any{}, Notices: []string{}}
	if raw == nil {
		return out
	}
	names := uniqueColumnNames(raw.Columns)
	for i, c := range raw.Columns {
		out.Columns = append(out.Columns, Column{Name: names[i], Type: c.Type})
	}
	rows := raw.Rows
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
		out.RowsTruncated = true
	}
	for _, r := range rows {
		m := make(map[string]any, len(names))
		for i, name := range names {
			var v any
			if i < len(r) {
				v = r[i]
			}
			m[name] = NormalizeValue(v)
		}
		out.Rows = append(out.Rows, m)
	}
	out.RowCount = len(out.Rows)
	out.ElapsedMs = raw.Elapsed.Milliseconds()
	out.Notices = append(out.Notices, raw.Notices...)
	return out
}

// Data returns the output as a map so commands can add command-specific keys.
func (o *Output) Data() map[string]any {
	return map[string]any{
		"columns":         o.Columns,
		"rows":            o.Rows,
		"row_count":       o.RowCount,
		"rows_truncated":  o.RowsTruncated,
		"cells_truncated": o.CellsTruncated,
		"elapsed_ms":      o.ElapsedMs,
		"notices":         o.Notices,
	}
}

// Column returns the values of one column across all rows.
func (o *Output) Column(name string) []any {
	out := make([]any, 0, len(o.Rows))
	for _, r := range o.Rows {
		out = append(out, r[name])
	}
	return out
}

func uniqueColumnNames(cols []Column) []string {
	names := make([]string, len(cols))
	seen := map[string]int{}
	for i, c := range cols {
		name := c.Name
		if name == "" {
			name = fmt.Sprintf("column_%d", i+1)
		}
		seen[name]++
		if seen[name] > 1 {
			name = fmt.Sprintf("%s_%d", name, seen[name])
		}
		names[i] = name
	}
	return names
}

var numericText = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?([eE][-+]?[0-9]+)?$`)

// NormalizeValue turns a pgx-decoded value into something encoding/json can
// carry losslessly: bytea becomes {"bytes": n}, times become RFC3339, UUIDs
// and network types become strings, numerics stay exact as JSON numbers, and
// NaN/Inf floats become strings so the envelope never fails to encode.
func NormalizeValue(v any) any {
	switch x := v.(type) {
	case nil:
		return nil
	case string, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, json.Number:
		return x
	case float32:
		return normalizeFloat(float64(x))
	case float64:
		return normalizeFloat(x)
	case []byte:
		return map[string]any{"bytes": len(x)}
	case [16]byte:
		return fmt.Sprintf("%x-%x-%x-%x-%x", x[0:4], x[4:6], x[6:8], x[8:10], x[10:16])
	case time.Time:
		return x.Format(time.RFC3339Nano)
	case time.Duration:
		return x.String()
	case pgtype.Numeric:
		if !x.Valid {
			return nil
		}
		dv, err := x.Value()
		if err != nil {
			return fmt.Sprint(x)
		}
		s, _ := dv.(string)
		if numericText.MatchString(s) {
			return json.Number(s)
		}
		return s
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = NormalizeValue(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, item := range x {
			out[k] = NormalizeValue(item)
		}
		return out
	case driver.Valuer:
		dv, err := x.Value()
		if err != nil {
			return fmt.Sprint(v)
		}
		if dv == nil {
			return nil
		}
		if reflect.TypeOf(dv) == reflect.TypeOf(v) {
			return fmt.Sprint(dv)
		}
		return NormalizeValue(dv)
	case fmt.Stringer:
		return x.String()
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr:
		if rv.IsNil() {
			return nil
		}
		return NormalizeValue(rv.Elem().Interface())
	case reflect.Slice, reflect.Array:
		out := make([]any, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			out[i] = NormalizeValue(rv.Index(i).Interface())
		}
		return out
	case reflect.Map:
		out := make(map[string]any, rv.Len())
		for _, k := range rv.MapKeys() {
			out[fmt.Sprint(k.Interface())] = NormalizeValue(rv.MapIndex(k).Interface())
		}
		return out
	}
	if b, err := json.Marshal(v); err == nil {
		return string(b)
	}
	return fmt.Sprint(v)
}

func normalizeFloat(f float64) any {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	}
	return f
}

const truncatedSuffix = "...[truncated]"

// TruncateCells shortens string cells (and stringifies oversized arrays or
// JSON documents) longer than max characters, marking cells_truncated.
func (o *Output) TruncateCells(max int) {
	if max <= 0 {
		return
	}
	for _, row := range o.Rows {
		for k, v := range row {
			if nv, cut := truncateValue(v, max); cut {
				row[k] = nv
				o.CellsTruncated = true
			}
		}
	}
}

func truncateValue(v any, max int) (any, bool) {
	switch x := v.(type) {
	case string:
		return truncateString(x, max)
	case map[string]any:
		if isByteaSummary(x) {
			return v, false
		}
		b, err := json.Marshal(x)
		if err != nil || len(b) <= max {
			return v, false
		}
		return truncateString(string(b), max)
	case []any:
		b, err := json.Marshal(x)
		if err != nil || len(b) <= max {
			return v, false
		}
		return truncateString(string(b), max)
	}
	return v, false
}

// isByteaSummary recognizes the {"bytes": n} placeholder NormalizeValue emits
// for bytea so cell truncation never mangles it.
func isByteaSummary(m map[string]any) bool {
	if len(m) != 1 {
		return false
	}
	_, ok := m["bytes"].(int)
	return ok
}

func truncateString(s string, max int) (any, bool) {
	if len(s) <= max {
		return s, false
	}
	r := []rune(s)
	if len(r) <= max {
		return s, false
	}
	return string(r[:max]) + truncatedSuffix, true
}

// FileOutput is returned instead of rows when the result is written to disk.
type FileOutput struct {
	Path          string `json:"path"`
	Bytes         int64  `json:"bytes"`
	Format        string `json:"format"`
	RowCount      int    `json:"row_count"`
	RowsTruncated bool   `json:"rows_truncated"`
}

// WriteFile writes the full (not cell-truncated) result to path as CSV when
// the name ends in .csv and as JSON otherwise. Column order is preserved.
func WriteFile(out *Output, path string) (*FileOutput, error) {
	if strings.TrimSpace(path) == "" {
		return nil, invalidArgs("--output path is empty", "")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, invalidArgs("invalid --output path: "+err.Error(), "")
	}
	format := "json"
	var data []byte
	if strings.EqualFold(filepath.Ext(abs), ".csv") {
		format = "csv"
		data, err = encodeCSV(out)
	} else {
		data, err = encodeOrderedJSON(out)
	}
	if err != nil {
		return nil, &Error{Code: CodeServerError, Message: "cannot encode result: " + err.Error(), Status: 500}
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
		return nil, invalidArgs("cannot create output directory: "+err.Error(), "")
	}
	if err := os.WriteFile(abs, data, 0o600); err != nil {
		return nil, invalidArgs("cannot write output file: "+err.Error(), "")
	}
	return &FileOutput{Path: abs, Bytes: int64(len(data)), Format: format, RowCount: out.RowCount, RowsTruncated: out.RowsTruncated}, nil
}

func encodeCSV(out *Output) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	header := make([]string, len(out.Columns))
	for i, c := range out.Columns {
		header[i] = c.Name
	}
	if err := w.Write(header); err != nil {
		return nil, err
	}
	for _, row := range out.Rows {
		record := make([]string, len(out.Columns))
		for i, c := range out.Columns {
			record[i] = CellString(row[c.Name])
		}
		if err := w.Write(record); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

func encodeOrderedJSON(out *Output) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("{\n  \"columns\": ")
	cols, err := json.Marshal(out.Columns)
	if err != nil {
		return nil, err
	}
	buf.Write(cols)
	fmt.Fprintf(&buf, ",\n  \"row_count\": %d,\n  \"rows_truncated\": %t,\n  \"rows\": [", out.RowCount, out.RowsTruncated)
	for i, row := range out.Rows {
		if i > 0 {
			buf.WriteString(",")
		}
		buf.WriteString("\n    {")
		for j, c := range out.Columns {
			if j > 0 {
				buf.WriteString(", ")
			}
			key, _ := json.Marshal(c.Name)
			val, err := json.Marshal(row[c.Name])
			if err != nil {
				return nil, err
			}
			buf.Write(key)
			buf.WriteString(": ")
			buf.Write(val)
		}
		buf.WriteString("}")
	}
	if len(out.Rows) > 0 {
		buf.WriteString("\n  ")
	}
	buf.WriteString("]\n}\n")
	return buf.Bytes(), nil
}

// CellString renders a normalized cell for CSV or table output.
func CellString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case json.Number:
		return x.String()
	case []any, map[string]any:
		b, err := json.Marshal(x)
		if err != nil {
			return fmt.Sprint(x)
		}
		return string(b)
	}
	return fmt.Sprint(v)
}
