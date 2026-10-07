package dbschema

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/coremesh-lab/coremesh/internal/config"
	"github.com/coremesh-lab/coremesh/internal/database"
	"github.com/coremesh-lab/coremesh/pkg/sdk"
)

func setup(t *testing.T, settings map[string]any) (*Plugin, *database.Manager) {
	t.Helper()
	db, err := database.Open(context.Background(), map[string]config.Database{
		"main": {Driver: "sqlite", DSN: "file:" + filepath.Join(t.TempDir(), "s.db")},
	}, database.Options{}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	p := New(db)
	if err := p.Configure(context.Background(), sdk.Config{Settings: settings}); err != nil {
		t.Fatal(err)
	}
	return p, db
}

func handle(p *Plugin, action string, payload any) (map[string]any, error) {
	resp, err := p.Handle(context.Background(), sdk.Request{Object: sdk.ObjectDBSchema, Action: action, Payload: payload})
	if err != nil {
		return nil, err
	}
	m, _ := resp.Payload.(map[string]any)
	return m, nil
}

func mustHandle(t *testing.T, p *Plugin, action string, payload any) map[string]any {
	t.Helper()
	m, err := handle(p, action, payload)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func activate(p *Plugin, module, version, hcl string) (map[string]any, error) {
	return handle(p, "Activate", map[string]any{"module": module, "version": version, "schema": hcl})
}

func columns(t *testing.T, db *database.Manager, table string) []string {
	t.Helper()
	pool, _ := db.DB("main")
	res, err := database.Query(context.Background(), pool, `SELECT name FROM pragma_table_info(?) ORDER BY cid`, table)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, r := range res.Rows {
		out = append(out, r[0].(string))
	}
	return out
}

func exec(t *testing.T, db *database.Manager, stmt string) {
	t.Helper()
	pool, _ := db.DB("main")
	if _, err := pool.Exec(stmt); err != nil {
		t.Fatal(err)
	}
}

// Schema-Bausteine (Atlas-HCL).
const partnerV1 = `
schema "main" {}
table "partner__business_partner" {
  schema = schema.main
  column "id"   { type = text }
  column "name" { type = text }
  primary_key { columns = [column.id] }
  index "partner__bp_name" { columns = [column.name] }
}
`

// v2: zusätzliche Spalte und Tabelle mit Fremdschlüssel auf eigene Tabelle.
const partnerV2 = `
schema "main" {}
table "partner__business_partner" {
  schema = schema.main
  column "id"    { type = text }
  column "name"  { type = text }
  column "email" {
    type = text
    null = true
  }
  primary_key { columns = [column.id] }
  index "partner__bp_name" { columns = [column.name] }
}
table "partner__address" {
  schema = schema.main
  column "id"         { type = integer }
  column "partner_id" { type = text }
  primary_key { columns = [column.id] }
  foreign_key "partner__address_partner" {
    columns     = [column.partner_id]
    ref_columns = [table.partner__business_partner.column.id]
  }
}
`

func TestModuleSchemaLifecycle(t *testing.T) {
	p, db := setup(t, nil)
	mod := map[string]any{"module": "partner", "version": "1.0.0"}

	if got := mustHandle(t, p, "CheckVersion", mod); got["migrated"] != false {
		t.Fatalf("vor Activate: %v", got)
	}
	got, err := activate(p, "partner", "1.0.0", partnerV1)
	if err != nil {
		t.Fatal(err)
	}
	if got["applied"] != true || len(got["statements"].([]string)) == 0 {
		t.Fatalf("v1: %v", got)
	}
	if got := mustHandle(t, p, "CheckVersion", mod); got["migrated"] != true {
		t.Fatalf("nach Activate: %v", got)
	}
	if got, _ := activate(p, "partner", "1.0.0", partnerV1); got["applied"] != false {
		t.Fatalf("gleiche Version erneut: %v", got)
	}

	// Additive Änderung: erlaubt.
	got, err = activate(p, "partner", "1.1.0", partnerV2)
	if err != nil {
		t.Fatal(err)
	}
	if cols := columns(t, db, "partner__business_partner"); !slices.Contains(cols, "email") {
		t.Fatalf("Spalte email fehlt: %v (%v)", cols, got)
	}
	if cols := columns(t, db, "partner__address"); len(cols) != 2 {
		t.Fatalf("partner__address: %v", cols)
	}
}

func TestDestructiveChangesRejected(t *testing.T) {
	p, db := setup(t, nil)
	if _, err := activate(p, "partner", "1.1.0", partnerV2); err != nil {
		t.Fatal(err)
	}

	cases := map[string]string{
		"DROP COLUMN": partnerV1 + `
table "partner__address" {
  schema = schema.main
  column "id"         { type = integer }
  column "partner_id" { type = text }
  primary_key { columns = [column.id] }
  foreign_key "partner__address_partner" {
    columns     = [column.partner_id]
    ref_columns = [table.partner__business_partner.column.id]
  }
}`, // email fehlt
		"DROP TABLE": strings.Replace(partnerV2, `table "partner__address"`, `table "partner__address_renamed"`, 1),
	}
	for name, hcl := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := activate(p, "partner", "2.0.0-"+strings.ReplaceAll(name, " ", ""), hcl)
			if !errors.Is(err, ErrDestructive) || !strings.Contains(err.Error(), name) {
				t.Fatalf("%s muss abgelehnt werden: %v", name, err)
			}
		})
	}
	// Nichts verändert, nichts registriert.
	if cols := columns(t, db, "partner__business_partner"); !slices.Contains(cols, "email") {
		t.Fatalf("Spalte email wurde gelöscht: %v", cols)
	}
	if got := mustHandle(t, p, "CheckVersion", map[string]any{"module": "partner", "version": "2.0.0-DROPCOLUMN"}); got["migrated"] != false {
		t.Fatalf("abgelehnte Version registriert: %v", got)
	}
}

