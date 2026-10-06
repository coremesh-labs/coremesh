package businesspartner

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"ariga.io/atlas/sql/schema"
	"ariga.io/atlas/sql/sqlite"
	_ "modernc.org/sqlite"

	"github.com/camel/coremesh/pkg/sdk"
	"github.com/camel/coremesh/pkg/sdk/metamodel"
	"github.com/camel/coremesh/pkg/sdk/module"
)

// testHost: SQLite mit einer einzigen Verbindung (Transaktionen über
// BEGIN/COMMIT wie im Host), iam-Funktionen als Attrappe.
type testHost struct {
	sdk.Host
	db           *sql.DB
	companyCodes []string            // in iam angelegt
	granted      map[string][]string // action → erlaubte Buchungskreise ("*" = alle)
}

func (h *testHost) Log(context.Context, sdk.LogLevel, string, map[string]string) error { return nil }

func (h *testHost) Query(ctx context.Context, _ string, q string, args ...any) (*sdk.QueryResult, error) {
	rows, err := h.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	res := &sdk.QueryResult{Columns: cols}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		for i, v := range vals {
			if b, ok := v.([]byte); ok {
				vals[i] = string(b)
			}
		}
		res.Rows = append(res.Rows, vals)
	}
	return res, rows.Err()
}

func (h *testHost) Exec(ctx context.Context, _ string, q string, args ...any) (sdk.ExecResult, error) {
	r, err := h.db.ExecContext(ctx, q, args...)
	if err != nil {
		return sdk.ExecResult{}, err
	}
	n, _ := r.RowsAffected()
	return sdk.ExecResult{RowsAffected: n}, nil
}

func (h *testHost) BeginTx(ctx context.Context, _ string, _ sql.TxOptions) (string, error) {
	_, err := h.db.ExecContext(ctx, "BEGIN")
	return "tx", err
}
func (h *testHost) CommitTx(ctx context.Context, _ string) error {
	_, err := h.db.ExecContext(ctx, "COMMIT")
	return err
}
func (h *testHost) RollbackTx(ctx context.Context, _ string) error {
	_, err := h.db.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
	return err
}

func (h *testHost) Handle(_ context.Context, req sdk.Request) (sdk.Response, error) {
	p, _ := req.Payload.(map[string]any)
	switch req.Object + "." + req.Action {
	case "CompanyCode.get":
		if slices.Contains(h.companyCodes, str(p["id"])) {
			return sdk.Response{Payload: map[string]any{"id": p["id"]}}, nil
		}
		return sdk.Response{}, sdk.ErrNotFound
	case "Account.Check":
		g := h.granted[str(p["action"])]
		return sdk.Response{Payload: map[string]any{"allowed": slices.Contains(g, "*") || slices.Contains(g, str(p["company_code"]))}}, nil
	case "Account.Granted":
		g := h.granted[str(p["action"])]
		if slices.Contains(g, "*") {
			return sdk.Response{Payload: map[string]any{"all": true, "company_codes": []any{}}}, nil
		}
		ccs := []any{}
		for _, c := range g {
			ccs = append(ccs, c)
		}
		return sdk.Response{Payload: map[string]any{"all": false, "company_codes": ccs}}, nil
	}
	return sdk.Response{}, fmt.Errorf("%w: %s.%s", sdk.ErrUnimplemented, req.Object, req.Action)
}

type env struct {
	t   *testing.T
	p   *module.Plugin
	h   *testHost
	ctx context.Context
}

