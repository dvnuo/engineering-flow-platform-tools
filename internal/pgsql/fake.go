package pgsql

import (
	"context"
	"time"
)

// FakeCall records one Executor.Query invocation.
type FakeCall struct {
	Conn  ConnParams
	SQL   string
	Args  []any
	Limit int
}

// FakeExecutor scripts results for tests that have no PostgreSQL server. The
// Handler receives every call (in order) and returns the raw result or error
// the real executor would have produced; a nil Handler returns an empty
// result. It is exported so command tests in other packages can use it.
type FakeExecutor struct {
	Calls   []FakeCall
	Handler func(call FakeCall) (*RawResult, error)
}

// Query implements Executor.
func (f *FakeExecutor) Query(_ context.Context, conn ConnParams, sql string, args []any, limit int) (*RawResult, error) {
	call := FakeCall{Conn: conn, SQL: sql, Args: args, Limit: limit}
	f.Calls = append(f.Calls, call)
	if f.Handler == nil {
		return &RawResult{Columns: []Column{}, Rows: [][]any{}}, nil
	}
	return f.Handler(call)
}

// Cols builds text columns from names.
func Cols(names ...string) []Column {
	out := make([]Column, 0, len(names))
	for _, n := range names {
		out = append(out, Column{Name: n, Type: "text"})
	}
	return out
}

// FakeResult builds a raw result with a one millisecond elapsed time.
func FakeResult(columns []Column, rows ...[]any) *RawResult {
	if rows == nil {
		rows = [][]any{}
	}
	return &RawResult{Columns: columns, Rows: rows, Elapsed: time.Millisecond}
}
