package tagmanagement

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"ariga.io/atlas/sql/schema"
	"ariga.io/atlas/sql/sqlite"
	_ "modernc.org/sqlite"

	"github.com/coremesh-lab/coremesh/pkg/sdk"
	"github.com/coremesh-lab/coremesh/pkg/sdk/metamodel"
	"github.com/coremesh-lab/coremesh/pkg/sdk/module"
	"github.com/coremesh-lab/coremesh/pkg/sdk/tagservice"
)

// testHost: SQLite mit einer Verbindung, Geschäftspartner, Buchungskreise,
// Berechtigungen (Account.Granted) und Übersetzungen als Attrappe.
type testHost struct {
	db       *sql.DB
	partners []string
	granted  map[string][]string // action → Buchungskreise ("*" = alle)
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
	case "BusinessPartner.get":
		if slices.Contains(h.partners, fmt.Sprint(p["id"])) {
			return sdk.Response{Payload: map[string]any{"id": p["id"]}}, nil
		}
		return sdk.Response{}, fmt.Errorf("%w: Partner %v", sdk.ErrNotFound, p["id"])
	case "Contract.get": // Verträge mit Vertragsart (Bedingungen von Tag Sets)
		if ct, ok := map[string]string{"c1": "RENT", "c2": "LOAN"}[fmt.Sprint(p["id"])]; ok {
			return sdk.Response{Payload: map[string]any{"_id": p["id"], "id": p["id"], "contract_type": ct, "active": true}}, nil
		}
		return sdk.Response{}, fmt.Errorf("%w: Vertrag %v", sdk.ErrNotFound, p["id"])
	case "RentalObject.get": // Ziel von Verweis-Tags
		if p["id"] == "ro1" {
			return sdk.Response{Payload: map[string]any{"id": "ro1", "name": "Wohnung 3. OG"}}, nil
		}
		return sdk.Response{}, fmt.Errorf("%w: Mietobjekt %v", sdk.ErrNotFound, p["id"])
	case "Catalog.GetDefinition":
		if d, ok := testDefs[fmt.Sprint(p["object"])]; ok {
			return sdk.Response{Payload: map[string]any{"definition": d, "available": true}}, nil
		}
		return sdk.Response{}, fmt.Errorf("%w: keine Definition für %v", sdk.ErrNotFound, p["object"])
	case "CompanyCode.get":
		if id := fmt.Sprint(p["id"]); id == "1000" || id == "2000" {
			return sdk.Response{Payload: map[string]any{"code": id}}, nil
		}
		return sdk.Response{}, sdk.ErrNotFound
	case "Account.Granted":
		g := h.granted[fmt.Sprint(p["action"])]
		if slices.Contains(g, "*") {
			return sdk.Response{Payload: map[string]any{"all": true, "company_codes": []any{}}}, nil
		}
		ccs := []any{}
		for _, c := range g {
			ccs = append(ccs, c)
		}
		return sdk.Response{Payload: map[string]any{"all": false, "company_codes": ccs}}, nil
	case "Catalog.Translations":
		if p["locale"] == "en" {
			return sdk.Response{Payload: map[string]any{"translations": map[string]any{"businesspartner.tags.RISK": "Risk class", "businesspartner.tags.RISK.HIGH": "High"}}}, nil
		}
		return sdk.Response{Payload: map[string]any{"translations": map[string]any{}}}, nil
	}
	return sdk.Response{}, fmt.Errorf("%w: %s.%s", sdk.ErrUnimplemented, req.Object, req.Action)
}

type env struct {
	t   *testing.T
	p   *module.Plugin
	h   *testHost
	ctx context.Context
	svc tagservice.Service
}

