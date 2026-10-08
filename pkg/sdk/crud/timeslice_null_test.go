package crud

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// Leerer Schlüsselteil (NULL in der Tabelle): Laden, Beenden und die Prüfung
// auf Überschneidung finden die Zeitscheibe trotzdem.
func TestEmptyKeyPartTimeSlice(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "n.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE cond (contract TEXT, kind TEXT, object TEXT, valid_from TEXT, valid_to TEXT, amount TEXT)`); err != nil {
		t.Fatal(err)
	}
	e := &Entity{Object: "Cond", Title: "Kondition", Table: "cond", Keys: []string{"contract", "kind", "object", "valid_from"}, TimeSlice: true,
		Fields: WithTimeSlice(
			Field{Key: "contract", Label: "Vertrag", Type: metamodel.TypeText, Required: true},
			Field{Key: "kind", Label: "Art", Type: metamodel.TypeText, Required: true},
			Field{Key: "object", Label: "Objekt (leer = ganzer Vertrag)", Type: metamodel.TypeText},
			Field{Key: "amount", Label: "Betrag", Type: metamodel.TypeText},
		)}
	NewSet(e).Bind(sqlDB{db})
	ctx := sdk.WithHost(context.Background(), grantHost{})
	if _, err := e.Create(ctx, map[string]any{"contract": "V1", "kind": "KM", "amount": "800", "valid_from": "2026-01-01"}); err != nil {
		t.Fatal(err)
	}
	var stored sql.NullString
	_ = db.QueryRow(`SELECT object FROM cond`).Scan(&stored)
	_, err = e.Create(ctx, map[string]any{"contract": "V1", "kind": "KM", "amount": "850", "valid_from": "2026-10-01"})
	if !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("Überschneidung mit leerem Objekt (gespeichert %v): %v", stored, err)
	}
	if _, err := e.Load(ctx, Record{"contract": "V1", "kind": "KM", "object": "", "valid_from": "2026-01-01"}); err != nil {
		t.Fatalf("Laden mit leerem Objekt: %v", err)
	}
}
