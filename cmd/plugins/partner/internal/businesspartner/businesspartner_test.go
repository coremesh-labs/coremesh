package businesspartner

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"ariga.io/atlas/sql/schema"
	"ariga.io/atlas/sql/sqlite"
	_ "modernc.org/sqlite"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
	"github.com/coremesh-labs/coremesh/pkg/sdk/module"
)

// testHost: SQLite mit einer einzigen Verbindung (Transaktionen über
// BEGIN/COMMIT wie im Host), iam-Funktionen als Attrappe.
type testHost struct {
	sdk.Host
	db           *sql.DB
	companyCodes []string            // in iam angelegt
	granted      map[string][]string // action → erlaubte Buchungskreise ("*" = alle)
	self         sdk.Handler         // Plugin selbst: alle übrigen Objects
	ranges       map[string]*fakeRange // Nummernkreis BusinessPartner je Intervallschlüssel
	events       []map[string]any      // gesendete SystemEvents
}

// fakeRange: Intervall wie in numrange (intern: fortlaufend, extern: Muster).
type fakeRange struct {
	external bool
	pattern  string
	current  int
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

func (h *testHost) Handle(ctx context.Context, req sdk.Request) (sdk.Response, error) {
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
	case "NumberRange.Define":
		return sdk.Response{}, nil
	case "NumberRange.Info", "NumberRange.Assign":
		var in struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		_ = sdk.Decode(req.Payload, &in)
		r := h.ranges[in.Key]
		if r == nil {
			r = &fakeRange{current: 99999}
			h.ranges[in.Key] = r
		}
		if req.Action == "Info" {
			return sdk.Response{Payload: map[string]any{"interval": "BusinessPartner|*|" + in.Key + "|0", "external": r.external,
				"external_pattern": r.pattern, "from": 100000, "to": 999999, "exists": true, "active": true}}, nil
		}
		v := strings.ToUpper(strings.TrimSpace(in.Value))
		switch {
		case !r.external && v != "":
			return sdk.Response{}, fmt.Errorf("%w: intern – Nummer leer lassen", sdk.ErrInvalidArgument)
		case !r.external:
			r.current++
			return sdk.Response{Payload: map[string]any{"number": fmt.Sprintf("%06d", r.current), "value": r.current}}, nil
		case v == "" || !regexp.MustCompile("^(?:"+r.pattern+")$").MatchString(v):
			return sdk.Response{}, fmt.Errorf("%w: Nummer %q passt nicht zu %s", sdk.ErrInvalidArgument, v, r.pattern)
		}
		return sdk.Response{Payload: map[string]any{"number": v, "external": true}}, nil
	case "SystemEvent.Push":
		ev, _ := req.Payload.(map[string]any)
		if ev == nil {
			_ = sdk.Decode(req.Payload, &ev)
		}
		h.events = append(h.events, ev)
		return sdk.Response{}, nil
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
	if h.self != nil {
		return h.self.Handle(ctx, req)
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

	h := &testHost{db: db, companyCodes: []string{"1000", "2000"}, ranges: map[string]*fakeRange{}, granted: map[string][]string{
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
	h.self = p // eigene Objects über den "Dispatcher" (Aggregate)
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
	return e.must("BusinessPartner", "create", data("type", "ORGANIZATION", "name1", name, "valid_from", "2000-01-01"))["id"].(string)
}

// --- Tests -----------------------------------------------------------------------------

func TestSeedsAndCatalogRules(t *testing.T) {
	e := setup(t)
	if n := len(e.items("PartnerCommType", map[string]any{"category_code": "PHONE"})); n != 2 {
		t.Fatalf("PHONE-Typen: %d", n)
	}
	roles := e.items("PartnerRoleType", nil)
	if len(roles) != 5 || roles[0]["code"] != "AUTHORITY" || roles[0]["is_creditor"] != true || roles[2]["code"] != "DEBITOR" || roles[2]["is_debitor"] != true {
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
	if bp["search_term"] != "ACMEAG" || bp["id"] != "100000" || bp["group_code"] != "STD" || bp["is_blocked"] != false {
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
	// Kein physisches Löschen: Adressen (immutable) haben keine Ende-Action.
	_, err = e.do("PartnerAddressData", "delete", map[string]any{"id": addr["id"]})
	expect(t, err, sdk.ErrUnimplemented, "Adresse löschen")

	b := e.must("PartnerBankDetail", "create", data("bp_id", id, "iban", "ch93 0076 2011 6238 5295 7", "bic", "ubswchzh80a"))
	if b["iban"] != "CH9300762011623852957" || b["bic"] != "UBSWCHZH80A" {
		t.Fatalf("Bank: %v", b)
	}
	_, err = e.do("PartnerBankDetail", "create", data("bp_id", id, "iban", "CH9400762011623852957"))
	expect(t, err, sdk.ErrInvalidArgument, "IBAN-Prüfziffer")

	// Lebenszyklus statt Löschen.
	_, err = e.do("PartnerCommType", "delete", map[string]any{"id": "MOBILE"})
	expect(t, err, sdk.ErrUnimplemented, "Katalog löschen")

	// Zeitscheibe (Adresszuordnung): Enddatum ist Pflicht, nie automatisch heute.
	pa := e.items("PartnerAddress", map[string]any{"bp_id": id})[0]
	_, err = e.do("PartnerAddress", "expire", map[string]any{"id": pa["id"]})
	expect(t, err, sdk.ErrInvalidArgument, "Enddatum fehlt")
	_, err = e.do("PartnerAddress", "expire", map[string]any{"id": pa["id"], "valid_to": "2000-01-01"})
	expect(t, err, sdk.ErrInvalidArgument, "Enddatum vor Beginn")
	if ended := e.must("PartnerAddress", "expire", map[string]any{"id": pa["id"], "valid_to": "2026-12-31"}); ended["valid_to"] != "2026-12-31" {
		t.Fatalf("expire: %v", ended)
	}

	// Zeitscheibe (Partner): Enddatum statt Inaktivieren; Beziehungen bleiben.
	// Nach dem Ende sind neue Verweise ab diesem Tag ungültig.
	bp = e.must("BusinessPartner", "expire", map[string]any{"id": id, "valid_to": "2026-12-31"})
	if bp["valid_to"] != "2026-12-31" || bp["id"] != id || bp["_id"] != id+"|"+str(bp["valid_from"]) {
		t.Fatalf("Partner beenden: %v", bp)
	}
	if n := len(e.items("PartnerContact", map[string]any{"bp_id": id})); n == 0 {
		t.Fatal("Kontakte nach Beenden weg")
	}
	_, err = e.do("PartnerContact", "create", data("bp_id", id, "comm_type_code", "MOBILE", "value", "+41 79 000 00 00", "valid_from", "2027-01-01"))
	expect(t, err, sdk.ErrInvalidArgument, "Verweis nach dem Ende des Partners")
	if _, err := e.do("BusinessPartner", "deactivate", map[string]any{"id": id}); !errors.Is(err, sdk.ErrUnimplemented) {
		t.Fatalf("deactivate gibt es nicht mehr: %v", err)
	}
}

func TestFinanceRolesAndCompanyCodes(t *testing.T) {
	e := setup(t)
	id := e.newBP("Kunde AG")

	_, err := e.do("PartnerRole", "create", data("bp_id", id, "role_code", "DEBITOR"))
	expect(t, err, sdk.ErrInvalidArgument, "Finanzrolle ohne Buchungskreis")
	_, err = e.do("PartnerRole", "create", data("bp_id", id, "role_code", "DEBITOR", "company_codes", "9999;140000"))
	expect(t, err, sdk.ErrInvalidArgument, "Buchungskreis nicht in iam")
	// Keine Finanzrolle: Interessent (eigener Rollentyp ohne Debitor/Kreditor)
	e.must("PartnerRoleType", "create", data("code", "PROSPECT", "description", "Interessent"))
	_, err = e.do("PartnerRole", "create", data("bp_id", id, "role_code", "PROSPECT", "company_codes", "1000;1200"))
	expect(t, err, sdk.ErrInvalidArgument, "Buchungskreis bei Nicht-Finanzrolle")
	e.must("PartnerRole", "create", data("bp_id", id, "role_code", "PROSPECT"))
	// Mieter ist Finanzrolle (Vorschlag): eigener Buchungskreis mit eigenem Abstimmkonto
	_, err = e.do("PartnerRole", "create", data("bp_id", id, "role_code", "TENANT"))
	expect(t, err, sdk.ErrInvalidArgument, "Mieter ohne Buchungskreis")
	_, err = e.do("PartnerRole", "create", data("bp_id", id, "role_code", "TENANT", "company_codes", "1000"))
	expect(t, err, sdk.ErrInvalidArgument, "Mieter ohne Abstimmkonto")
	e.must("PartnerRole", "create", data("bp_id", id, "role_code", "TENANT", "company_codes", "1000;1200"))

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
	cc := e.items(ccObject, map[string]any{"bp_id": id, "role_code": "DEBITOR"})
	if len(cc) != 1 || cc[0]["reconciliation_account"] != "140000" || cc[0]["payment_terms"] != "NT30" || cc[0]["dunning_block"] != false {
		t.Fatalf("Buchungskreisdaten: %v", cc)
	}

	// Kreditor-Daten ohne Kreditor-Rolle → abgelehnt; PROSPECT ist keine Finanzrolle.
	_, err = e.do(ccObject, "create", data("bp_id", id, "company_code", "2000", "role_code", "CREDITOR", "reconciliation_account", "1600"))
	expect(t, err, sdk.ErrInvalidArgument, "Kreditor ohne Rolle")
	_, err = e.do(ccObject, "create", data("bp_id", id, "company_code", "2000", "role_code", "PROSPECT", "reconciliation_account", "1200"))
	expect(t, err, sdk.ErrInvalidArgument, "keine Finanzrolle")
	_, err = e.do(ccObject, "create", data("bp_id", id, "company_code", "2000", "role_code", "DEBITOR"))
	expect(t, err, sdk.ErrInvalidArgument, "Buchungskreisdaten ohne Abstimmkonto")

	// Buchungskreisdaten haben weder Zeitscheibe noch Status-Flag: immutable.
	_, err = e.do(ccObject, "delete", map[string]any{"id": cc[0]["id"]})
	expect(t, err, sdk.ErrUnimplemented, "Buchungskreisdaten löschen")
	e.must(ccObject, "create", data("bp_id", id, "company_code", "2000", "role_code", "DEBITOR", "reconciliation_account", "140000",
		"payment_terms", "NT10", "posting_block", "true"))

	// Sicht und Änderungen nur in erlaubten Buchungskreisen.
	e.h.granted["list"] = []string{"2000"}
	if got := e.items(ccObject, map[string]any{"bp_id": id, "role_code": "DEBITOR"}); len(got) != 1 || got[0]["company_code"] != "2000" || got[0]["posting_block"] != true {
		t.Fatalf("gefilterte Liste: %v", got)
	}
	e.h.granted["update"] = []string{"2000"}
	_, err = e.do(ccObject, "update", map[string]any{"id": id + "|1000|DEBITOR", "data": row("payment_terms", "NT60")})
	expect(t, err, sdk.ErrPermissionDenied, "Änderung in fremdem Buchungskreis")

	// Rolle beenden (Zeitscheibe): Enddatum setzen, Buchungskreisdaten bleiben.
	if ended := e.must("PartnerRole", "expire", map[string]any{"id": role["id"], "valid_to": "2026-06-30"}); ended["valid_to"] != "2026-06-30" {
		t.Fatalf("Rolle beenden: %v", ended)
	}
	e.h.granted["list"] = []string{"*"}
	if n := len(e.items(ccObject, map[string]any{"bp_id": id, "role_code": "DEBITOR"})); n != 2 {
		t.Fatalf("Buchungskreisdaten nach Rollenende: %d", n)
	}
}

// TestFinanceRoleWithoutData: Wird ein Rollentyp nachträglich Finanzrolle,
// lässt sich ein Partner mit dieser Rolle ohne Buchungskreisdaten nicht mehr
// speichern, bis sie ergänzt sind.
func TestFinanceRoleWithoutData(t *testing.T) {
	e := setup(t)
	id := e.newBP("Müller")
	e.must("PartnerRoleType", "create", data("code", "OCCUPANT", "description", "Bewohner"))
	e.must("PartnerRole", "create", data("bp_id", id, "role_code", "OCCUPANT"))
	bp := e.must("BusinessPartner", "get", map[string]any{"id": id})
	types := e.items("PartnerRoleType", map[string]any{})
	var typeID any
	for _, rt := range types {
		if rt["code"] == "OCCUPANT" {
			typeID = rt["_id"]
		}
	}
	e.must("PartnerRoleType", "update", map[string]any{"id": typeID, "data": row("is_debitor", true)})
	_, err := e.do("BusinessPartner", "update", map[string]any{"id": bp["_id"], "data": row("name2", "Anna")})
	expect(t, err, sdk.ErrInvalidArgument, "Finanzrolle ohne Buchungskreisdaten")
	if err == nil || !strings.Contains(err.Error(), "OCCUPANT") {
		t.Fatalf("Meldung nennt die Rolle nicht: %v", err)
	}
	e.must(ccObject, "create", data("bp_id", id, "company_code", "1000", "role_code", "OCCUPANT", "reconciliation_account", "1200"))
	e.must("BusinessPartner", "update", map[string]any{"id": bp["_id"], "data": row("name2", "Anna")})
}

func TestMetamodel(t *testing.T) {
	p := newPlugin(t)
	resp, err := p.Handle(context.Background(), sdk.Request{Object: sdk.ObjectCatalog, Action: sdk.ActionDescribe})
	if err != nil {
		t.Fatal(err)
	}
	desc := resp.Payload.(metamodel.DescribeResponse)
	defs := desc.Objects
	if len(defs) != 13 {
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

// TestModule: Das Modul bündelt alle 13 Objects unter einem Namensraum,
// Kataloge in einer eigenen Gruppe; das Schema kommt mit schema-Block.
func TestModule(t *testing.T) {
	p := newPlugin(t)
	resp, _ := p.Handle(context.Background(), sdk.Request{Object: sdk.ObjectCatalog, Action: sdk.ActionDescribe})
	desc := resp.Payload.(metamodel.DescribeResponse)
	if len(desc.Modules) != 1 || desc.Modules[0].Name != Name || len(desc.Modules[0].Objects) != 13 {
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
	if sections["Kataloge"] != 6 || sections["Partnerdaten"] != 7 || desc.Modules[0].Objects[0].Object != "BusinessPartner" {
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

// TestLabelsAndLookups: Verweise tragen lesbare Texte (_labels) und sind im
// Metamodell als Lookup beschrieben.
func TestLabelsAndLookups(t *testing.T) {
	e := setup(t)
	bp := e.must("BusinessPartner", "create", data("type", "ORGANIZATION", "name1", "Label AG"))
	addr := e.must("PartnerAddressData", "create", data("street", "Seestrasse", "house_no", "1", "zip_code", "8000", "city", "Zürich", "country", "ch"))
	e.must("PartnerAddress", "create", data("bp_id", bp["id"], "address_id", addr["id"], "address_role_code", "MAIN"))

	got := e.items("PartnerAddress", map[string]any{"bp_id": bp["id"]})
	labels, _ := got[0]["_labels"].(map[string]any)
	if labels["address_role_code"] != "Hauptanschrift" || labels["address_id"] != "Seestrasse 1 8000 Zürich CH" || labels["bp_id"] != "Label AG" {
		t.Fatalf("_labels: %v", got[0]["_labels"])
	}

	// Suche in Katalogen (Lookup-Dialog).
	if roles := e.items("PartnerAddressRole", map[string]any{"q": "rechnung"}); len(roles) != 1 || roles[0]["code"] != "INVOICE" {
		t.Fatalf("Suche: %v", roles)
	}

	resp, _ := e.p.Handle(context.Background(), sdk.Request{Object: sdk.ObjectCatalog, Action: sdk.ActionDescribe})
	for _, d := range resp.Payload.(metamodel.DescribeResponse).Objects {
		if d.Name != "PartnerAddress" {
			continue
		}
		for _, f := range d.Fields {
			if f.Key == "address_role_code" && (f.Lookup == nil || f.Lookup.Object != "PartnerAddressRole" || f.Lookup.ValueField != "code") {
				t.Fatalf("Lookup: %+v", f.Lookup)
			}
		}
	}
}

// TestAggregate: Geschäftspartner mit Adresse und Kommunikation in einem
// Aufruf – atomar, auch über die Fachregeln der Unter-Objects.
func TestAggregate(t *testing.T) {
	e := setup(t)
	addr := e.must("PartnerAddressData", "create", data("street", "Bahnhofstrasse", "zip_code", "8001", "city", "Zürich", "country", "CH"))

	out := e.must("BusinessPartner", "saveAggregate", map[string]any{
		"data": row("type", "ORGANIZATION", "name1", "Aggregat AG"),
		"relations": map[string]any{
			"adressen":      map[string]any{"create": []any{row("address_id", addr["id"], "address_role_code", "MAIN")}},
			"kommunikation": map[string]any{"create": []any{row("comm_type_code", "EMAIL_WORK", "value", "info@aggregat.ch")}},
		},
	})
	rels := out["relations"].(map[string]any)
	if len(rels["adressen"].([]any)) != 1 || len(rels["kommunikation"].([]any)) != 1 || len(rels["rollen"].([]any)) != 0 {
		t.Fatalf("Aggregat: %v", rels)
	}

	// Ungültige E-Mail im Unter-Object: auch der Partner entsteht nicht.
	_, err := e.do("BusinessPartner", "saveAggregate", map[string]any{
		"data":      row("type", "ORGANIZATION", "name1", "Fehler AG"),
		"relations": map[string]any{"kommunikation": map[string]any{"create": []any{row("comm_type_code", "EMAIL_WORK", "value", "kein-mail")}}},
	})
	expect(t, err, sdk.ErrInvalidArgument, "ungültige E-Mail im Aggregat")
	if n := len(e.items("BusinessPartner", map[string]any{"q": "Fehler AG"})); n != 0 {
		t.Fatalf("Rollback: %d Partner", n)
	}
}

// TestLifecycleTypes: Jede Entität hat genau einen Lebenszyklus, abgeleitet
// aus Zeitscheibe bzw. Status-Flag; nur passende Ende-Actions sind Routen.
func TestLifecycleTypes(t *testing.T) {
	p := newPlugin(t)
	resp, _ := p.Handle(context.Background(), sdk.Request{Object: sdk.ObjectCatalog, Action: sdk.ActionDescribe})
	got := map[string]metamodel.LifecycleType{}
	for _, d := range resp.Payload.(metamodel.DescribeResponse).Objects {
		got[d.Name] = d.Lifecycle.Kind()
	}
	want := map[string]metamodel.LifecycleType{
		"BusinessPartner":     metamodel.LifecycleTimeSlice,
		"PartnerAddress":      metamodel.LifecycleTimeSlice, // Zuordnung Partner ↔ Adresse mit Rolle
		"PartnerRole":         metamodel.LifecycleTimeSlice,
		"PartnerContact":      metamodel.LifecycleTimeSlice,
		"PartnerBankDetail":   metamodel.LifecycleTimeSlice,
		"PartnerAddressRole":  metamodel.LifecycleTimeSlice, // Katalog mit Zeitscheibe
		"PartnerAddressData":  metamodel.LifecycleImmutable, // Adressdetails
		"PartnerCommCategory": metamodel.LifecycleImmutable, // Katalog ohne Zeitscheibe
		"PartnerCompanyCode":  metamodel.LifecycleImmutable,
	}
	for obj, w := range want {
		if got[obj] != w {
			t.Errorf("%s: %s, erwartet %s", obj, got[obj], w)
		}
	}
	m, _ := p.Manifest(context.Background())
	for _, c := range m.Capabilities {
		if slices.Contains(c.Actions, "delete") {
			t.Errorf("%s bietet delete an", c.Object)
		}
		if got[c.Object] == metamodel.LifecycleImmutable && (slices.Contains(c.Actions, "expire") || slices.Contains(c.Actions, "deactivate")) {
			t.Errorf("%s ist immutable, bietet aber eine Ende-Action an: %v", c.Object, c.Actions)
		}
	}
}

// TestValidFromInPrimaryKey: Regel – bei jeder Entität mit Zeitscheibe ist
// valid_from (letzter) Teil des Primärschlüssels, im Code wie im Schema.
func TestValidFromInPrimaryKey(t *testing.T) {
	m := New()
	var desired schema.Schema
	if err := sqlite.EvalHCLBytes([]byte("schema \"main\" {}\n"+schemaHCL), &desired, nil); err != nil {
		t.Fatal(err)
	}
	for _, e := range m.set.Entities() {
		if !e.TimeSlice {
			continue
		}
		if e.Keys[len(e.Keys)-1] != "valid_from" {
			t.Errorf("%s: Keys %v – valid_from muss letzter Schlüsselteil sein", e.Object, e.Keys)
		}
		tb, ok := desired.Table(e.Table)
		if !ok || tb.PrimaryKey == nil {
			t.Fatalf("%s: Tabelle %s ohne Primärschlüssel", e.Object, e.Table)
		}
		var pk []string
		for _, p := range tb.PrimaryKey.Parts {
			pk = append(pk, p.C.Name)
		}
		if !slices.Equal(pk, e.Keys) {
			t.Errorf("%s: Primärschlüssel %v ≠ Keys %v", e.Table, pk, e.Keys)
		}
	}
}

// TestTimeSliceVersions: Ein fachlicher Schlüssel kann mehrere Zeitscheiben
// haben; sie dürfen sich nicht überschneiden. Ohne valid_from in der id gilt
// die heute gültige Zeitscheibe.
func TestTimeSliceVersions(t *testing.T) {
	e := setup(t)
	_, err := e.do("PartnerRoleType", "create", data("code", "DEBITOR", "description", "Debitor neu", "valid_from", "2030-01-01"))
	expect(t, err, sdk.ErrInvalidArgument, "überschneidende Zeitscheibe")

	e.must("PartnerRoleType", "expire", map[string]any{"id": "DEBITOR", "valid_to": "2029-12-31"})
	next := e.must("PartnerRoleType", "create", data("code", "DEBITOR", "description", "Debitor neu", "is_debitor", true, "valid_from", "2030-01-01"))
	if next["_id"] != "DEBITOR|2030-01-01" || next["id"] != "DEBITOR" {
		t.Fatalf("neue Zeitscheibe: %v", next)
	}
	if got := e.must("PartnerRoleType", "get", map[string]any{"id": "DEBITOR"}); got["description"] != "Debitor" {
		t.Fatalf("heute gültige Zeitscheibe: %v", got)
	}
	if got := e.must("PartnerRoleType", "get", map[string]any{"id": "DEBITOR|2030-01-01"}); got["description"] != "Debitor neu" {
		t.Fatalf("Zeitscheibe über volle id: %v", got)
	}
	// Ein späteres Ende darf nicht in die folgende Zeitscheibe reichen.
	_, err = e.do("PartnerRoleType", "expire", map[string]any{"id": "DEBITOR|1900-01-01", "valid_to": "2030-06-30"})
	expect(t, err, sdk.ErrInvalidArgument, "Ende in die folgende Zeitscheibe")

	// Partner: fachliche id bleibt, die Datensatz-ID enthält das Beginndatum;
	// Änderungen über die fachliche id treffen nur die gültige Zeitscheibe.
	id := e.newBP("Versionen AG")
	got := e.must("BusinessPartner", "get", map[string]any{"id": id})
	if got["id"] != id || got["_id"] != id+"|2000-01-01" {
		t.Fatalf("Partner: %v", got)
	}
	e.must("BusinessPartner", "update", map[string]any{"id": id, "data": row("name2", "Neu")})
	if got := e.must("BusinessPartner", "get", map[string]any{"id": id + "|2000-01-01"}); got["name2"] != "Neu" {
		t.Fatalf("update über fachliche id: %v", got)
	}
	_, err = e.do("BusinessPartner", "update", map[string]any{"id": id, "data": row("valid_from", "2001-01-01")})
	expect(t, err, sdk.ErrInvalidArgument, "Beginndatum ist Schlüssel")
}

// TestMigrationFrom040: Atlas migriert den Bestand von 0.4.0 (Primärschlüssel
// ohne valid_from, Partner ohne Zeitscheibe) ohne Datenverlust.
func TestMigrationFrom040(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	drv, err := sqlite.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	apply := func(hcl string) []schema.Change {
		t.Helper()
		var desired schema.Schema
		if err := sqlite.EvalHCLBytes([]byte("schema \"main\" {}\n"+hcl), &desired, nil); err != nil {
			t.Fatal(err)
		}
		desired.Name = "main"
		for _, tb := range desired.Tables {
			tb.Schema = &desired
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
		return changes
	}
	old, err := os.ReadFile("testdata/schema-0.4.0.hcl")
	if err != nil {
		t.Fatal(err)
	}
	apply(string(old))
	for _, q := range []string{
		`INSERT INTO partner__role_types (code, description, valid_from, valid_to) VALUES ('DEBITOR', 'Debitor', '1900-01-01', '9999-12-31')`,
		`INSERT INTO partner__bp (id, type, name1) VALUES ('bp1', 'ORGANIZATION', 'Alt AG')`,
		`INSERT INTO partner__roles (bp_id, role_code, valid_from, valid_to) VALUES ('bp1', 'DEBITOR', '2020-01-01', '9999-12-31')`,
		`INSERT INTO partner__contacts (id, bp_id, comm_type_code, value, valid_from, valid_to) VALUES ('c1', 'bp1', 'EMAIL_WORK', 'a@alt.ch', '2020-01-01', '9999-12-31')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	changes := apply(schemaHCL)
	for _, c := range changes {
		if _, drop := c.(*schema.DropTable); drop {
			t.Fatalf("Migration darf keine Tabelle löschen: %T", c)
		}
	}
	var from, to, name string
	if err := db.QueryRow(`SELECT valid_from, valid_to, name1 FROM partner__bp WHERE id = 'bp1'`).Scan(&from, &to, &name); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(from, "1900-01-01") || !strings.HasPrefix(to, "9999-12-31") || name != "Alt AG" {
		t.Fatalf("Partner nach Migration: %s %s %s", from, to, name)
	}
	var n int
	if err := db.QueryRow(`SELECT (SELECT COUNT(*) FROM partner__roles) + (SELECT COUNT(*) FROM partner__contacts) + (SELECT COUNT(*) FROM partner__role_types)`).Scan(&n); err != nil || n != 3 {
		t.Fatalf("Bestand nach Migration: %d %v", n, err)
	}
	// Zweite Zeitscheibe desselben Codes ist jetzt möglich (Primärschlüssel mit valid_from).
	if _, err := db.Exec(`INSERT INTO partner__role_types (code, description, valid_from, valid_to) VALUES ('DEBITOR', 'Debitor neu', '2030-01-01', '9999-12-31')`); err != nil {
		t.Fatal(err)
	}
}

// TestTranslationsComplete: Jeder Text der Metamodelle hat eine Übersetzung in
// de, en und zh-CN – und keine Datei enthält verwaiste Schlüssel.
func TestTranslationsComplete(t *testing.T) {
	p := newPlugin(t)
	resp, _ := p.Handle(context.Background(), sdk.Request{Object: sdk.ObjectCatalog, Action: sdk.ActionDescribe})
	d := resp.Payload.(metamodel.DescribeResponse)
	want := map[string]bool{}
	for _, m := range d.Modules {
		want[m.TitleKey], want[m.DescriptionKey] = true, true
		for _, o := range m.Objects {
			want[o.SectionKey] = true
		}
	}
	for _, o := range d.Objects {
		want[o.TitleKey] = true
		for _, f := range o.Fields {
			want[f.LabelKey] = true
			for _, op := range f.Options {
				want[op.LabelKey] = true
			}
		}
		for _, s := range o.Sections {
			want[s.TitleKey] = true
		}
	}
	for _, loc := range metamodel.Locales {
		dict := d.Translations[loc]
		for k := range want {
			if dict[k] == "" {
				t.Errorf("%s: Übersetzung fehlt: %s", loc, k)
			}
		}
		for k := range dict {
			if !want[k] && !strings.Contains(k, ".actions.") {
				t.Errorf("%s: verwaister Schlüssel: %s", loc, k)
			}
		}
	}
}

// TestHistoryFilter: list liefert standardmäßig nur heute gültige Datensätze;
// includeHistory=true auch beendete und künftige.
func TestHistoryFilter(t *testing.T) {
	e := setup(t)
	id := e.newBP("Historie AG")
	e.must("PartnerContact", "create", data("bp_id", id, "comm_type_code", "EMAIL_WORK", "value", "alt@h.ch", "valid_from", "2020-01-01", "valid_to", "2020-12-31"))
	e.must("PartnerContact", "create", data("bp_id", id, "comm_type_code", "EMAIL_WORK", "value", "jetzt@h.ch", "valid_from", "2021-01-01"))
	e.must("PartnerContact", "create", data("bp_id", id, "comm_type_code", "MOBILE", "value", "+41 79 000 00 00", "valid_from", "2099-01-01"))
	values := func(q map[string]any) string {
		var out []string
		for _, r := range e.items("PartnerContact", q) {
			out = append(out, str(r["value"]))
		}
		slices.Sort(out)
		return strings.Join(out, ",")
	}
	if got := values(map[string]any{"bp_id": id}); got != "jetzt@h.ch" {
		t.Fatalf("Standard: %s", got)
	}
	if got := values(map[string]any{"bp_id": id, "includeHistory": "true"}); got != "+41 79 000 00 00,alt@h.ch,jetzt@h.ch" {
		t.Fatalf("mit Historie: %s", got)
	}
}

// TestFilterByRole: list mit role liefert nur Partner, die die Rolle heute haben
// (Auswahl in anderen Modulen, z. B. Eigentümer eines Mietobjekts).
func TestFilterByRole(t *testing.T) {
	e := setup(t)
	a, b := e.newBP("Mieter AG"), e.newBP("Andere AG")
	e.must("PartnerRole", "create", data("bp_id", a, "role_code", "TENANT", "valid_from", "2000-01-01", "company_codes", "1000;1200"))
	e.must("PartnerRole", "create", data("bp_id", b, "role_code", "TENANT", "valid_from", "2000-01-01", "valid_to", "2001-12-31", "company_codes", "1000;1200"))
	got := e.items("BusinessPartner", map[string]any{"role": "TENANT"})
	if len(got) != 1 || got[0]["id"] != a {
		t.Fatalf("Mieter heute: %v", got)
	}
	if n := len(e.items("BusinessPartner", map[string]any{})); n < 2 {
		t.Fatalf("ohne Filter: %d", n)
	}
}

// TestPersonSalutation: Geschlecht nur bei natürlichen Personen, Anrede als
// Vorschlag nach Art und Geschlecht, Briefanrede aus der Vorlage.
func TestPersonSalutation(t *testing.T) {
	e := setup(t)
	p := e.must("BusinessPartner", "create", data("type", "PERSON", "name1", "Muster", "name2", "Erika", "gender", "FEMALE"))
	if p["salutation_code"] != "FRAU" || p["letter_salutation"] != "Sehr geehrte Frau Muster" {
		t.Fatalf("Person: %v", p)
	}
	o := e.must("BusinessPartner", "create", data("type", "ORGANIZATION", "name1", "Stadt Musterhausen", "gender", "MALE"))
	if o["gender"] != nil || o["salutation_code"] != "FIRMA" || o["letter_salutation"] != "Sehr geehrte Damen und Herren" {
		t.Fatalf("Organisation: %v", o)
	}
	if n := e.must("BusinessPartner", "create", data("type", "PERSON", "name1", "Ohne")); n["salutation_code"] != nil {
		t.Fatalf("ohne Geschlecht kein Vorschlag: %v", n)
	}
	_, err := e.do("BusinessPartner", "create", data("type", "ORGANIZATION", "name1", "X", "salutation_code", "HERR"))
	expect(t, err, sdk.ErrInvalidArgument, "Anrede einer Person für Organisation")
	e.must("PartnerSalutation", "create", data("code", "dr", "description", "Frau Dr.", "letter_text", "Sehr geehrte Frau Dr. {name1}", "person_type", "PERSON"))
	if d := e.must("BusinessPartner", "create", data("type", "PERSON", "name1", "Klug", "salutation_code", "DR")); d["letter_salutation"] != "Sehr geehrte Frau Dr. Klug" {
		t.Fatalf("eigene Anrede: %v", d)
	}
}
