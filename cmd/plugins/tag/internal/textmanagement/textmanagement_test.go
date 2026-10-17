package textmanagement

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"ariga.io/atlas/sql/schema"
	"ariga.io/atlas/sql/sqlite"
	_ "modernc.org/sqlite"

	"github.com/coremesh-labs/coremesh/cmd/plugins/tag/internal/tagmanagement"
	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/module"
	"github.com/coremesh-labs/coremesh/pkg/sdk/textservice"
)

// testHost: SQLite, Verträge (Contract.get) und Rechte (Account.Check) als Attrappe.
type testHost struct {
	db       *sql.DB
	readOnly bool // nur lesen, kein <Object>.update
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

func (h *testHost) BeginTx(context.Context, string, sql.TxOptions) (string, error) { return "tx", nil }
func (h *testHost) CommitTx(context.Context, string) error                         { return nil }
func (h *testHost) RollbackTx(context.Context, string) error                       { return nil }

func (h *testHost) Handle(_ context.Context, req sdk.Request) (sdk.Response, error) {
	p, _ := req.Payload.(map[string]any)
	switch req.Object + "." + req.Action {
	case "Contract.get":
		if ct, ok := map[string]string{"1000|MV-1": "MV", "1000|DL-1": "DL"}[fmt.Sprint(p["id"])]; ok {
			return sdk.Response{Payload: map[string]any{"id": p["id"], "company_code": "1000", "contract_type": ct}}, nil
		}
		return sdk.Response{}, fmt.Errorf("%w: Vertrag %v", sdk.ErrNotFound, p["id"])
	case "Account.Check":
		return sdk.Response{Payload: map[string]any{"allowed": !(h.readOnly && p["action"] == "update")}}, nil
	case "SystemEvent.Register":
		return sdk.Response{Payload: map[string]any{}}, nil
	}
	return sdk.Response{}, fmt.Errorf("%w: %s.%s", sdk.ErrUnimplemented, req.Object, req.Action)
}

type env struct {
	t   *testing.T
	p   *module.Plugin
	h   *testHost
	ctx context.Context
	svc textservice.Client
}

type pluginServices struct{ p *module.Plugin }

func (s pluginServices) Call(ctx context.Context, object, action string, payload any) (sdk.Response, error) {
	return s.p.Handle(ctx, sdk.Request{Object: object, Action: action, Payload: payload})
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
	h := &testHost{db: db}
	// wie im Plugin tag: beide Module, getrennte Tabellen-Präfixe
	p := module.NewPlugin(module.Info{Name: "tag", Version: "test"}, tagmanagement.New(), New())
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
	init := resp.Payload.(sdk.SchemaInitResponse)
	drv, err := sqlite.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	var desired schema.Schema
	if err := sqlite.EvalHCLBytes([]byte(init.Schema), &desired, nil); err != nil {
		t.Fatalf("schemaHCL: %v", err)
	}
	desired.Name = "main"
	for _, tb := range desired.Tables {
		tb.Schema = &desired
	}
	current, _ := drv.InspectSchema(ctx, "main", nil)
	changes, err := drv.SchemaDiff(current, &desired)
	if err != nil {
		t.Fatal(err)
	}
	if err := drv.ApplyChanges(ctx, changes); err != nil {
		t.Fatal(err)
	}
	for _, s := range init.Seed {
		for _, r := range s.Rows {
			var cols, marks []string
			var args []any
			for k, v := range r {
				cols, marks, args = append(cols, k), append(marks, "?"), append(args, v)
			}
			if _, err := db.Exec("INSERT INTO "+s.Table+" ("+strings.Join(cols, ", ")+") VALUES ("+strings.Join(marks, ", ")+")", args...); err != nil {
				t.Fatal(err)
			}
		}
	}
	return &env{t: t, p: p, h: h, ctx: hctx, svc: textservice.New(pluginServices{p})}
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

func invalid(t *testing.T, err error, what string) {
	t.Helper()
	if !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("%s: ungültige Eingabe erwartet, bekommen %v", what, err)
	}
}

func TestTexts(t *testing.T) {
	e := setup(t)
	ctx := e.ctx
	mv := textservice.Text{EntityType: "Contract", EntityID: "1000|MV-1", Language: "de"}
	with := func(cat, content string, pos int) textservice.Text {
		x := mv
		x.Category, x.Content, x.Position = cat, content, pos
		return x
	}
	// ohne Zuordnung nicht erlaubt
	_, err := e.svc.Save(ctx, with("NOTE", "Schlüssel beim Hausmeister", 0))
	invalid(t, err, "Kategorie ohne Zuordnung")
	e.must("TextAssignment", "create", map[string]any{"entity_type": "Contract", "company_code": "*", "category": "NOTE"})
	e.must("TextAssignment", "create", map[string]any{"entity_type": "Contract", "company_code": "1000", "category": "DESCRIPTION",
		"mandatory": true, "condition_field": "contract_type", "condition_values": "MV, GM"})

	l, err := e.svc.List(ctx, textservice.ListRequest{EntityType: "Contract", EntityID: "1000|MV-1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Categories) != 2 || len(l.Missing) != 1 || l.Missing[0].Code != "DESCRIPTION" || !l.CanEdit || len(l.Languages) != 2 {
		t.Fatalf("Liste: %+v", l)
	}
	// Bedingung: Dienstleistungsvertrag hat keine Beschreibung
	if l, _ := e.svc.List(ctx, textservice.ListRequest{EntityType: "Contract", EntityID: "1000|DL-1"}); len(l.Categories) != 1 || len(l.Missing) != 0 {
		t.Fatalf("Bedingung: %+v", l)
	}
	// mehrfach: nächste freie Position
	n1, err := e.svc.Save(ctx, with("NOTE", "Schlüssel beim Hausmeister", 0))
	if err != nil || n1.Position != 1 {
		t.Fatalf("Anmerkung 1: %+v %v", n1, err)
	}
	if n2, err := e.svc.Save(ctx, with("NOTE", "Hund erlaubt", 0)); err != nil || n2.Position != 2 {
		t.Fatalf("Anmerkung 2: %+v %v", n2, err)
	}
	// Übersetzung auf derselben Position, doppelt verboten
	if en, err := e.svc.Save(ctx, textservice.Text{EntityType: "Contract", EntityID: "1000|MV-1", Category: "NOTE", Position: 1, Language: "en",
		Content: "Key with the caretaker"}); err != nil || en.Position != 1 {
		t.Fatalf("Übersetzung: %+v %v", en, err)
	}
	_, err = e.svc.Save(ctx, with("NOTE", "doppelt", 1))
	invalid(t, err, "Position und Sprache doppelt")
	// einmalig: nur Position 1
	_, err = e.svc.Save(ctx, with("DESCRIPTION", "zweite", 2))
	invalid(t, err, "einmalige Kategorie auf Position 2")
	d, err := e.svc.Save(ctx, with("DESCRIPTION", "3-Zimmer-Wohnung mit Balkon\n", 0))
	if err != nil || d.Position != 1 || d.Content != "3-Zimmer-Wohnung mit Balkon" {
		t.Fatalf("Beschreibung: %+v %v", d, err)
	}
	_, err = e.svc.Save(ctx, with("DESCRIPTION", "noch eine", 0))
	invalid(t, err, "einmalige Kategorie zweimal")
	if c, err := e.svc.Check(ctx, textservice.CheckRequest{EntityType: "Contract", EntityID: "1000|MV-1"}); err != nil || len(c.Missing) != 0 {
		t.Fatalf("Pflicht erfüllt: %+v %v", c, err)
	}
	// unbekannte Sprache, leerer Text
	x := with("NOTE", "Hallo", 0)
	x.Language = "fr"
	_, err = e.svc.Save(ctx, x)
	invalid(t, err, "unbekannte Sprache")
	_, err = e.svc.Save(ctx, with("NOTE", "  ", 0))
	invalid(t, err, "leerer Text")
	// ändern: Inhalt
	n1.Content = "Schlüssel bei Frau Müller"
	if u, err := e.svc.Save(ctx, textservice.Text{ID: n1.ID, Content: n1.Content}); err != nil || u.Content != n1.Content || u.Language != "de" || u.Position != 1 {
		t.Fatalf("ändern: %+v %v", u, err)
	}
	// Liste: Kategorie, Position, Sprache
	l, _ = e.svc.List(ctx, textservice.ListRequest{EntityType: "Contract", EntityID: "1000|MV-1"})
	var got []string
	for _, it := range l.Items {
		got = append(got, fmt.Sprintf("%s/%d/%s", it.Category, it.Position, it.Language))
	}
	if want := "NOTE/1/de|NOTE/1/en|NOTE/2/de|DESCRIPTION/1/de"; strings.Join(got, "|") != want {
		t.Fatalf("Reihenfolge %s, erwartet %s", strings.Join(got, "|"), want)
	}
	if l, _ := e.svc.List(ctx, textservice.ListRequest{EntityType: "Contract", EntityID: "1000|MV-1", Language: "en"}); len(l.Items) != 1 {
		t.Fatalf("nur Englisch: %d", len(l.Items))
	}
	// einmal ↔ mehrfach: mit Position 2 nicht auf einmal umstellbar
	_, err = e.p.Handle(ctx, sdk.Request{Object: "TextCategory", Action: "update", Payload: map[string]any{"id": "NOTE", "data": map[string]any{"cardinality": "SINGLE"}}})
	invalid(t, err, "auf einmal umstellen mit Position 2")
	// Rechte: nur lesen
	e.h.readOnly = true
	if l, err := e.svc.List(ctx, textservice.ListRequest{EntityType: "Contract", EntityID: "1000|MV-1"}); err != nil || l.CanEdit {
		t.Fatalf("nur lesen: %v %v", l.CanEdit, err)
	}
	if _, err := e.svc.Save(ctx, with("NOTE", "x", 0)); !errors.Is(err, sdk.ErrPermissionDenied) {
		t.Fatalf("schreiben ohne Recht: %v", err)
	}
	e.h.readOnly = false
	if err := e.svc.Remove(ctx, textservice.RemoveRequest{ID: d.ID}); err != nil {
		t.Fatal(err)
	}
	if c, _ := e.svc.Check(ctx, textservice.CheckRequest{EntityType: "Contract", EntityID: "1000|MV-1"}); len(c.Missing) != 1 {
		t.Fatalf("Pflicht nach Entfernen: %+v", c)
	}
	_, err = e.svc.List(ctx, textservice.ListRequest{EntityType: "Contract", EntityID: "1000|XX"})
	invalid(t, err, "unbekannter Datensatz")
}

func (h *testHost) Read(_ context.Context, req sdk.Request, _ sdk.RowWriter) (sdk.ReadEnd, error) {
	return sdk.ReadEnd{}, fmt.Errorf("%w: %s.%s", sdk.ErrUnimplemented, req.Object, req.Action)
}

func (s pluginServices) Read(ctx context.Context, object, action string, payload any, w sdk.RowWriter) (sdk.ReadEnd, error) {
	return s.p.Read(ctx, sdk.Request{Object: object, Action: action, Payload: payload}, w)
}

// Konsole: Eingaben in "data".
func TestTextsDataPayload(t *testing.T) {
	e := setup(t)
	e.must("TextAssignment", "create", map[string]any{"entity_type": "Contract", "company_code": "*", "category": "NOTE"})
	out := e.must(textservice.Object, textservice.ActionSave, map[string]any{"entity_type": "Contract", "entity_id": "1000|MV-1",
		"category": "NOTE", "language": "de", "content": "über die Konsole"})
	if out == nil {
		if l, _ := e.svc.List(e.ctx, textservice.ListRequest{EntityType: "Contract", EntityID: "1000|MV-1"}); len(l.Items) != 1 {
			t.Fatalf("Text über data: %+v", l)
		}
	}
}

func TestTextValidity(t *testing.T) {
	e := setup(t)
	ctx := e.ctx
	e.must("TextAssignment", "create", map[string]any{"entity_type": "Contract", "company_code": "*", "category": "DESCRIPTION", "mandatory": true})
	d := func(content, from, to string) textservice.Text {
		return textservice.Text{EntityType: "Contract", EntityID: "1000|MV-1", Language: "de", Category: "DESCRIPTION", Content: content, ValidFrom: from, ValidTo: to}
	}
	old, err := e.svc.Save(ctx, d("alt", "", "2026-12-31"))
	if err != nil || old.ValidTo != "2026-12-31" || old.Position != 1 {
		t.Fatalf("alt: %+v %v", old, err)
	}
	_, err = e.svc.Save(ctx, d("überschneidet", "2026-06-01", ""))
	invalid(t, err, "Zeitraum überschneidet sich")
	_, err = e.svc.Save(ctx, d("verkehrt", "2027-02-01", "2027-01-01"))
	invalid(t, err, "bis vor ab")
	if n, err := e.svc.Save(ctx, d("neu", "2027-01-01", "")); err != nil || n.Position != 1 || n.ValidFrom != "2027-01-01" {
		t.Fatalf("neue Fassung: %+v %v", n, err)
	}
	at := func(date string) []textservice.Text {
		l, err := e.svc.List(ctx, textservice.ListRequest{EntityType: "Contract", EntityID: "1000|MV-1", EffectiveDate: date})
		if err != nil {
			t.Fatal(err)
		}
		return l.Items
	}
	if all := at(""); len(all) != 2 || all[0].Content != "alt" {
		t.Fatalf("alle Fassungen: %+v", all)
	}
	if l := at("2027-03-01"); len(l) != 1 || l[0].Content != "neu" {
		t.Fatalf("Stichtag 2027: %+v", l)
	}
	// Pflicht je Stichtag: Lücke nach Ende der alten Fassung, wenn die neue fehlt
	if err := e.svc.Remove(ctx, textservice.RemoveRequest{ID: at("2027-03-01")[0].ID}); err != nil {
		t.Fatal(err)
	}
	for date, want := range map[string]int{"2026-07-01": 0, "2027-03-01": 1} {
		if c, err := e.svc.Check(ctx, textservice.CheckRequest{EntityType: "Contract", EntityID: "1000|MV-1", EffectiveDate: date}); err != nil || len(c.Missing) != want {
			t.Fatalf("Pflicht am %s: %+v %v", date, c, err)
		}
	}
}