func setup(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	h := &testHost{db: db, partners: []string{"bp1", "bp2"}, granted: map[string][]string{"update": {"*"}}}
	p := module.NewPlugin(module.Info{Name: "tag", Version: "test"}, New())
	if err := p.Err(); err != nil {
		t.Fatal(err)
	}
	hctx := sdk.WithHost(ctx, h)
	if err := p.Configure(hctx, sdk.Config{Host: h}); err != nil {
		t.Fatal(err)
	}
	resp, err := p.Handle(ctx, sdk.Request{Object: sdk.ObjectDBSchema, Action: sdk.ActionInit, Payload: sdk.SchemaInitRequest{Module: "tag"}})
	if err != nil {
		t.Fatal(err)
	}
	drv, err := sqlite.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	var desired schema.Schema
	if err := sqlite.EvalHCLBytes([]byte(resp.Payload.(sdk.SchemaInitResponse).Schema), &desired, nil); err != nil {
		t.Fatalf("schemaHCL: %v", err)
	}
	desired.Name = "main"
	for _, tb := range desired.Tables {
		tb.Schema = &desired
		if !strings.HasPrefix(tb.Name, "tag__") {
			t.Fatalf("Tabelle ohne Präfix: %s", tb.Name)
		}
	}
	current, _ := drv.InspectSchema(ctx, "main", nil)
	changes, err := drv.SchemaDiff(current, &desired)
	if err != nil {
		t.Fatal(err)
	}
	if err := drv.ApplyChanges(ctx, changes); err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, p: p, h: h, ctx: hctx}
	e.svc = tagservice.New(pluginServices{p})
	return e
}

// pluginServices leitet Client-Aufrufe direkt an das Plugin (statt Dispatcher).
type pluginServices struct{ p *module.Plugin }

func (s pluginServices) Call(ctx context.Context, object, action string, payload any) (sdk.Response, error) {
	return s.p.Handle(ctx, sdk.Request{Object: object, Action: action, Payload: payload})
}

func (e *env) must(object, action string, data map[string]any) map[string]any {
	e.t.Helper()
	resp, err := e.p.Handle(e.ctx, sdk.Request{Object: object, Action: action, Payload: map[string]any{"data": data}})
	if err != nil {
		e.t.Fatalf("%s.%s: %v", object, action, err)
	}
	m, _ := resp.Payload.(map[string]any)
	return m
}

func expect(t *testing.T, err error, target error, what string) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("%s: %v erwartet, bekommen %v", what, target, err)
	}
}

// define legt das Beispiel an: Tag Set RISK_SET (global) mit Risikoklasse,
// Kreditlimit und Prüfdatum samt Regeln, und COMPLIANCE_1000 (nur Buchungskreis 1000).
func (e *env) define() {
	e.must("TagType", "create", map[string]any{"code": "RISK", "name": "Risikoklasse", "data_type": "STRING", "value_mode": "OPTIONS", "translation_key": "businesspartner.tags.RISK"})
	e.must("TagValueOption", "create", map[string]any{"tag_type_code": "RISK", "code": "LOW", "label": "Niedrig", "valid_from": "2000-01-01"})
	e.must("TagValueOption", "create", map[string]any{"tag_type_code": "RISK", "code": "HIGH", "label": "Hoch", "translation_key": "businesspartner.tags.RISK.HIGH",
		"valid_from": "2000-01-01", "valid_to": "2027-06-30"})
	e.must("TagType", "create", map[string]any{"code": "CREDIT_LIMIT", "name": "Kreditlimit", "data_type": "CURRENCY", "value_mode": "FREE"})
	e.must("TagType", "create", map[string]any{"code": "AUDIT_DATE", "name": "Prüfdatum", "data_type": "DATE", "value_mode": "FREE"})
	e.must("TagType", "create", map[string]any{"code": "EMPLOYEES", "name": "Mitarbeitende", "data_type": "INTEGER", "value_mode": "FREE"})
	e.must("TagType", "create", map[string]any{"code": "CERTIFIED_AT", "name": "Zertifiziert am", "data_type": "TIMESTAMP", "value_mode": "FREE"})

	e.must("TagSet", "create", map[string]any{"code": "RISK_SET", "name": "Risiko", "valid_from": "2000-01-01"})
	for i, tag := range []string{"RISK", "CREDIT_LIMIT", "AUDIT_DATE", "CERTIFIED_AT"} {
		e.must("TagSetItem", "create", map[string]any{"tag_set_code": "RISK_SET", "tag_type_code": tag, "mandatory": tag == "RISK", "sort_order": i, "valid_from": "2000-01-01"})
	}
	e.must("TagSetRule", "create", map[string]any{"tag_set_code": "RISK_SET", "rule_type": "REQUIRES", "source_tag": "RISK", "condition_value": "HIGH", "target_tag": "AUDIT_DATE", "valid_from": "2000-01-01"})
	e.must("TagSetRule", "create", map[string]any{"tag_set_code": "RISK_SET", "rule_type": "SHOW_IF", "source_tag": "RISK", "condition_value": "HIGH", "target_tag": "CREDIT_LIMIT", "valid_from": "2000-01-01"})
	e.must("TagSetRule", "create", map[string]any{"tag_set_code": "RISK_SET", "rule_type": "EXCLUDES", "source_tag": "AUDIT_DATE", "target_tag": "CERTIFIED_AT", "valid_from": "2000-01-01"})
	e.must("TagSetAssignment", "create", map[string]any{"entity_type": "BusinessPartner", "company_code": "*", "tag_set_code": "RISK_SET", "valid_from": "2000-01-01"})

	e.must("TagSet", "create", map[string]any{"code": "COMPLIANCE_1000", "name": "Compliance 1000", "valid_from": "2000-01-01"})
	e.must("TagSetItem", "create", map[string]any{"tag_set_code": "COMPLIANCE_1000", "tag_type_code": "EMPLOYEES", "valid_from": "2000-01-01"})
	e.must("TagSetAssignment", "create", map[string]any{"entity_type": "BusinessPartner", "company_code": "1000", "tag_set_code": "COMPLIANCE_1000", "valid_from": "2000-01-01"})
}

