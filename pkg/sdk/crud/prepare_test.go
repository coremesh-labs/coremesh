package crud

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// Prepare läuft vor der Transaktion (und vor Validate); ein Fehler bricht ab.
func TestPrepareBeforeTransaction(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "p.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE doc (id TEXT PRIMARY KEY, number TEXT)`); err != nil {
		t.Fatal(err)
	}
	var order []string
	e := &Entity{Object: "Doc", Title: "Beleg", Table: "doc", Keys: []string{"id"},
		Fields: []Field{
			{Key: "id", Label: "ID", Type: metamodel.TypeText, Required: true},
			{Key: "number", Label: "Nummer", Type: metamodel.TypeText, ReadOnly: true},
		},
		Prepare: func(_ context.Context, rec Record) error {
			order = append(order, "prepare")
			if rec["id"] == "fehler" {
				return Invalid("keine Nummer")
			}
			rec["number"] = "N-1"
			return nil
		},
		Validate: func(_ context.Context, rec, _ Record) error {
			order = append(order, "validate:"+Str(rec["number"]))
			return nil
		},
	}
	NewSet(e).Bind(sqlDB{db})
	ctx := sdk.WithHost(context.Background(), grantHost{})
	if _, err := e.Create(ctx, map[string]any{"id": "d1"}); err != nil {
		t.Fatal(err)
	}
	var nr string
	_ = db.QueryRow("SELECT number FROM doc WHERE id = 'd1'").Scan(&nr)
	if nr != "N-1" || !slices.Equal(order, []string{"prepare", "validate:N-1"}) {
		t.Fatalf("Nummer %q, Reihenfolge %v", nr, order)
	}
	if _, err := e.Create(ctx, map[string]any{"id": "fehler"}); !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("Fehler aus Prepare: %v", err)
	}
	var n int
	_ = db.QueryRow("SELECT COUNT(*) FROM doc").Scan(&n)
	if n != 1 {
		t.Fatalf("nach Fehler in Prepare: %d Zeilen", n)
	}
}