// setup wendet schemaHCL mit Atlas an und spielt die Seeds ein – wie DBSchema.
func setup(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "p.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	h := &testHost{db: db, companyCodes: []string{"1000", "2000"}, granted: map[string][]string{
		"list": {"*"}, "get": {"*"}, "create": {"*"}, "update": {"*"}, "delete": {"*"},
	}}
	p := newPlugin(t)
	if err := p.Configure(sdk.WithHost(ctx, h), sdk.Config{Host: h}); err != nil {
		t.Fatal(err)
	}
	// Soll-Schema und Seeds so, wie der Host sie über DBSchema.Init abholt.
	resp, err := p.Handle(ctx, sdk.Request{Object: sdk.ObjectDBSchema, Action: sdk.ActionInit, Payload: sdk.SchemaInitRequest{Module: "partner"}})
	if err != nil {
		t.Fatal(err)
	}
	si := resp.Payload.(sdk.SchemaInitResponse)

	drv, err := sqlite.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	var desired schema.Schema
	if err := sqlite.EvalHCLBytes([]byte(si.Schema), &desired, nil); err != nil {
		t.Fatalf("schemaHCL: %v", err)
	}
	desired.Name = "main"
	for _, tb := range desired.Tables {
		tb.Schema = &desired
		if !strings.HasPrefix(tb.Name, "partner__") {
			t.Fatalf("Tabelle ohne Modul-Präfix: %s", tb.Name)
		}
	}
	current, err := drv.InspectSchema(ctx, "main", nil)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := drv.SchemaDiff(current, &desired)
	if err != nil {
		t.Fatal(err)
	}
	if err := drv.ApplyChanges(ctx, changes); err != nil {
		t.Fatal(err)
	}
	for _, s := range si.Seed {
		for _, r := range s.Rows {
			cols := slices.Sorted(maps.Keys(r))
			args := make([]any, len(cols))
			for i, c := range cols {
				args[i] = r[c]
			}
			q := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT DO NOTHING", s.Table,
				strings.Join(cols, ", "), strings.TrimSuffix(strings.Repeat("?, ", len(cols)), ", "))
			if _, err := db.Exec(q, args...); err != nil {
				t.Fatalf("Seed %s: %v", s.Table, err)
			}
		}
	}
	return &env{t: t, p: p, h: h, ctx: sdk.WithHost(ctx, h)}
}

func (e *env) do(object, action string, payload any) (map[string]any, error) {
	resp, err := e.p.Handle(e.ctx, sdk.Request{Object: object, Action: action, Payload: payload})
	m, _ := resp.Payload.(map[string]any)
	return m, err
}

func (e *env) must(object, action string, payload any) map[string]any {
	e.t.Helper()
	m, err := e.do(object, action, payload)
	if err != nil {
		e.t.Fatalf("%s.%s: %v", object, action, err)
	}
	return m
}

func (e *env) items(object string, query map[string]any) []map[string]any {
	e.t.Helper()
	m := e.must(object, "list", map[string]any{"query": query})
	var out []map[string]any
	for _, it := range m["items"].([]any) {
		out = append(out, it.(map[string]any))
	}
	return out
}

func data(kv ...any) map[string]any { return map[string]any{"data": row(kv...)} }

func expect(t *testing.T, err error, target error, what string) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("%s: %v erwartet, bekommen: %v", what, target, err)
	}
}

func (e *env) newBP(name string) string {
	return e.must("BusinessPartner", "create", data("type", "ORGANIZATION", "name1", name))["id"].(string)
}

// --- Tests -----------------------------------------------------------------------------

func TestSeedsAndCatalogRules(t *testing.T) {
	e := setup(t)
	if n := len(e.items("PartnerCommType", map[string]any{"category_code": "PHONE"})); n != 2 {
		t.Fatalf("PHONE-Typen: %d", n)
	}
	roles := e.items("PartnerRoleType", nil)
	if len(roles) != 4 || roles[1]["code"] != "DEBITOR" || roles[1]["is_debitor"] != true {
		t.Fatalf("Rollentypen: %v", roles)
	}

	// is_main eindeutig: Adressrolle global, Kommunikationstyp je Kategorie.
	_, err := e.do("PartnerAddressRole", "create", data("code", "DELIVERY", "description", "Lieferanschrift", "is_main", true))
	expect(t, err, sdk.ErrInvalidArgument, "zweite Hauptanschrift")
	_, err = e.do("PartnerCommType", "create", data("code", "EMAIL_PRIV", "category_code", "EMAIL", "description", "privat", "is_main", true))
	expect(t, err, sdk.ErrInvalidArgument, "zweiter Haupt-E-Mail-Typ")
	e.must("PartnerCommType", "create", data("code", "FAX_WORK", "category_code", "FAX", "description", "Fax", "is_main", true)) // andere Kategorie: ok
	_, err = e.do("PartnerCommType", "create", data("code", "X", "category_code", "GIBTSNICHT", "description", "x"))
	expect(t, err, sdk.ErrInvalidArgument, "unbekannte Kategorie")

	// Zeitscheibe: Standard und Prüfung.
	r := e.must("PartnerAddressRole", "create", data("code", "DELIVERY", "description", "Lieferanschrift"))
	if r["valid_to"] != "9999-12-31" || r["valid_from"] == nil {
		t.Fatalf("Standard-Zeitscheibe: %v", r)
	}
	_, err = e.do("PartnerAddressRole", "create", data("code", "SITE", "description", "Standort", "valid_from", "2026-05-01", "valid_to", "2026-04-30"))
	expect(t, err, sdk.ErrInvalidArgument, "valid_from > valid_to")

	// Code ist nach dem Anlegen fest.
	_, err = e.do("PartnerAddressRole", "update", map[string]any{"id": "DELIVERY", "data": row("code", "DELIV2", "description", "x")})
	expect(t, err, sdk.ErrInvalidArgument, "Code ändern")
}

