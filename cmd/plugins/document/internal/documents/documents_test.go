package documents

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

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/docservice"
	"github.com/coremesh-labs/coremesh/pkg/sdk/events"
	"github.com/coremesh-labs/coremesh/pkg/sdk/module"
)

// testHost: SQLite, Ziel-Datensätze (Contract.get) und Rechte (Account.Check:
// action → Buchungskreise).
type testHost struct {
	db      *sql.DB
	records map[string]string   // Contract-ID → Buchungskreis
	granted map[string][]string // read/update → Buchungskreise ("*" = alle)
}

func (h *testHost) Log(context.Context, sdk.LogLevel, string, map[string]string) error { return nil }

func (h *testHost) Handle(_ context.Context, req sdk.Request) (sdk.Response, error) {
	var p map[string]any
	_ = sdk.Decode(req.Payload, &p)
	switch req.Object + "." + req.Action {
	case "Contract.get":
		cc, ok := h.records[fmt.Sprint(p["id"])]
		if !ok {
			return sdk.Response{}, fmt.Errorf("%w: Vertrag %v", sdk.ErrNotFound, p["id"])
		}
		return sdk.Response{Payload: map[string]any{"id": p["id"], "company_code": cc}}, nil
	case "Account.Check":
		g := h.granted[fmt.Sprint(p["action"])]
		attrs, _ := p["attrs"].(map[string]any)
		cc := fmt.Sprint(attrs["company_code"])
		return sdk.Response{Payload: map[string]any{"allowed": slices.Contains(g, "*") || slices.Contains(g, cc)}}, nil
	case "SystemEvent.Register":
		return sdk.Response{Payload: map[string]any{}}, nil
	}
	return sdk.Response{}, fmt.Errorf("%w: %s.%s", sdk.ErrUnimplemented, req.Object, req.Action)
}

func (h *testHost) Read(context.Context, sdk.Request, sdk.RowWriter) (sdk.ReadEnd, error) {
	return sdk.ReadEnd{}, sdk.ErrUnimplemented
}

func (h *testHost) Query(ctx context.Context, _ string, q string, args ...any) (*sdk.QueryResult, error) {
	rows, err := h.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	out := &sdk.QueryResult{Columns: cols}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		out.Rows = append(out.Rows, vals)
	}
	return out, rows.Err()
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

type env struct {
	t   *testing.T
	p   *module.Plugin
	h   *testHost
	ctx context.Context
	svc docservice.Client
}

type pluginServices struct{ p *module.Plugin }

func (s pluginServices) Call(ctx context.Context, object, action string, payload any) (sdk.Response, error) {
	return s.p.Handle(ctx, sdk.Request{Object: object, Action: action, Payload: payload})
}

func (s pluginServices) Read(ctx context.Context, object, action string, payload any, w sdk.RowWriter) (sdk.ReadEnd, error) {
	return s.p.Read(ctx, sdk.Request{Object: object, Action: action, Payload: payload}, w)
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
	h := &testHost{db: db, records: map[string]string{"1000|MV-1": "1000", "2000|MV-9": "2000"},
		granted: map[string][]string{"read": {"*"}, "update": {"1000"}}}
	p := module.NewPlugin(module.Info{Name: "document", Version: "test"}, New())
	if err := p.Err(); err != nil {
		t.Fatal(err)
	}
	hctx := sdk.WithHost(ctx, h)
	if err := p.Configure(hctx, sdk.Config{Host: h}); err != nil {
		t.Fatal(err)
	}
	resp, err := p.Handle(ctx, sdk.Request{Object: sdk.ObjectDBSchema, Action: sdk.ActionInit, Payload: sdk.SchemaInitRequest{Module: "document"}})
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
		if !strings.HasPrefix(tb.Name, "document__") {
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
	for _, s := range si.Seed {
		for _, r := range s.Rows {
			cols := slices.Sorted(maps.Keys(r))
			args := make([]any, len(cols))
			for i, c := range cols {
				args[i] = r[c]
			}
			if _, err := db.Exec(fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", s.Table, strings.Join(cols, ", "),
				strings.TrimSuffix(strings.Repeat("?, ", len(cols)), ", ")), args...); err != nil {
				t.Fatal(err)
			}
		}
	}
	ctx = sdk.WithCall(hctx, sdk.CallContext{RequestID: "test", UserID: "tester"})
	return &env{t: t, p: p, h: h, ctx: ctx, svc: docservice.New(pluginServices{p})}
}

func expect(t *testing.T, err error, target error, what string) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("%s: erwartet %v, bekommen %v", what, target, err)
	}
}

