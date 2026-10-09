package crud

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// TestKeyDate: Ohne „Gültig ab“ gilt der Stichtag des Benutzers (Metadaten
// key_date), ohne Stichtag heute.
func TestKeyDate(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE s (code TEXT, valid_from TEXT, valid_to TEXT)`); err != nil {
		t.Fatal(err)
	}
	e := &Entity{Object: "S", Title: "Scheibe", Table: "s", Keys: []string{"code", "valid_from"}, TimeSlice: true,
		Fields: WithTimeSlice(Field{Key: "code", Label: "Code", Type: metamodel.TypeText, Required: true})}
	NewSet(e).Bind(sqlDB{db})
	base := sdk.WithHost(context.Background(), grantHost{})
	ctx := sdk.WithCall(base, sdk.CallContext{Metadata: map[string]string{sdk.MetaKeyDate: "2026-01-01"}})
	if KeyDate(ctx) != "2026-01-01" || KeyDate(base) != Today() {
		t.Fatalf("KeyDate: %s / %s", KeyDate(ctx), KeyDate(base))
	}
	resp, err := e.Create(ctx, map[string]any{"code": "A"})
	if err != nil {
		t.Fatal(err)
	}
	if v := resp.Payload.(Record)["valid_from"]; v != "2026-01-01" {
		t.Fatalf("Gültig ab: %v", v)
	}
	bad := sdk.WithCall(base, sdk.CallContext{Metadata: map[string]string{sdk.MetaKeyDate: "kein Datum"}})
	if KeyDate(bad) != Today() {
		t.Fatal("ungültiger Stichtag: heute")
	}
}