func TestBusinessPartnerAndContacts(t *testing.T) {
	e := setup(t)
	bp := e.must("BusinessPartner", "create", data("type", "ORGANIZATION", "name1", "Acme AG", "name2", "Zürich"))
	if bp["search_term"] != "ACME AG" || bp["is_blocked"] != false {
		t.Fatalf("Partner: %v", bp)
	}
	_, err := e.do("BusinessPartner", "create", data("type", "ROBOT", "name1", "x"))
	expect(t, err, sdk.ErrInvalidArgument, "ungültige Art")
	if got := e.items("BusinessPartner", map[string]any{"q": "acme"}); len(got) != 1 {
		t.Fatalf("Suche: %v", got)
	}
	id := bp["id"].(string)

	contact := func(typ, value string) error {
		_, err := e.do("PartnerContact", "create", data("bp_id", id, "comm_type_code", typ, "value", value))
		return err
	}
	expect(t, contact("EMAIL_WORK", "keine-mail"), sdk.ErrInvalidArgument, "E-Mail-Format")
	expect(t, contact("PHONE_WORK", "abc"), sdk.ErrInvalidArgument, "Telefon-Zeichen")
	if err := contact("EMAIL_WORK", "info@acme.ch"); err != nil {
		t.Fatal(err)
	}
	if err := contact("MOBILE", "+41 79 123 45 67"); err != nil {
		t.Fatal(err)
	}
	e.must("PartnerCommType", "create", data("code", "HOMEPAGE", "category_code", "WEB", "description", "Webseite"))
	expect(t, contact("HOMEPAGE", "acme.ch"), sdk.ErrInvalidArgument, "URL ohne Schema")
	if err := contact("HOMEPAGE", "https://acme.ch"); err != nil {
		t.Fatal(err)
	}
	_, err = e.do("PartnerContact", "create", data("bp_id", "gibtsnicht", "comm_type_code", "MOBILE", "value", "+41 1"))
	expect(t, err, sdk.ErrInvalidArgument, "unbekannter Partner")

	// Adresse + Zuordnung, Bank.
	addr := e.must("PartnerAddressData", "create", data("street", "Bahnhofstrasse", "house_no", "1", "zip_code", "8001", "city", "Zürich", "country", "ch"))
	if addr["country"] != "CH" {
		t.Fatalf("Land: %v", addr["country"])
	}
	_, err = e.do("PartnerAddressData", "create", data("street", "x", "zip_code", "1", "city", "y", "country", "Schweiz"))
	expect(t, err, sdk.ErrInvalidArgument, "Land")
	e.must("PartnerAddress", "create", data("bp_id", id, "address_id", addr["id"], "address_role_code", "MAIN", "is_default", true))
	_, err = e.do("PartnerAddressData", "delete", map[string]any{"id": addr["id"]})
	expect(t, err, sdk.ErrFailedPrecondition, "verwendete Adresse löschen")

	b := e.must("PartnerBankDetail", "create", data("bp_id", id, "iban", "ch93 0076 2011 6238 5295 7", "bic", "ubswchzh80a"))
	if b["iban"] != "CH9300762011623852957" || b["bic"] != "UBSWCHZH80A" {
		t.Fatalf("Bank: %v", b)
	}
	_, err = e.do("PartnerBankDetail", "create", data("bp_id", id, "iban", "CH9400762011623852957"))
	expect(t, err, sdk.ErrInvalidArgument, "IBAN-Prüfziffer")

	// Katalog in Verwendung, Löschen des Partners räumt Beziehungen ab.
	_, err = e.do("PartnerCommType", "delete", map[string]any{"id": "MOBILE"})
	expect(t, err, sdk.ErrFailedPrecondition, "verwendeter Kommunikationstyp")
	e.must("BusinessPartner", "delete", map[string]any{"id": id})
	if n := len(e.items("PartnerContact", map[string]any{"bp_id": id})); n != 0 {
		t.Fatalf("Kontakte nach Löschen: %d", n)
	}
	e.must("PartnerAddressData", "delete", map[string]any{"id": addr["id"]}) // jetzt frei
}

