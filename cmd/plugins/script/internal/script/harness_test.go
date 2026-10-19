package script

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"ariga.io/atlas/sql/schema"
	"ariga.io/atlas/sql/sqlite"
	_ "modernc.org/sqlite"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/module"
)

// testHost: Datenbank (SQLite) und nachgebildete Objects anderer Module –
// TItem (list, update, Datenstrom list), Template (render, get), Account,
// SystemEvent.Register und Hook.Subscribe.
type testHost struct {
	db    *sql.DB
	mu    sync.Mutex
	deny  map[string]bool // Object.action ohne Berechtigung
	calls []sdk.Request
	items []map[string]any
}

func (h *testHost) Log(context.Context, sdk.LogLevel, string, map[string]string) error { return nil }

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
	res, err := h.db.ExecContext(ctx, q, args...)
	if err != nil {
		return sdk.ExecResult{}, err
	}
	n, _ := res.RowsAffected()
	return sdk.ExecResult{RowsAffected: n}, nil
}

func (h *testHost) BeginTx(context.Context, string, sql.TxOptions) (string, error) { return "tx", nil }
func (h *testHost) CommitTx(context.Context, string) error                         { return nil }
func (h *testHost) RollbackTx(context.Context, string) error                       { return nil }

func (h *testHost) Read(_ context.Context, req sdk.Request, w sdk.RowWriter) (sdk.ReadEnd, error) {
	h.record(req)
	if req.Object+"."+req.Action != "TItem.list" {
		return sdk.ReadEnd{}, sdk.ErrUnimplemented
	}
	cols := []string{"id", "group", "amount", "date"}
	if err := w.Header(sdk.ReadHeader{Columns: cols}); err != nil {
		return sdk.ReadEnd{}, err
	}
	for _, it := range h.items {
		if err := w.Rows([][]any{{it["id"], it["group"], it["amount"], it["date"]}}); err != nil {
			return sdk.ReadEnd{}, err
		}
	}
	return sdk.ReadEnd{Rows: int64(len(h.items))}, nil
}

func (h *testHost) record(req sdk.Request) map[string]any {
	var p map[string]any
	_ = sdk.Decode(req.Payload, &p)
	h.mu.Lock()
	defer h.mu.Unlock()
	h.calls = append(h.calls, sdk.Request{Object: req.Object, Action: req.Action, Payload: p})
	return p
}

func (h *testHost) Handle(_ context.Context, req sdk.Request) (sdk.Response, error) {
	p := h.record(req)
	key := req.Object + "." + req.Action
	switch key {
	case "Account.Granted":
		if h.deny[fmt.Sprint(p["object"])+"."+fmt.Sprint(p["action"])] {
			return sdk.Response{Payload: sdk.GrantSet{}}, nil
		}
		return sdk.Response{Payload: sdk.GrantSet{CompanyCodeGrant: sdk.CompanyCodeGrant{All: true}, Rules: []sdk.GrantRule{{CompanyCodes: []string{"*"}}}}}, nil
	case "Account.Check":
		return sdk.Response{Payload: map[string]any{"allowed": true}}, nil
	case "SystemEvent.Register", "Hook.Subscribe":
		return sdk.Response{Payload: map[string]any{}}, nil
	case "TItem.list":
		items := make([]any, len(h.items))
		for i, it := range h.items {
			items[i] = it
		}
		return sdk.Response{Payload: map[string]any{"items": items, "total": len(items)}}, nil
	case "TItem.update":
		return sdk.Response{Payload: map[string]any{"id": p["id"], "ok": true}}, nil
	case "Template.render":
		return sdk.Response{Payload: map[string]any{"job_number": "V-1", "status": "DONE"}}, nil
	case "Template.get":
		return sdk.Response{Payload: map[string]any{"status": "DONE", "file_name": "bericht.pdf", "content_type": "application/pdf", "content": "JVBERi0="}}, nil
	}
	return sdk.Response{}, fmt.Errorf("%w: %s", sdk.ErrUnimplemented, key)
}

func (h *testHost) find(object, action string) []map[string]any {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []map[string]any
	for _, c := range h.calls {
		if c.Object == object && c.Action == action {
			pl, _ := c.Payload.(map[string]any)
			out = append(out, pl)
		}
	}
	return out
}

type env struct {
	t   *testing.T
	m   *Module
	p   *module.Plugin
	h   *testHost
	ctx context.Context // mit Benutzer
	sys context.Context // Systemanfrage (Event)
}

func setup(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "t.db")+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	h := &testHost{db: db, deny: map[string]bool{}, items: []map[string]any{
		{"id": "1", "group": "A", "amount": 1000.0, "date": "2026-01-15"},
		{"id": "2", "group": "B", "amount": 250.5, "date": "2026-02-01"},
		{"id": "3", "group": "A", "amount": 2000.0, "date": "2026-03-31"},
	}}
	mod := New()
	mod.background = false
	p := module.NewPlugin(module.Info{Name: Name, Version: "test"}, mod)
	if err := p.Err(); err != nil {
		t.Fatal(err)
	}
	hctx := sdk.WithHost(ctx, h)
	if err := p.Configure(hctx, sdk.Config{Host: h}); err != nil {
		t.Fatal(err)
	}
	resp, err := p.Handle(hctx, sdk.Request{Object: sdk.ObjectDBSchema, Action: sdk.ActionInit, Payload: sdk.SchemaInitRequest{Module: Name}})
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
		if !strings.HasPrefix(tb.Name, Name+"__") {
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
	return &env{t: t, m: mod, p: p, h: h,
		ctx: sdk.WithCall(hctx, sdk.CallContext{RequestID: "test", UserID: "tester"}),
		sys: sdk.WithCall(hctx, sdk.CallContext{RequestID: "event"})}
}

func (e *env) callIn(ctx context.Context, object, action string, payload any) (map[string]any, error) {
	resp, err := e.p.Handle(ctx, sdk.Request{Object: object, Action: action, Payload: payload})
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := sdk.Decode(resp.Payload, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (e *env) call(object, action string, payload any) (map[string]any, error) {
	return e.callIn(e.ctx, object, action, payload)
}

func (e *env) must(object, action string, payload any) map[string]any {
	e.t.Helper()
	out, err := e.call(object, action, payload)
	if err != nil {
		e.t.Fatalf("%s.%s: %v", object, action, err)
	}
	return out
}

func (e *env) script(code, source string, extra map[string]any) {
	e.t.Helper()
	data := map[string]any{"code": code, "name": "Test " + code, "source": source, "allowed_calls": defaultAllowedCalls}
	for k, v := range extra {
		data[k] = v
	}
	e.must(scriptObject, "create", map[string]any{"data": data})
}

func (e *env) run(code string, data map[string]any) (map[string]any, error) {
	return e.call(scriptObject, "run", map[string]any{"id": code, "data": data})
}
