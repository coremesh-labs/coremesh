package crud

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// TestInsertColumnDefault: Ein Feld ohne Wert bekommt den Standardwert der
// Spalte (NOT NULL DEFAULT 0), statt mit NULL zu scheitern.
func TestInsertColumnDefault(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "d.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE cat (code TEXT PRIMARY KEY, name TEXT, sort_order INTEGER NOT NULL DEFAULT 0)`); err != nil {
		t.Fatal(err)
	}
	e := &Entity{Object: "Cat", Title: "Katalog", Table: "cat", Keys: []string{"code"}, Fields: []Field{
		{Key: "code", Label: "Schlüssel", Type: metamodel.TypeText, Required: true},
		{Key: "name", Label: "Bezeichnung", Type: metamodel.TypeText},
		{Key: "sort_order", Label: "Reihenfolge", Type: metamodel.TypeNumber},
	}}
	NewSet(e).Bind(sqlDB{db})
	ctx := sdk.WithHost(context.Background(), grantHost{})
	if _, err := e.Create(ctx, map[string]any{"code": "A", "name": "ohne Reihenfolge"}); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(`SELECT sort_order FROM cat WHERE code = 'A'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("sort_order %d %v", n, err)
	}
}