func codes(s tagservice.Schema) string {
	var out []string
	for _, set := range s.Sets {
		for _, it := range set.Items {
			out = append(out, it.Tag.Code+"@"+it.Scope)
		}
	}
	return strings.Join(out, ",")
}

func TestDefinitionsValidated(t *testing.T) {
	e := setup(t)
	e.define()
	for name, tc := range map[string]struct {
		object string
		data   map[string]any
	}{
		"Code-Format":               {"TagType", map[string]any{"code": "risk", "name": "x", "data_type": "STRING", "value_mode": "FREE"}},
		"Auswahl bei CURRENCY":      {"TagType", map[string]any{"code": "AMOUNT", "name": "x", "data_type": "CURRENCY", "value_mode": "OPTIONS"}},
		"Option bei freiem Tag":     {"TagValueOption", map[string]any{"tag_type_code": "AUDIT_DATE", "code": "X", "label": "x"}},
		"Regel mit fremdem Tag":     {"TagSetRule", map[string]any{"tag_set_code": "RISK_SET", "rule_type": "REQUIRES", "source_tag": "RISK", "target_tag": "EMPLOYEES", "valid_from": "2000-01-01"}},
		"unbekannter Buchungskreis": {"TagSetAssignment", map[string]any{"entity_type": "BusinessPartner", "company_code": "9999", "tag_set_code": "RISK_SET", "valid_from": "2000-01-01"}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := e.p.Handle(e.ctx, sdk.Request{Object: tc.object, Action: "create", Payload: map[string]any{"data": tc.data}})
			expect(t, err, sdk.ErrInvalidArgument, name)
		})
	}
	// Metamodelle gültig, Übersetzungen vollständig.
	resp, _ := e.p.Handle(e.ctx, sdk.Request{Object: sdk.ObjectCatalog, Action: sdk.ActionDescribe})
	d := resp.Payload.(metamodel.DescribeResponse)
	for _, o := range d.Objects {
		if err := o.Validate(); err != nil {
			t.Error(err)
		}
		for _, f := range o.Fields {
			for _, loc := range metamodel.Locales {
				if d.Translations[loc][f.LabelKey] == "" {
					t.Errorf("%s: %s fehlt", loc, f.LabelKey)
				}
			}
		}
	}
}

