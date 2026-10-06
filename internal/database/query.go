package database

import (
	"context"
	"database/sql"
	"math"
	"strconv"
	"strings"

	"github.com/camel/coremesh/pkg/sdk"
)

// Querier ist ein Pool (*sql.DB) oder eine Transaktion (*sql.Tx).
type Querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Query führt ein SELECT aus und liest alle Zeilen. Text-Spalten, die der
// Treiber als []byte liefert, werden zu string.
func Query(ctx context.Context, q Querier, query string, args ...any) (*sdk.QueryResult, error) {
	rows, err := q.QueryContext(ctx, query, normalizeArgs(args)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	res := &sdk.QueryResult{Columns: cols, Rows: [][]any{}}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		for i, v := range vals {
			if b, ok := v.([]byte); ok {
				vals[i] = string(b)
			}
		}
		res.Rows = append(res.Rows, vals)
	}
	return res, rows.Err()
}

// Exec führt INSERT/UPDATE/DELETE aus. LastInsertID ist 0, wenn der Treiber
// sie nicht unterstützt (z. B. PostgreSQL).
func Exec(ctx context.Context, q Querier, query string, args ...any) (sdk.ExecResult, error) {
	r, err := q.ExecContext(ctx, query, normalizeArgs(args)...)
	if err != nil {
		return sdk.ExecResult{}, err
	}
	n, _ := r.RowsAffected()
	id, _ := r.LastInsertId()
	return sdk.ExecResult{RowsAffected: n, LastInsertID: id}, nil
}

// normalizeArgs wandelt ganzzahlige float64 (so kommen Zahlen über
// google.protobuf.Value an) in int64, damit Treiber sie als INTEGER binden.
func normalizeArgs(args []any) []any {
	out := make([]any, len(args))
	for i, a := range args {
		if f, ok := a.(float64); ok && f == math.Trunc(f) && math.Abs(f) <= 1<<53 {
			out[i] = int64(f)
			continue
		}
		out[i] = a
	}
	return out
}

// Rebind wandelt ?-Platzhalter für PostgreSQL-Treiber in $1, $2, … um.
// Einfache Variante: ? innerhalb von String-Literalen wird nicht erkannt.
func Rebind(driver, query string) string {
	if driver != "pgx" && driver != "postgres" {
		return query
	}
	var b strings.Builder
	n := 0
	for _, r := range query {
		if r == '?' {
			n++
			b.WriteString("$" + strconv.Itoa(n))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
