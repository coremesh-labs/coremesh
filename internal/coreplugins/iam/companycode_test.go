package iam

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/camel/coremesh/internal/config"
	"github.com/camel/coremesh/internal/coreplugins/dbschema"
	"github.com/camel/coremesh/internal/database"
	"github.com/camel/coremesh/pkg/sdk"
)

func TestParseCompanyCodes(t *testing.T) {
	perms, err := parsePermissions("Partner.*@1000, 2000\n*.list@*\nGreeting.say\nPartner.*@1000")
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		o, a, ccs string
	}
	var got []row
	for _, g := range perms {
		got = append(got, row{g.Object, g.Action, strings.Join(g.CompanyCodes, ",")})
	}
	want := []row{{"Partner", "*", "1000,2000"}, {"*", "list", AllCompanyCodes}, {"Greeting", "say", AllCompanyCodes}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v", got)
	}
	if got := formatPermissions(perms); !slices.Equal(got, []string{"Partner.*@1000,2000", "*.list", "Greeting.say"}) {
		t.Fatalf("format: %v", got)
	}
	for _, bad := range []string{"Partner.*@", "Partner.*@10 00", "Partner.*@a/b"} {
		if _, err := parsePermissions(bad); err == nil {
			t.Errorf("%q muss abgelehnt werden", bad)
		}
	}
}

