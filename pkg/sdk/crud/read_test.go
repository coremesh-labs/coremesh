package crud

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

func readIDs(t *testing.T, res *sdk.QueryResult, col string) []string {
	t.Helper()
	i := slices.Index(res.Columns, col)
	if i < 0 {
		t.Fatalf("Spalte %s fehlt: %v", col, res.Columns)
	}
	var out []string
	for _, r := range res.Rows {
		out = append(out, Str(r[i]))
	}
	return out
}

func TestReadListUsesListAccess(t *testing.T) {
	e, user, system := contracts(t)
	req := sdk.Request{Object: "Contract", Action: "list"}

	// Benutzer: dieselben Datensätze wie in list, Bankverbindung nur in 1000.
	res, end, err := sdk.ReadAll(user, sdk.ReadFunc(e.ReadList), req)
	if err != nil {
		t.Fatal(err)
	}
	if got := readIDs(t, res, "id"); !slices.Equal(got, []string{"c1", "c2"}) || end.Rows != 2 {
		t.Fatalf("Benutzer: %v, %+v", got, end)
	}
	iban := slices.Index(res.Columns, "iban")
	if res.Rows[0][iban] != "DE01" || res.Rows[1][iban] != nil {
		t.Fatalf("Feldgruppe bank: %v / %v", res.Rows[0][iban], res.Rows[1][iban])
	}
	if !slices.Equal(res.Columns, e.Columns()) {
		t.Fatalf("Spalten: %v", res.Columns)
	}

	// Suche wie in list, im Format der Oberfläche ({"query": …}).
	e.Search = []string{"property"}
	res, _, err = sdk.ReadAll(system, sdk.ReadFunc(e.ReadList), sdk.Request{Payload: map[string]any{"query": map[string]any{"q": "p2"}}})
	if err != nil || !slices.Equal(readIDs(t, res, "id"), []string{"c3"}) {
		t.Fatalf("Suche: %v %v", err, res.Rows)
	}
}

// ledgerItems: zusammengesetzter Schlüssel, n Zeilen – Paging über mehrere Seiten.
func ledgerItems(t *testing.T, n int) (*Entity, context.Context) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "l.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE item (company_code TEXT, doc TEXT, line INTEGER, amount INTEGER, PRIMARY KEY (company_code, doc, line))`); err != nil {
		t.Fatal(err)
	}
	tx, _ := db.Begin()
	for i := range n {
		// Einfügen in gemischter Reihenfolge; gelesen wird nach dem Schlüssel.
		j := (i * 7919) % n
		tx.Exec(`INSERT INTO item VALUES (?, ?, ?, ?)`, []string{"1000", "2000"}[j%2], fmt.Sprintf("D%05d", j/3), j%3+1, j*100)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	e := &Entity{Object: "JournalEntryItem", Title: "Einzelposten", Table: "item", Keys: []string{"company_code", "doc", "line"},
		Fields: []Field{
			{Key: "company_code", Label: "Buchungskreis", Type: metamodel.TypeText},
			{Key: "doc", Label: "Beleg", Type: metamodel.TypeText},
			{Key: "line", Label: "Position", Type: metamodel.TypeNumber},
			{Key: "amount", Label: "Betrag", Type: metamodel.TypeNumber},
		},
	}
	NewSet(e).Bind(sqlDB{db})
	return e, sdk.WithHost(context.Background(), grantHost{})
}

func TestReadListPagesAndResumes(t *testing.T) {
	const n = 2500 // > 2 Seiten zu readPageSize
	e, ctx := ledgerItems(t, n)
	var w countRows
	end, err := e.ReadList(ctx, sdk.Request{}, &sdk.StrictWriter{W: &w})
	if err != nil {
		t.Fatal(err)
	}
	if end.Rows != n || len(w.keys) != n || end.Cursor != "" || w.blocks != 3 {
		t.Fatalf("Ende %+v, %d Zeilen, %d Blöcke", end, len(w.keys), w.blocks)
	}
	if !slices.IsSorted(w.keys) || len(slices.Compact(slices.Clone(w.keys))) != n {
		t.Fatal("Zeilen nicht eindeutig oder nicht nach dem Schlüssel sortiert")
	}

	// limit + after: in Stücken zu 1100 lesen ergibt dieselben Zeilen.
	var all []string
	after := ""
	for range 10 {
		var part countRows
		end, err := e.ReadList(ctx, sdk.Request{Payload: map[string]any{"limit": float64(1100), "after": after}}, &sdk.StrictWriter{W: &part})
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, part.keys...)
		if end.Cursor == "" {
			break
		}
		after = end.Cursor
	}
	if !slices.Equal(all, w.keys) {
		t.Fatalf("fortgesetzt: %d Zeilen statt %d", len(all), n)
	}

	// Genau auf der Grenze: limit = Anzahl → kein Cursor.
	end, err = e.ReadList(ctx, sdk.Request{Payload: map[string]any{"limit": n}}, &sdk.StrictWriter{W: &countRows{}})
	if err != nil || end.Cursor != "" {
		t.Fatalf("limit = n: %v %+v", err, end)
	}
	for _, bad := range []any{"x", -1} {
		if _, err := e.ReadList(ctx, sdk.Request{Payload: map[string]any{"limit": bad}}, &sdk.StrictWriter{W: &countRows{}}); err == nil {
			t.Errorf("limit %v: Fehler erwartet", bad)
		}
	}
}

// countRows merkt sich die Schlüssel der Zeilen (company_code|doc|line).
type countRows struct {
	blocks int
	keys   []string
}

func (c *countRows) Header(sdk.ReadHeader) error { return nil }
func (c *countRows) Rows(rows [][]any) error {
	c.blocks++
	for _, r := range rows {
		c.keys = append(c.keys, fmt.Sprintf("%s|%s|%v", r[0], r[1], r[2]))
	}
	return nil
}