func TestOtherModulesAreInvisible(t *testing.T) {
	p, db := setup(t, nil)
	// Tabellen anderer Module, inkl. Präfix-Ähnlichkeit (partner_service__ vs. partner__).
	exec(t, db, `CREATE TABLE orders__sales_order (id TEXT PRIMARY KEY)`)
	exec(t, db, `CREATE TABLE partner_service__contact (id TEXT PRIMARY KEY)`)
	exec(t, db, `CREATE TABLE partnerx__thing (id TEXT PRIMARY KEY)`)

	// Das Modul kennt diese Tabellen nicht – Atlas darf sie dennoch nicht droppen.
	got, err := activate(p, "partner", "1.0.0", partnerV1)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range got["statements"].([]string) {
		if !strings.Contains(stmt, "partner__") {
			t.Errorf("Statement außerhalb des Moduls: %s", stmt)
		}
	}
	for _, tbl := range []string{"orders__sales_order", "partner_service__contact", "partnerx__thing"} {
		if len(columns(t, db, tbl)) == 0 {
			t.Errorf("Tabelle %s wurde verändert/gelöscht", tbl)
		}
	}
}

func TestIsolationViolationsRejected(t *testing.T) {
	p, db := setup(t, nil)
	exec(t, db, `CREATE TABLE orders__sales_order (id TEXT PRIMARY KEY)`)

	cases := map[string]string{
		"fremde Tabelle": `
schema "main" {}
table "orders__sales_order" {
  schema = schema.main
  column "id" { type = text }
  column "hacked" {
    type = text
    null = true
  }
}`,
		"ohne Präfix": `
schema "main" {}
table "business_partner" {
  schema = schema.main
  column "id" { type = text }
}`,
		"Index ohne Präfix": `
schema "main" {}
table "partner__bp" {
  schema = schema.main
  column "id" { type = text }
  index "idx_id" { columns = [column.id] }
}`,
		"Fremdschlüssel auf anderes Modul": `
schema "main" {}
table "orders__sales_order" {
  schema = schema.main
  column "id" { type = text }
}
table "partner__bp" {
  schema = schema.main
  column "order_id" { type = text }
  foreign_key "partner__bp_order" {
    columns     = [column.order_id]
    ref_columns = [table.orders__sales_order.column.id]
  }
}`,
	}
	for name, hcl := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := activate(p, "partner", "1.0.0", hcl); !errors.Is(err, sdk.ErrPermissionDenied) {
				t.Fatalf("muss abgelehnt werden: %v", err)
			}
		})
	}
	if cols := columns(t, db, "orders__sales_order"); len(cols) != 1 {
		t.Fatalf("fremde Tabelle verändert: %v", cols)
	}

	if _, err := activate(p, "partner", "1.0.0", `das ist kein HCL {`); !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("ungültiges HCL: %v", err)
	}
	if _, err := activate(p, HostModule, "9", partnerV1); !errors.Is(err, sdk.ErrPermissionDenied) {
		t.Fatalf("reservierter Modulname: %v", err)
	}
}

