package crud

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// Wieder aktivieren: bei ja/nein-Status als Aktion, Status wieder aktiv;
// nicht bei Auswahl-Status, eigener Ende-Logik (OnEnd) oder NoReactivate.
func TestReactivate(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "r.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE acc (code TEXT PRIMARY KEY, name TEXT, is_active BOOLEAN DEFAULT 1)`); err != nil {
		t.Fatal(err)
	}
	fields := func() []Field {
		return []Field{{Key: "code", Label: "Code", Type: metamodel.TypeText, Required: true},
			{Key: "name", Label: "Name", Type: metamodel.TypeText},
			{Key: "is_active", Label: "Aktiv", Type: metamodel.TypeBoolean, ReadOnly: true}}
	}
	e := &Entity{Object: "Acc", Title: "Konto", Table: "acc", Keys: []string{"code"}, StatusField: "is_active", Fields: fields()}
	NewSet(e).Bind(sqlDB{db})
	offered := func(e *Entity) bool {
		for _, a := range e.Definition().Actions {
			if a.Name == ReactivateAction {
				return a.Record
			}
		}
		return false
	}
	if !offered(e) {
		t.Fatal("Wieder aktivieren fehlt")
	}
	ctx := sdk.WithHost(context.Background(), grantHost{})
	if _, err := e.Create(ctx, map[string]any{"code": "A", "name": "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Deactivate(ctx, map[string]any{"id": "A"}); err != nil {
		t.Fatal(err)
	}
	resp, err := e.Reactivate(ctx, map[string]any{"id": "A"})
	if err != nil {
		t.Fatal(err)
	}
	if rec := resp.Payload.(Record); !AsBool(rec["is_active"]) {
		t.Fatalf("nicht aktiv: %v", rec)
	}
	for name, other := range map[string]*Entity{
		"Auswahl":      {StatusField: "is_active", StatusActive: "ACTIVE", StatusInactive: "DEPRECATED"},
		"OnEnd":        {StatusField: "is_active", OnEnd: func(context.Context, string, Record) (map[string]any, error) { return nil, nil }},
		"NoReactivate": {StatusField: "is_active", NoReactivate: true},
	} {
		other.Object, other.Title, other.Table, other.Keys, other.Fields = "X"+name, name, "acc", []string{"code"}, fields()
		NewSet(other)
		if offered(other) {
			t.Errorf("%s: Wieder aktivieren angeboten", name)
		}
		if _, err := other.Reactivate(ctx, map[string]any{"id": "A"}); err == nil {
			t.Errorf("%s: Reactivate ohne Fehler", name)
		}
	}
}
