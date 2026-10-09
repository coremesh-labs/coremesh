package crud

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// TestReportSeverity: Eine Prüfung meldet je Stufe einen Fehler, eine Warnung
// (gespeichert, Text in _warnings) oder nichts.
func TestReportSeverity(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "w.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE m (code TEXT PRIMARY KEY, value REAL)`); err != nil {
		t.Fatal(err)
	}
	level := SeverityError
	e := &Entity{Object: "M", Title: "Messung", Table: "m", Keys: []string{"code"}, Fields: []Field{
		{Key: "code", Label: "Schlüssel", Type: metamodel.TypeText, Required: true},
		{Key: "value", Label: "Wert", Type: metamodel.TypeNumber},
	}, Validate: func(ctx context.Context, rec, old Record) error {
		if v, _ := rec["value"].(float64); v > 100 {
			return Report(ctx, level, "Wert %.0f über 100", v)
		}
		return nil
	}}
	NewSet(e).Bind(sqlDB{db})
	ctx := sdk.WithHost(context.Background(), grantHost{})
	if _, err := e.Create(ctx, map[string]any{"code": "A", "value": 120.0}); !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("Fehler erwartet: %v", err)
	}
	level = SeverityWarning
	resp, err := e.Create(ctx, map[string]any{"code": "A", "value": 120.0})
	if err != nil {
		t.Fatal(err)
	}
	if w := resp.Payload.(Record)[WarningsField]; !reflect.DeepEqual(w, []string{"Wert 120 über 100"}) {
		t.Fatalf("Warnung: %v", w)
	}
	level = SeverityNone
	resp, err = e.Update(ctx, map[string]any{"id": "A", "data": map[string]any{"value": 130.0}})
	if err != nil || resp.Payload.(Record)[WarningsField] != nil {
		t.Fatalf("keine Prüfung: %v %v", resp.Payload, err)
	}
}