func TestFinanceRolesAndCompanyCodes(t *testing.T) {
	e := setup(t)
	id := e.newBP("Kunde AG")

	_, err := e.do("PartnerRole", "create", data("bp_id", id, "role_code", "DEBITOR"))
	expect(t, err, sdk.ErrInvalidArgument, "Finanzrolle ohne Buchungskreis")
	_, err = e.do("PartnerRole", "create", data("bp_id", id, "role_code", "DEBITOR", "company_codes", "9999;140000"))
	expect(t, err, sdk.ErrInvalidArgument, "Buchungskreis nicht in iam")
	_, err = e.do("PartnerRole", "create", data("bp_id", id, "role_code", "TENANT", "company_codes", "1000"))
	expect(t, err, sdk.ErrInvalidArgument, "Buchungskreis bei Nicht-Finanzrolle")
	e.must("PartnerRole", "create", data("bp_id", id, "role_code", "TENANT"))

	// Ohne Recht im Buchungskreis 1000 → abgelehnt, nichts angelegt (Transaktion).
	e.h.granted["create"] = []string{"2000"}
	_, err = e.do("PartnerRole", "create", data("bp_id", id, "role_code", "DEBITOR", "company_codes", "1000;140000;NT30"))
	expect(t, err, sdk.ErrPermissionDenied, "Buchungskreis ohne Berechtigung")
	if n := len(e.items("PartnerRole", map[string]any{"bp_id": id, "role_code": "DEBITOR"})); n != 0 {
		t.Fatal("Rolle trotz Fehler angelegt")
	}
	e.h.granted["create"] = []string{"*"}

	role := e.must("PartnerRole", "create", data("bp_id", id, "role_code", "DEBITOR", "valid_from", "2026-01-01", "company_codes", "1000;140000;NT30"))
	if role["company_codes"] != "1000" || !strings.Contains(role["id"].(string), "|") {
		t.Fatalf("Rolle: %v", role)
	}
	if got := e.must("PartnerRole", "get", map[string]any{"id": role["id"]}); got["role_code"] != "DEBITOR" {
		t.Fatalf("zusammengesetzte id: %v", got)
	}
	cc := e.items(ccObject, map[string]any{"bp_id": id})
	if len(cc) != 1 || cc[0]["reconciliation_account"] != "140000" || cc[0]["payment_terms"] != "NT30" || cc[0]["dunning_block"] != false {
		t.Fatalf("Buchungskreisdaten: %v", cc)
	}

	// Kreditor-Daten ohne Kreditor-Rolle → abgelehnt; TENANT ist keine Finanzrolle.
	_, err = e.do(ccObject, "create", data("bp_id", id, "company_code", "2000", "role_code", "CREDITOR"))
	expect(t, err, sdk.ErrInvalidArgument, "Kreditor ohne Rolle")
	_, err = e.do(ccObject, "create", data("bp_id", id, "company_code", "2000", "role_code", "TENANT"))
	expect(t, err, sdk.ErrInvalidArgument, "keine Finanzrolle")

	// Buchungskreis-Zwang: letzter Eintrag der aktiven Rolle bleibt.
	_, err = e.do(ccObject, "delete", map[string]any{"id": cc[0]["id"]})
	expect(t, err, sdk.ErrFailedPrecondition, "letzter Buchungskreis")
	e.must(ccObject, "create", data("bp_id", id, "company_code", "2000", "role_code", "DEBITOR", "payment_terms", "NT10", "posting_block", "true"))
	e.must(ccObject, "delete", map[string]any{"id": cc[0]["id"]})

	// Sicht und Änderungen nur in erlaubten Buchungskreisen.
	e.must(ccObject, "create", data("bp_id", id, "company_code", "1000", "role_code", "DEBITOR"))
	e.h.granted["list"] = []string{"2000"}
	if got := e.items(ccObject, map[string]any{"bp_id": id}); len(got) != 1 || got[0]["company_code"] != "2000" || got[0]["posting_block"] != true {
		t.Fatalf("gefilterte Liste: %v", got)
	}
	e.h.granted["update"] = []string{"2000"}
	_, err = e.do(ccObject, "update", map[string]any{"id": id + "|1000|DEBITOR", "data": row("payment_terms", "NT60")})
	expect(t, err, sdk.ErrPermissionDenied, "Änderung in fremdem Buchungskreis")

	// Rolle beenden: letzte Zuordnung → Buchungskreisdaten werden mit gelöscht.
	e.must("PartnerRole", "delete", map[string]any{"id": role["id"]})
	e.h.granted["list"] = []string{"*"}
	if n := len(e.items(ccObject, map[string]any{"bp_id": id})); n != 0 {
		t.Fatalf("Buchungskreisdaten nach Rollenende: %d", n)
	}
}

