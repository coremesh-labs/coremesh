package dbschema

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"testing"

	"github.com/coremesh-lab/coremesh/internal/config"
	"github.com/coremesh-lab/coremesh/internal/database"
	"github.com/coremesh-lab/coremesh/internal/testutil/pgtest"
	"github.com/coremesh-lab/coremesh/pkg/sdk"
)

func setupPG(t *testing.T, isolation string) (*Plugin, *database.Manager) {
	t.Helper()
	db, err := database.Open(context.Background(), map[string]config.Database{
		"main": {Driver: "pgx", DSN: pgtest.New(t)},
	}, database.Options{}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	p := New(db)
	if err := p.Configure(context.Background(), sdk.Config{Settings: map[string]any{"isolation": isolation}}); err != nil {
		t.Fatal(err)
	}
	return p, db
}

// pgTables liefert "schema.tabelle" aller Benutzer-Tabellen.
func pgTables(t *testing.T, db *database.Manager) []string {
	t.Helper()
	pool, _ := db.DB("main")
	res, err := database.Query(context.Background(), pool, `
		SELECT table_schema || '.' || table_name FROM information_schema.tables
		WHERE table_schema NOT IN ('pg_catalog', 'information_schema') ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, r := range res.Rows {
		out = append(out, r[0].(string))
	}
	return out
}

func pgColumns(t *testing.T, db *database.Manager, schemaName, table string) []string {
	t.Helper()
	pool, _ := db.DB("main")
	res, err := database.Query(context.Background(), pool,
		`SELECT column_name FROM information_schema.columns WHERE table_schema = $1 AND table_name = $2 ORDER BY ordinal_position`,
		schemaName, table)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, r := range res.Rows {
		out = append(out, r[0].(string))
	}
	return out
}

// Im Schema-Modus brauchen Namen kein Präfix: Das DB-Schema trennt die Module.
const pgPartnerV1 = `
schema "main" {}
table "business_partner" {
  schema = schema.main
  column "id"   { type = text }
  column "name" { type = text }
  primary_key { columns = [column.id] }
  index "bp_name" { columns = [column.name] }
}
`

const pgPartnerV2 = `
schema "main" {}
table "business_partner" {
  schema = schema.main
  column "id"    { type = text }
  column "name"  { type = text }
  column "email" {
    type = text
    null = true
  }
  primary_key { columns = [column.id] }
  index "bp_name" { columns = [column.name] }
}
table "address" {
  schema = schema.main
  column "id"         { type = integer }
  column "partner_id" { type = text }
  primary_key { columns = [column.id] }
  foreign_key "address_partner" {
    columns     = [column.partner_id]
    ref_columns = [table.business_partner.column.id]
  }
}
`

func TestPostgresSchemaIsolation(t *testing.T) {
	p, db := setupPG(t, IsolationSchema)

	// Zwei Module mit gleichen Tabellennamen – jedes im eigenen Schema.
	for _, mod := range []string{"partner", "partner-service"} {
		got, err := activate(p, mod, "1.0.0", pgPartnerV1)
		if err != nil {
			t.Fatalf("%s: %v", mod, err)
		}
		if got["schema"] != sdk.SchemaName(mod) || got["applied"] != true {
			t.Fatalf("%s: %v", mod, got)
		}
	}
	want := []string{
		"mod_partner.business_partner",
		"mod_partner_service.business_partner",
		"public.coremesh_schema_migrations",
	}
	if got := pgTables(t, db); !slices.Equal(got, want) {
		t.Fatalf("Tabellen:\n got %v\nwant %v", got, want)
	}

	// Additive Migration nur im eigenen Schema.
	if _, err := activate(p, "partner", "1.1.0", pgPartnerV2); err != nil {
		t.Fatal(err)
	}
	if cols := pgColumns(t, db, "mod_partner", "business_partner"); !slices.Contains(cols, "email") {
		t.Fatalf("mod_partner: %v", cols)
	}
	if cols := pgColumns(t, db, "mod_partner_service", "business_partner"); slices.Contains(cols, "email") {
		t.Fatalf("fremdes Schema verändert: %v", cols)
	}

	// Destruktiv: v1 würde email und address entfernen.
	_, err := activate(p, "partner", "2.0.0", pgPartnerV1)
	if !errors.Is(err, ErrDestructive) {
		t.Fatalf("DROP muss abgelehnt werden: %v", err)
	}
	if cols := pgColumns(t, db, "mod_partner", "business_partner"); !slices.Contains(cols, "email") {
		t.Fatalf("Rollback fehlt: %v", cols)
	}
}

func TestPostgresPrefixIsolation(t *testing.T) {
	p, db := setupPG(t, IsolationPrefix)
	pool, _ := db.DB("main")
	if _, err := pool.Exec(`CREATE TABLE orders__sales_order (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := activate(p, "partner", "1.0.0", partnerV1); err != nil {
		t.Fatal(err)
	}
	want := []string{"public.coremesh_schema_migrations", "public.orders__sales_order", "public.partner__business_partner"}
	if got := pgTables(t, db); !slices.Equal(got, want) {
		t.Fatalf("Tabellen:\n got %v\nwant %v", got, want)
	}
}

func TestSchemaIsolationRequiresPostgres(t *testing.T) {
	db, err := database.Open(context.Background(), map[string]config.Database{
		"main": {Driver: "sqlite", DSN: "file:" + t.TempDir() + "/x.db"},
	}, database.Options{}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	err = New(db).Configure(context.Background(), sdk.Config{Settings: map[string]any{"isolation": "schema"}})
	if !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("SQLite + isolation: schema: %v", err)
	}
}
