package dbschema

import (
	"context"
	"database/sql"
	"fmt"
	"maps"
	"slices"
	"strings"

	"ariga.io/atlas/sql/schema"

	"github.com/camel/coremesh/internal/database"
	"github.com/camel/coremesh/pkg/sdk"
)

// applySeeds fügt die Stammdaten eines Moduls ein – in der Transaktion der
// Migration und nur, wenn der Primärschlüssel noch nicht existiert
// (INSERT … ON CONFLICT DO NOTHING; SQLite ≥ 3.24 und PostgreSQL).
//
// Schutzprüfungen wie beim Schema:
//   - nur Tabellen aus dem Soll-Schema des Moduls (also mit Modul-Präfix bzw.
//     im Modul-Schema),
//   - nur Spalten dieser Tabellen; Tabellen- und Spaltennamen kommen aus dem
//     geprüften Schema, Werte werden immer gebunden,
//   - jede Zeile enthält den vollständigen Primärschlüssel.
func applySeeds(ctx context.Context, tx *sql.Tx, driver string, d dialect, sc scope, desired *schema.Schema, seeds []sdk.SchemaSeed) (int64, error) {
	var inserted int64
	for _, s := range seeds {
		t, ok := desired.Table(s.Table)
		if !ok || !sc.ownsTable(t) {
			return inserted, fmt.Errorf("%w: Seed für %q: keine Tabelle des Moduls %s", sdk.ErrPermissionDenied, s.Table, sc.module)
		}
		if t.PrimaryKey == nil || len(t.PrimaryKey.Parts) == 0 {
			return inserted, fmt.Errorf("%w: Seed für %s: Tabelle braucht einen Primärschlüssel", sdk.ErrInvalidArgument, t.Name)
		}
		table := quoteIdent(t.Name)
		if sc.prefix == "" && d.schemas {
			table = quoteIdent(sc.schema) + "." + table
		}
		for i, row := range s.Rows {
			cols := slices.Sorted(maps.Keys(row))
			for _, c := range cols {
				if _, ok := t.Column(c); !ok {
					return inserted, fmt.Errorf("%w: Seed %s[%d]: unbekannte Spalte %q", sdk.ErrInvalidArgument, t.Name, i, c)
				}
			}
			for _, p := range t.PrimaryKey.Parts {
				if p.C == nil || row[p.C.Name] == nil {
					return inserted, fmt.Errorf("%w: Seed %s[%d]: Primärschlüssel unvollständig", sdk.ErrInvalidArgument, t.Name, i)
				}
			}
			quoted := make([]string, len(cols))
			marks := make([]string, len(cols))
			args := make([]any, len(cols))
			for j, c := range cols {
				quoted[j], marks[j], args[j] = quoteIdent(c), "?", row[c]
			}
			query := database.Rebind(driver, fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT DO NOTHING",
				table, strings.Join(quoted, ", "), strings.Join(marks, ", ")))
			res, err := tx.ExecContext(ctx, query, args...)
			if err != nil {
				return inserted, fmt.Errorf("Seed %s[%d]: %w", t.Name, i, err)
			}
			n, _ := res.RowsAffected()
			inserted += n
		}
	}
	return inserted, nil
}

func quoteIdent(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