func TestMetamodel(t *testing.T) {
	p := newPlugin(t)
	resp, err := p.Handle(context.Background(), sdk.Request{Object: sdk.ObjectCatalog, Action: sdk.ActionDescribe})
	if err != nil {
		t.Fatal(err)
	}
	desc := resp.Payload.(metamodel.DescribeResponse)
	defs := desc.Objects
	if len(defs) != 11 {
		t.Fatalf("Objects: %d", len(defs))
	}
	for _, d := range defs {
		if err := d.Validate(); err != nil {
			t.Error(err)
		}
	}
	m, _ := p.Manifest(context.Background())
	for _, d := range defs { // jede Action im Metamodell ist eine Route
		for _, a := range d.Actions {
			if !slices.ContainsFunc(m.Capabilities, func(c sdk.Capability) bool { return c.Object == d.Name && slices.Contains(c.Actions, a.Name) }) {
				t.Errorf("%s.%s fehlt im Manifest", d.Name, a.Name)
			}
		}
	}
}

func newPlugin(t *testing.T) *module.Plugin {
	t.Helper()
	p := module.NewPlugin(module.Info{Name: "partner", Version: "test"}, New())
	if err := p.Err(); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestModule: Das Modul bündelt alle 11 Objects unter einem Namensraum,
// Kataloge in einer eigenen Gruppe; das Schema kommt mit schema-Block.
func TestModule(t *testing.T) {
	p := newPlugin(t)
	resp, _ := p.Handle(context.Background(), sdk.Request{Object: sdk.ObjectCatalog, Action: sdk.ActionDescribe})
	desc := resp.Payload.(metamodel.DescribeResponse)
	if len(desc.Modules) != 1 || desc.Modules[0].Name != Name || len(desc.Modules[0].Objects) != 11 {
		t.Fatalf("Module: %+v", desc.Modules)
	}
	defined := map[string]bool{}
	for _, d := range desc.Objects {
		defined[d.Name] = true
	}
	if err := desc.Modules[0].Validate(defined); err != nil {
		t.Fatal(err)
	}
	sections := map[string]int{}
	for _, o := range desc.Modules[0].Objects {
		sections[o.Section]++
	}
	if sections["Kataloge"] != 4 || sections["Partnerdaten"] != 7 || desc.Modules[0].Objects[0].Object != "BusinessPartner" {
		t.Fatalf("Navigation: %+v", desc.Modules[0].Objects)
	}

	resp, err := p.Handle(context.Background(), sdk.Request{Object: sdk.ObjectDBSchema, Action: sdk.ActionInit, Payload: sdk.SchemaInitRequest{Module: "partner"}})
	if err != nil {
		t.Fatal(err)
	}
	if s := resp.Payload.(sdk.SchemaInitResponse).Schema; strings.Count(s, `schema "main"`) != 1 {
		t.Fatalf("schema-Block: %d×", strings.Count(s, `schema "main"`))
	}

	// Nicht registrierte Routen lehnt das Plugin ab.
	if _, err := p.Handle(context.Background(), sdk.Request{Object: "Unbekannt", Action: "list"}); !errors.Is(err, sdk.ErrUnimplemented) {
		t.Fatalf("unbekanntes Object: %v", err)
	}
}