// TestDocuments: mehrere Dokumente an einem Vertrag, Gültigkeit, Ändern,
// Entfernen (bleibt gespeichert), Rechte nach dem Ziel-Datensatz, Umschlüsselung.
func TestDocuments(t *testing.T) {
	e := setup(t)
	ctx := e.ctx
	attach := func(id string, d docservice.Document) (docservice.Document, error) {
		return e.svc.Attach(ctx, docservice.AttachRequest{EntityType: "Contract", EntityID: id, Document: d})
	}
	v, err := attach("1000|MV-1", docservice.Document{DocType: "contract", Title: "Mietvertrag", DocDate: "2026-01-02", Location: "vertraege/mv-1.pdf"})
	if err != nil || v.ID == "" || v.DocType != "CONTRACT" || v.DocTypeName != "Vertrag" || v.CreatedBy != "tester" {
		t.Fatalf("Vertrag: %+v %v", v, err)
	}
	if _, err := attach("1000|MV-1", docservice.Document{DocType: "AGB", Title: "AGB 2026", DocDate: "2026-01-02", Location: "agb-2026.pdf",
		ValidFrom: "2026-01-01", ValidTo: "2026-12-31"}); err != nil {
		t.Fatal(err)
	}
	n, err := attach("1000|MV-1", docservice.Document{DocType: "AMENDMENT", Title: "Nachtrag Stellplatz", DocDate: "2026-11-15",
		Location: "https://dms.example/123", ValidFrom: "2027-01-01"})
	if err != nil {
		t.Fatal(err)
	}
	l, err := e.svc.List(ctx, docservice.ListRequest{EntityType: "Contract", EntityID: "1000|MV-1"})
	if err != nil || len(l.Items) != 3 || l.Items[0].Title != "Nachtrag Stellplatz" || !l.CanEdit || len(l.Types) < 10 {
		t.Fatalf("Liste: %+v %v", l, err)
	}
	if l, _ := e.svc.List(ctx, docservice.ListRequest{EntityType: "Contract", EntityID: "1000|MV-1", EffectiveDate: "2027-03-01"}); len(l.Items) != 2 {
		t.Fatalf("gültig am 01.03.2027: %+v", l.Items)
	}

	// Prüfungen
	_, err = attach("1000|MV-1", docservice.Document{DocType: "GIBTSNICHT", Title: "x", Location: "x"})
	expect(t, err, sdk.ErrInvalidArgument, "unbekannte Dokumentart")
	_, err = attach("1000|MV-1", docservice.Document{DocType: "OTHER", Title: "x"})
	expect(t, err, sdk.ErrInvalidArgument, "ohne Ablage")
	_, err = attach("1000|MV-1", docservice.Document{DocType: "OTHER", Title: "x", Location: "x", ValidFrom: "2026-02-01", ValidTo: "2026-01-01"})
	expect(t, err, sdk.ErrInvalidArgument, "bis vor ab")
	_, err = attach("1000|GIBTSNICHT", docservice.Document{DocType: "OTHER", Title: "x", Location: "x"})
	expect(t, err, sdk.ErrInvalidArgument, "unbekannter Datensatz")
	_, err = attach("2000|MV-9", docservice.Document{DocType: "OTHER", Title: "x", Location: "x"})
	expect(t, err, sdk.ErrPermissionDenied, "ohne Änderungsrecht im Buchungskreis")
	if l, err := e.svc.List(ctx, docservice.ListRequest{EntityType: "Contract", EntityID: "2000|MV-9"}); err != nil || l.CanEdit {
		t.Fatalf("Lesen ohne Änderungsrecht: %+v %v", l, err)
	}
	e.h.granted["read"] = []string{"1000"}
	_, err = e.svc.List(ctx, docservice.ListRequest{EntityType: "Contract", EntityID: "2000|MV-9"})
	expect(t, err, sdk.ErrPermissionDenied, "ohne Leserecht")

	// Ändern und Entfernen
	u, err := e.svc.Update(ctx, docservice.UpdateRequest{ID: n.ID, Document: docservice.Document{DocType: "AMENDMENT", Title: "Nachtrag Stellplatz SP001",
		Location: "https://dms.example/123", ValidFrom: "2027-01-01"}})
	if err != nil || u.Title != "Nachtrag Stellplatz SP001" {
		t.Fatalf("ändern: %+v %v", u, err)
	}
	if err := e.svc.Detach(ctx, docservice.DetachRequest{ID: n.ID}); err != nil {
		t.Fatal(err)
	}
	if l, _ := e.svc.List(ctx, docservice.ListRequest{EntityType: "Contract", EntityID: "1000|MV-1"}); len(l.Items) != 2 {
		t.Fatalf("nach Entfernen: %d", len(l.Items))
	}
	var removed int
	_ = e.h.db.QueryRow(`SELECT COUNT(*) FROM document__reference WHERE removed_at IS NOT NULL AND removed_by = 'tester'`).Scan(&removed)
	if removed != 1 {
		t.Fatal("entfernter Verweis nicht gespeichert")
	}

	// Umschlüsselung
	ev := events.Event{Object: "Contract", Action: "rekey", Data: map[string]any{"old_id": "1000|MV-1", "new_id": "1000|MV-2026-0001"}}
	if _, err := e.p.Handle(ctx, sdk.Request{Object: docservice.Object, Action: events.CallbackAction, Payload: map[string]any{"event": ev}}); err != nil {
		t.Fatal(err)
	}
	var moved int
	_ = e.h.db.QueryRow(`SELECT COUNT(*) FROM document__reference WHERE entity_id = '1000|MV-2026-0001'`).Scan(&moved)
	if moved != 3 {
		t.Fatalf("umgeschlüsselt: %d", moved)
	}
}