func TestSchemaPerCompanyCode(t *testing.T) {
	e := setup(t)
	e.define()
	s, err := e.svc.Schema(e.ctx, tagservice.GetRequest{EntityType: "BusinessPartner", EffectiveDate: "2026-10-01"})
	if err != nil {
		t.Fatal(err)
	}
	if got := codes(s); got != "RISK@*,CREDIT_LIMIT@*,AUDIT_DATE@*,CERTIFIED_AT@*" {
		t.Fatalf("global: %s", got)
	}
	s, _ = e.svc.Schema(e.ctx, tagservice.GetRequest{EntityType: "BusinessPartner", CompanyCode: "1000", EffectiveDate: "2026-10-01", Locale: "en"})
	if got := codes(s); !strings.HasSuffix(got, ",EMPLOYEES@1000") || len(s.Sets) != 2 {
		t.Fatalf("Buchungskreis 1000: %s", got)
	}
	if o := s.Sets[0].Items[0].Tag.Options; s.Sets[0].Items[0].Tag.Name != "Risk class" || o[0].Label != "High" || o[1].Label != "Niedrig" {
		t.Fatalf("Übersetzung über translation_key: %+v", s.Sets[0].Items[0].Tag)
	}
	if s, _ := e.svc.Schema(e.ctx, tagservice.GetRequest{EntityType: "BusinessPartner", CompanyCode: "2000"}); strings.Contains(codes(s), "EMPLOYEES") {
		t.Fatal("Tag Set von 1000 in Buchungskreis 2000 sichtbar")
	}
	// Auswahlwert HIGH gilt bis 2027-06-30.
	s, _ = e.svc.Schema(e.ctx, tagservice.GetRequest{EntityType: "BusinessPartner", EffectiveDate: "2027-07-01"})
	if opts := s.Sets[0].Items[0].Tag.Options; len(opts) != 1 || opts[0].Code != "LOW" {
		t.Fatalf("abgelaufener Auswahlwert: %+v", opts)
	}
}

func violations(t *testing.T, e *env, req tagservice.SetRequest) string {
	t.Helper()
	vs, err := e.svc.Validate(e.ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, v := range vs {
		out = append(out, v.Tag+":"+v.Code)
	}
	slices.Sort(out)
	return strings.Join(out, ",")
}

func TestTypesAndRules(t *testing.T) {
	e := setup(t)
	e.define()
	req := func(vals map[string]*tagservice.Value) tagservice.SetRequest {
		return tagservice.SetRequest{EntityType: "BusinessPartner", EntityID: "bp1", ValidFrom: "2026-01-01", Values: vals}
	}
	for want, vals := range map[string]map[string]*tagservice.Value{
		"RISK:required":                    {"AUDIT_DATE": tagservice.Date("2026-03-01")},
		"AUDIT_DATE:required":              {"RISK": tagservice.Option("HIGH")},
		"CREDIT_LIMIT:hidden":              {"RISK": tagservice.Option("LOW"), "CREDIT_LIMIT": tagservice.Money("1000", "EUR")},
		"CERTIFIED_AT:excludes":            {"RISK": tagservice.Option("LOW"), "AUDIT_DATE": tagservice.Date("2026-03-01"), "CERTIFIED_AT": tagservice.Timestamp("2026-01-05T10:00:00+01:00")},
		"CREDIT_LIMIT:type,RISK:required":  {"CREDIT_LIMIT": tagservice.Money("12,50", "EUR")},
		"CREDIT_LIMIT:type,RISK:required ": {"CREDIT_LIMIT": tagservice.Money("12.50", "EURO")},
		"RISK:option":                      {"RISK": tagservice.Option("MEDIUM")},
		"AUDIT_DATE:type,RISK:required":    {"AUDIT_DATE": tagservice.String("morgen")},
		"EMPLOYEES:unknown,RISK:required":  {"EMPLOYEES": tagservice.Integer(5)}, // nur in Buchungskreis 1000
	} {
		if got := violations(t, e, req(vals)); got != strings.TrimSpace(want) {
			t.Errorf("%v: %s, erwartet %s", vals, got, want)
		}
	}

	out, err := e.svc.Set(e.ctx, req(map[string]*tagservice.Value{
		"RISK": tagservice.Option("HIGH"), "AUDIT_DATE": tagservice.Date("2026-03-01"), "CREDIT_LIMIT": tagservice.Money("1250.50", "eur"),
	}))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]tagservice.Assignment{}
	for _, a := range out.Values {
		got[a.Tag] = a
	}
	if *got["CREDIT_LIMIT"].Value.Amount != "1250.50" || *got["CREDIT_LIMIT"].Value.Currency != "EUR" || got["RISK"].OptionLabel != "Hoch" ||
		!out.State.Visible["CREDIT_LIMIT"] || !out.State.Required["AUDIT_DATE"] {
		t.Fatalf("Werte/Zustand: %+v", out)
	}
	// Verstoß beim Schreiben: nichts geschrieben.
	_, err = e.svc.Set(e.ctx, req(map[string]*tagservice.Value{"AUDIT_DATE": nil}))
	expect(t, err, sdk.ErrInvalidArgument, "REQUIRES beim Entfernen")
}