func TestBootstrapHostMigrations(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "migrations")
	os.Mkdir(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "0002_b.sql"), []byte("CREATE TABLE coremesh__b (id INTEGER);"), 0o644)
	os.WriteFile(filepath.Join(dir, "0001_a.sql"), []byte("CREATE TABLE coremesh__a (id INTEGER);"), 0o644)
	p, _ := setup(t, map[string]any{"migrations_dir": dir})

	first := mustHandle(t, p, "Activate", nil)
	if applied := first["applied"].([]any); len(applied) != 2 || applied[0] != "0001_a" {
		t.Fatalf("erster Lauf: %v", first)
	}
	if second := mustHandle(t, p, "Activate", nil); len(second["applied"].([]any)) != 0 {
		t.Fatalf("zweiter Lauf: %v", second)
	}
}

const seedSchema = `
schema "main" {}
table "partner__comm_categories" {
  schema = schema.main
  column "code"        { type = text }
  column "description" { type = text }
  primary_key { columns = [column.code] }
}
table "partner__comm_types" {
  schema = schema.main
  column "code"          { type = text }
  column "category_code" { type = text }
  column "is_main" {
    type    = boolean
    default = false
  }
  primary_key { columns = [column.code] }
  foreign_key "partner__comm_types_cat" {
    columns     = [column.category_code]
    ref_columns = [table.partner__comm_categories.column.code]
  }
}
`

func seedActivate(p *Plugin, version string, seed []map[string]any) (map[string]any, error) {
	return handle(p, "Activate", map[string]any{"module": "partner", "version": version, "schema": seedSchema, "seed": seed})
}

func TestSeeds(t *testing.T) {
	p, db := setup(t, nil)
	seed := []map[string]any{
		{"table": "partner__comm_categories", "rows": []any{
			map[string]any{"code": "EMAIL", "description": "E-Mail"},
			map[string]any{"code": "PHONE", "description": "Telefon"},
		}},
		{"table": "partner__comm_types", "rows": []any{
			map[string]any{"code": "EMAIL_WORK", "category_code": "EMAIL", "is_main": true},
		}},
	}
	got, err := seedActivate(p, "1.0.0", seed)
	if err != nil {
		t.Fatal(err)
	}
	if got["seeded"] != int64(3) {
		t.Fatalf("seeded: %v", got["seeded"])
	}

	// Geänderte Beschreibung im Bestand bleibt, neue Zeile kommt dazu (nur fehlende Schlüssel).
	exec(t, db, `UPDATE partner__comm_categories SET description = 'Mail (geändert)' WHERE code = 'EMAIL'`)
	seed[0]["rows"] = append(seed[0]["rows"].([]any), map[string]any{"code": "WEB", "description": "Webseite"})
	got, err = seedActivate(p, "1.1.0", seed)
	if err != nil || got["seeded"] != int64(1) {
		t.Fatalf("zweiter Lauf: %v %v", got, err)
	}
	pool, _ := db.DB("main")
	res, _ := database.Query(context.Background(), pool, `SELECT description FROM partner__comm_categories WHERE code = 'EMAIL'`)
	if res.Rows[0][0] != "Mail (geändert)" {
		t.Fatalf("Bestand überschrieben: %v", res.Rows)
	}

	for name, bad := range map[string][]map[string]any{
		"fremde Tabelle":    {{"table": "iam__users", "rows": []any{map[string]any{"id": "x"}}}},
		"unbekannte Spalte": {{"table": "partner__comm_categories", "rows": []any{map[string]any{"code": "X", "boese": 1}}}},
		"ohne Schlüssel":    {{"table": "partner__comm_categories", "rows": []any{map[string]any{"description": "x"}}}},
	} {
		if _, err := seedActivate(p, "9."+name, bad); err == nil {
			t.Errorf("%s: muss abgelehnt werden", name)
		}
	}
}