func TestCompanyCodeScopedAccess(t *testing.T) {
	p, adminID := setup(t)
	ctx := as(adminID)

	for _, cc := range []string{"1000", "2000", "3000"} {
		if _, err := call(t, p, ctx, "CompanyCode", "create", map[string]any{"data": map[string]any{"code": cc, "description": "BK " + cc}}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := call(t, p, ctx, "Role", "create", map[string]any{"data": map[string]any{
		"name": "Partner Nord", "permissions": "Partner.*@1000,2000\nPartner.list"}}); err != nil {
		t.Fatal(err)
	}
	u, err := call(t, p, ctx, "User", "create", map[string]any{"data": map[string]any{
		"username": "nord", "password": "nord-passwort-1", "roles": "Partner Nord"}})
	if err != nil {
		t.Fatal(err)
	}
	uid := u["id"].(string)
	asNord := as(uid)

	check := func(object, action, cc string) bool {
		m, err := call(t, p, asNord, "Account", "Check", map[string]any{"object": object, "action": action, "company_code": cc})
		if err != nil {
			t.Fatal(err)
		}
		return m["allowed"].(bool)
	}
	if !check("Partner", "update", "1000") || !check("Partner", "update", "2000") || check("Partner", "update", "3000") {
		t.Fatal("update ist auf 1000 und 2000 beschränkt")
	}
	if !check("Partner", "list", "3000") {
		t.Fatal("Partner.list ohne @ gilt in allen Buchungskreisen")
	}
	if check("Contract", "get", "1000") {
		t.Fatal("ohne Berechtigung")
	}

	granted := func(object, action string) sdk.GrantSet {
		resp, err := p.Handle(asNord, sdk.Request{Object: "Account", Action: "Granted", Payload: map[string]any{"object": object, "action": action}})
		if err != nil {
			t.Fatal(err)
		}
		return resp.Payload.(sdk.GrantSet)
	}
	if g := granted("Partner", "update"); g.All || !slices.Equal(g.CompanyCodes, []string{"1000", "2000"}) {
		t.Fatalf("Granted update: %+v", g)
	}
	if g := granted("Partner", "list"); !g.All {
		t.Fatalf("Granted list: %+v", g)
	}
	if g := granted("Contract", "get"); g.All || len(g.CompanyCodes) != 0 {
		t.Fatalf("Granted ohne Recht: %+v", g)
	}

	// Der Dispatcher prüft nur „überhaupt“: update ist (irgendwo) erlaubt.
	if ok, _ := p.Allowed(context.Background(), uid, "Partner", "update"); !ok {
		t.Fatal("Allowed muss ohne Buchungskreis zutreffen")
	}
	// Der Admin (*.* ohne @) darf überall; System-Anfragen ohne Benutzer ebenso.
	if m, _ := call(t, p, ctx, "Account", "Check", map[string]any{"object": "X", "action": "y", "company_code": "3000"}); !m["allowed"].(bool) {
		t.Fatal("Admin")
	}
	if m, _ := call(t, p, context.Background(), "Account", "Check", map[string]any{"object": "X", "action": "y", "company_code": "9"}); !m["allowed"].(bool) {
		t.Fatal("System-Anfrage")
	}

	// Rolle zeigt die zusammengefasste Schreibweise.
	roles, _ := call(t, p, ctx, "Role", "list", nil)
	var nord map[string]any
	for _, r := range roles["items"].([]any) {
		if r.(map[string]any)["name"] == "Partner Nord" {
			nord = r.(map[string]any)
		}
	}
	if nord["permissions"] != "Partner.*@1000,2000\nPartner.list" {
		t.Fatalf("Darstellung: %q", nord["permissions"])
	}
}

func TestCompanyCodeIntegrity(t *testing.T) {
	p, adminID := setup(t)
	ctx := as(adminID)
	call(t, p, ctx, "CompanyCode", "create", map[string]any{"data": map[string]any{"code": "1000", "description": "Zürich"}})

	if _, err := call(t, p, ctx, "Role", "create", map[string]any{"data": map[string]any{"name": "R1", "permissions": "Partner.*@9999"}}); !errors.Is(err, sdk.ErrInvalidArgument) || !strings.Contains(err.Error(), "9999") {
		t.Fatalf("unbekannter Buchungskreis: %v", err)
	}
	if _, err := call(t, p, ctx, "Role", "create", map[string]any{"data": map[string]any{"name": "R2", "permissions": "Partner.*@1000"}}); err != nil {
		t.Fatal(err)
	}
	// Buchungskreise sind immutable: kein Löschen, keine Deaktivierung.
	if _, err := call(t, p, ctx, "CompanyCode", "delete", map[string]any{"id": "1000"}); !errors.Is(err, sdk.ErrUnimplemented) {
		t.Fatalf("Buchungskreis löschen: %v", err)
	}
	for name, data := range map[string]map[string]any{
		"doppelt":  {"code": "1000"},
		"ungültig": {"code": "10 00"},
		"Stern":    {"code": "*"},
		"zu lang":  {"code": strings.Repeat("1", 21)},
	} {
		if _, err := call(t, p, ctx, "CompanyCode", "create", map[string]any{"data": data}); !errors.Is(err, sdk.ErrInvalidArgument) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := call(t, p, ctx, "CompanyCode", "update", map[string]any{"id": "1000", "data": map[string]any{"code": "2000", "description": "x"}}); !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("Nummer ändern: %v", err)
	}
	if m, err := call(t, p, ctx, "CompanyCode", "update", map[string]any{"id": "1000", "data": map[string]any{"code": "1000", "description": "Zürich Nord"}}); err != nil || m["description"] != "Zürich Nord" {
		t.Fatalf("Beschreibung ändern: %v %v", m, err)
	}
}

// schemaV010 ist das Schema von iam 0.1.0 (vor den Buchungskreisen).
const schemaV010 = `
schema "main" {}
table "iam__roles" {
  schema = schema.main
  column "id" { type = text }
  column "name" { type = text }
  column "description" {
    type = text
    null = true
  }
  column "created_at" { type = text }
  primary_key { columns = [column.id] }
}
table "iam__role_permissions" {
  schema = schema.main
  column "role_id" { type = text }
  column "object" { type = text }
  column "action" { type = text }
  primary_key { columns = [column.role_id, column.object, column.action] }
  foreign_key "iam__role_permissions_role" {
    columns     = [column.role_id]
    ref_columns = [table.iam__roles.column.id]
    on_delete   = CASCADE
  }
}
`

// TestUpgradeFrom010: Bestehende Berechtigungen gelten nach dem Upgrade in
// allen Buchungskreisen ("*"), es geht nichts verloren.
func TestUpgradeFrom010(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, map[string]config.Database{
		"main": {Driver: "sqlite", DSN: "file:" + filepath.Join(t.TempDir(), "up.db")},
	}, database.Options{}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	schema := dbschema.New(db)
	schema.Configure(ctx, sdk.Config{})
	activate := func(version, hcl string) {
		t.Helper()
		if _, err := schema.Handle(ctx, sdk.Request{Object: "DBSchema", Action: "Activate",
			Payload: map[string]any{"module": Name, "version": version, "schema": hcl}}); err != nil {
			t.Fatalf("%s: %v", version, err)
		}
	}
	activate("0.1.0", schemaV010)
	pool, _ := db.DB("main")
	pool.Exec(`INSERT INTO iam__roles (id, name, created_at) VALUES ('r1', 'Leser', 'x')`)
	pool.Exec(`INSERT INTO iam__role_permissions (role_id, object, action) VALUES ('r1', 'Greeting', 'list')`)

	activate("0.2.0", schemaHCL) // Spalte company_code + neuer Primärschlüssel + neue Tabellen

	res, err := database.Query(ctx, pool, `SELECT object, action, company_code FROM iam__role_permissions`)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 1 || res.Rows[0][2] != "*" {
		t.Fatalf("nach Upgrade: %v", res.Rows)
	}

	// 0.5.0: einmalige Übernahme nach iam__role_auth (je Object.Action eine Zeile).
	pool.Exec(`INSERT INTO iam__company_codes (id, created_at) VALUES ('1000', 'x'), ('2000', 'x')`)
	pool.Exec(`INSERT INTO iam__role_permissions (role_id, object, action, company_code) VALUES ('r1', 'Partner', '*', '1000'), ('r1', 'Partner', '*', '2000')`)
	p := New(db)
	p.settings = settings{Database: "main"}
	for i, want := range []int{2, 0} { // zweiter Lauf: nichts mehr zu tun
		n, err := p.migrateLegacy(ctx)
		if err != nil || n != want {
			t.Fatalf("Lauf %d: %d, %v", i, n, err)
		}
	}
	roles, err := p.grantsByRole(ctx, pool, "r1")
	if err != nil {
		t.Fatal(err)
	}
	if got := formatPermissions(roles["r1"]); !slices.Equal(got, []string{"Greeting.list", "Partner.*@1000,2000"}) {
		t.Fatalf("übernommen: %v", got)
	}
}