func TestHistoryAndEffectiveDate(t *testing.T) {
	e := setup(t)
	e.define()
	set := func(from string, vals map[string]*tagservice.Value) error {
		_, err := e.svc.Set(e.ctx, tagservice.SetRequest{EntityType: "BusinessPartner", EntityID: "bp1", ValidFrom: from, Values: vals})
		return err
	}
	if err := set("2026-01-01", map[string]*tagservice.Value{"RISK": tagservice.Option("HIGH"), "AUDIT_DATE": tagservice.Date("2026-03-01"), "CREDIT_LIMIT": tagservice.Money("5000", "CHF")}); err != nil {
		t.Fatal(err)
	}
	// Ab 2027: niedriges Risiko – Kreditlimit ist dann nicht mehr vorgesehen.
	if err := set("2027-01-01", map[string]*tagservice.Value{"RISK": tagservice.Option("LOW"), "CREDIT_LIMIT": nil}); err != nil {
		t.Fatal(err)
	}
	at := func(date string) string {
		et, err := e.svc.Get(e.ctx, tagservice.GetRequest{EntityType: "BusinessPartner", EntityID: "bp1", EffectiveDate: date})
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, a := range et.Values {
			out = append(out, a.Tag+"="+text(a.Value))
		}
		return strings.Join(out, ",")
	}
	if got := at("2026-12-31"); got != "AUDIT_DATE=2026-03-01,CREDIT_LIMIT=5000 CHF,RISK=HIGH" {
		t.Fatalf("Stichtag 2026: %s", got)
	}
	if got := at("2027-06-01"); got != "AUDIT_DATE=2026-03-01,RISK=LOW" {
		t.Fatalf("Stichtag 2027: %s", got)
	}
	hist, _ := e.svc.History(e.ctx, tagservice.GetRequest{EntityType: "BusinessPartner", EntityID: "bp1"}, "RISK")
	if len(hist) != 2 || hist[0].ValidTo != "2026-12-31" || hist[1].ValidFrom != "2027-01-01" || hist[0].ID != hist[1].ID {
		t.Fatalf("Historie: %+v", hist)
	}
	// Vor einer künftigen Zeitscheibe wird nicht überschrieben.
	err := set("2026-06-01", map[string]*tagservice.Value{"RISK": tagservice.Option("LOW"), "CREDIT_LIMIT": nil})
	expect(t, err, sdk.ErrInvalidArgument, "künftige Zeitscheibe")
	// Abgelaufener Auswahlwert HIGH (bis 2027-06-30) ist danach nicht wählbar.
	vs := violations(t, e, tagservice.SetRequest{EntityType: "BusinessPartner", EntityID: "bp1", ValidFrom: "2027-07-01",
		Values: map[string]*tagservice.Value{"RISK": tagservice.Option("HIGH"), "CREDIT_LIMIT": tagservice.Money("1", "EUR")}})
	if !strings.Contains(vs, "RISK:option") {
		t.Fatalf("abgelaufener Auswahlwert: %s", vs)
	}
	// Suche zum Stichtag.
	if ids, _ := e.svc.Find(e.ctx, tagservice.FindRequest{EntityType: "BusinessPartner", Tag: "RISK", Value: tagservice.Option("HIGH"), EffectiveDate: "2026-06-01"}); len(ids) != 1 || ids[0] != "bp1" {
		t.Fatalf("Suche: %v", ids)
	}
	if ids, _ := e.svc.Find(e.ctx, tagservice.FindRequest{EntityType: "BusinessPartner", Tag: "RISK", Value: tagservice.Option("HIGH"), EffectiveDate: "2027-06-01"}); len(ids) != 0 {
		t.Fatalf("Suche 2027: %v", ids)
	}
}

func TestCompanyCodeScopeAndAccess(t *testing.T) {
	e := setup(t)
	e.define()
	base := map[string]*tagservice.Value{"RISK": tagservice.Option("LOW")}
	if _, err := e.svc.Set(e.ctx, tagservice.SetRequest{EntityType: "BusinessPartner", EntityID: "bp1", ValidFrom: "2026-01-01", Values: base}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Set(e.ctx, tagservice.SetRequest{EntityType: "BusinessPartner", EntityID: "bp1", CompanyCode: "1000", ValidFrom: "2026-01-01",
		Values: map[string]*tagservice.Value{"EMPLOYEES": tagservice.Integer(42)}}); err != nil {
		t.Fatal(err)
	}
	get := func(cc string) string {
		et, err := e.svc.Get(e.ctx, tagservice.GetRequest{EntityType: "BusinessPartner", EntityID: "bp1", CompanyCode: cc, EffectiveDate: "2026-06-01"})
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, a := range et.Values {
			out = append(out, a.Tag+"@"+a.CompanyCode+"="+text(a.Value))
		}
		return strings.Join(out, ",")
	}
	if got := get("1000"); got != "EMPLOYEES@1000=42,RISK@*=LOW" {
		t.Fatalf("Buchungskreis 1000: %s", got)
	}
	if got := get("2000"); got != "RISK@*=LOW" {
		t.Fatalf("Buchungskreis 2000: %s", got)
	}

	// Rechte: update nur in 2000 → kein Schreiben in 1000 und keine globalen Werte.
	e.h.granted["update"] = []string{"2000"}
	_, err := e.svc.Set(e.ctx, tagservice.SetRequest{EntityType: "BusinessPartner", EntityID: "bp1", CompanyCode: "1000", ValidFrom: "2026-02-01",
		Values: map[string]*tagservice.Value{"EMPLOYEES": tagservice.Integer(43)}})
	expect(t, err, sdk.ErrPermissionDenied, "Buchungskreis ohne Recht")
	_, err = e.svc.Set(e.ctx, tagservice.SetRequest{EntityType: "BusinessPartner", EntityID: "bp1", ValidFrom: "2026-02-01",
		Values: map[string]*tagservice.Value{"RISK": tagservice.Option("HIGH"), "AUDIT_DATE": tagservice.Date("2026-02-01")}})
	expect(t, err, sdk.ErrPermissionDenied, "globaler Wert ohne Recht für alle Buchungskreise")

	// Unbekannter Datensatz: über die Action des Fachmoduls geprüft.
	_, err = e.svc.Get(e.ctx, tagservice.GetRequest{EntityType: "BusinessPartner", EntityID: "gibtsnicht"})
	expect(t, err, sdk.ErrNotFound, "unbekannter Partner")
	_, err = e.svc.Get(e.ctx, tagservice.GetRequest{EntityType: "Unbekannt", EntityID: "x"})
	expect(t, err, sdk.ErrInvalidArgument, "Objekttyp ohne get")
}

func TestDeprecatedTag(t *testing.T) {
	e := setup(t)
	e.define()
	set := func(from string, vals map[string]*tagservice.Value) error {
		_, err := e.svc.Set(e.ctx, tagservice.SetRequest{EntityType: "BusinessPartner", EntityID: "bp2", ValidFrom: from, Values: vals})
		return err
	}
	if err := set("2026-01-01", map[string]*tagservice.Value{"RISK": tagservice.Option("LOW"), "CERTIFIED_AT": tagservice.Timestamp("2026-01-05T10:00:00+01:00")}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.p.Handle(e.ctx, sdk.Request{Object: "TagType", Action: "deactivate", Payload: map[string]any{"id": "CERTIFIED_AT"}}); err != nil {
		t.Fatal(err)
	}
	// Bestandswert bleibt lesbar (in UTC gespeichert), neue Werte werden abgelehnt.
	et, _ := e.svc.Get(e.ctx, tagservice.GetRequest{EntityType: "BusinessPartner", EntityID: "bp2", EffectiveDate: "2026-06-01"})
	var ts string
	for _, a := range et.Values {
		if a.Tag == "CERTIFIED_AT" {
			ts = *a.Value.Timestamp
		}
	}
	if ts != "2026-01-05T09:00:00Z" {
		t.Fatalf("Bestandswert: %q", ts)
	}
	err := set("2026-07-01", map[string]*tagservice.Value{"CERTIFIED_AT": tagservice.Timestamp("2026-07-01T08:00:00Z")})
	expect(t, err, sdk.ErrInvalidArgument, "veralteter Tag")
	// Veraltete Tags lassen sich keinem Tag Set mehr hinzufügen.
	_, err = e.p.Handle(e.ctx, sdk.Request{Object: "TagSetItem", Action: "create", Payload: map[string]any{"data": map[string]any{
		"tag_set_code": "COMPLIANCE_1000", "tag_type_code": "CERTIFIED_AT", "valid_from": "2026-01-01"}}})
	expect(t, err, sdk.ErrInvalidArgument, "veralteter Tag im Tag Set")
}
