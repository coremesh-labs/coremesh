package ai

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"ariga.io/atlas/sql/schema"
	"ariga.io/atlas/sql/sqlite"
	_ "modernc.org/sqlite"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
	"github.com/coremesh-labs/coremesh/pkg/sdk/module"
)

// Testobjekte: Partner mit Relation adressen (Partner-Adresse → Adressdaten über
// address_id) und kommunikation; Kataloge Rolle (klein) und Partner (groß).
var testDefs = map[string]metamodel.ObjectDefinition{
	"TPartner": {Name: "TPartner", Title: "Partner", Fields: []metamodel.FieldDefinition{
		{Key: "id", Label: "Nummer", Type: metamodel.TypeText},
		{Key: "type", Label: "Art", Type: metamodel.TypeSelect, Editable: true, Required: true,
			Options: []metamodel.Option{{Value: "PERSON", Label: "Person"}, {Value: "ORGANIZATION", Label: "Organisation"}}},
		{Key: "name1", Label: "Name 1", Type: metamodel.TypeText, Editable: true, Required: true},
		{Key: "name2", Label: "Name 2", Type: metamodel.TypeText, Editable: true},
		{Key: "parent_id", Label: "Konzern", Type: metamodel.TypeText, Editable: true, Lookup: &metamodel.Lookup{Object: "TPartner", ValueField: "id", LabelFields: []string{"name1"}}},
		{Key: "valid_to", Label: "gültig bis", Type: metamodel.TypeDate, Editable: true},
	}, Sections: []metamodel.SectionDefinition{
		{Key: "adressen", Title: "Adressen", Relation: &metamodel.Relation{Object: "TPartnerAddress", ForeignKey: "bp_id"}},
		{Key: "kommunikation", Title: "Kommunikation", Relation: &metamodel.Relation{Object: "TContact", ForeignKey: "bp_id"}},
	}},
	"TPartnerAddress": {Name: "TPartnerAddress", Title: "Partner-Adresse", Fields: []metamodel.FieldDefinition{
		{Key: "bp_id", Label: "Partner", Type: metamodel.TypeText, Editable: true},
		{Key: "address_id", Label: "Adresse", Type: metamodel.TypeText, Editable: true, Lookup: &metamodel.Lookup{Object: "TAddress", ValueField: "id", LabelFields: []string{"street"}}},
		{Key: "role", Label: "Adressrolle", Type: metamodel.TypeText, Editable: true, Required: true, Lookup: &metamodel.Lookup{Object: "TRole", ValueField: "code", LabelFields: []string{"description"}}},
	}},
	"TAddress": {Name: "TAddress", Title: "Adresse", Fields: []metamodel.FieldDefinition{
		{Key: "street", Label: "Straße", Type: metamodel.TypeText, Editable: true},
		{Key: "zip", Label: "PLZ", Type: metamodel.TypeText, Editable: true},
		{Key: "city", Label: "Ort", Type: metamodel.TypeText, Editable: true},
	}},
	"TContact": {Name: "TContact", Title: "Kommunikation", Fields: []metamodel.FieldDefinition{
		{Key: "bp_id", Label: "Partner", Type: metamodel.TypeText, Editable: true},
		{Key: "value", Label: "Wert", Type: metamodel.TypeText, Editable: true},
		{Key: "is_default", Label: "Standard", Type: metamodel.TypeBoolean, Editable: true},
	}},
}

type testHost struct {
	db      *sql.DB
	mu      sync.Mutex
	numbers int
	deny    map[string]bool
	calls   []sdk.Request
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
func (h *testHost) Read(context.Context, sdk.Request, sdk.RowWriter) (sdk.ReadEnd, error) {
	return sdk.ReadEnd{}, sdk.ErrUnimplemented
}

func (h *testHost) Handle(_ context.Context, req sdk.Request) (sdk.Response, error) {
	var p map[string]any
	_ = sdk.Decode(req.Payload, &p)
	h.mu.Lock()
	defer h.mu.Unlock()
	h.calls = append(h.calls, sdk.Request{Object: req.Object, Action: req.Action, Payload: p})
	list := func(items ...any) (sdk.Response, error) {
		return sdk.Response{Payload: map[string]any{"items": items}}, nil
	}
	switch req.Object + "." + req.Action {
	case "Account.Check":
		return sdk.Response{Payload: map[string]any{"allowed": !h.deny[fmt.Sprint(p["object"])+"."+fmt.Sprint(p["action"])]}}, nil
	case "Account.Granted":
		return sdk.Response{Payload: sdk.GrantSet{CompanyCodeGrant: sdk.CompanyCodeGrant{All: true}, Rules: []sdk.GrantRule{{CompanyCodes: []string{"*"}}}}}, nil
	case "NumberRange.Define":
		return sdk.Response{Payload: map[string]any{}}, nil
	case "NumberRange.Next":
		h.numbers++
		return sdk.Response{Payload: map[string]any{"number": fmt.Sprintf("KI-2026-%06d", h.numbers)}}, nil
	case "Catalog.GetDefinition":
		d, ok := testDefs[fmt.Sprint(p["object"])]
		if !ok {
			return sdk.Response{}, sdk.ErrNotFound
		}
		return sdk.Response{Payload: map[string]any{"definition": d, "available": true}}, nil
	case "TRole.list":
		return list(map[string]any{"code": "MAIN", "description": "Hauptanschrift"}, map[string]any{"code": "INVOICE", "description": "Rechnungsanschrift"})
	case "TPartner.list": // groß: mehr als max_values
		var items []any
		for i := 0; i < 5; i++ {
			items = append(items, map[string]any{"id": fmt.Sprint(i), "name1": "P"})
		}
		return list(items...)
	case "TAddress.create":
		return sdk.Response{Payload: map[string]any{"id": fmt.Sprintf("A%d", len(h.calls))}}, nil
	case "TPartner.saveAggregate":
		return sdk.Response{Payload: map[string]any{"record": map[string]any{"id": "100001"}}}, nil
	}
	return sdk.Response{}, fmt.Errorf("%w: %s.%s", sdk.ErrUnimplemented, req.Object, req.Action)
}

func (h *testHost) find(object, action string) []map[string]any {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []map[string]any
	for _, c := range h.calls {
		if c.Object == object && c.Action == action {
			out = append(out, c.Payload.(map[string]any))
		}
	}
	return out
}

// fakeAI: nachgebildeter Dienst – merkt sich die Anfragen, antwortet mit answer.
type fakeAI struct {
	mu         sync.Mutex
	bodies     []map[string]any
	auth       []string
	answer     string
	status     int
	failFirst  int  // so viele Antworten mit HTTP 503 zuerst
	noPDFImage bool // PDF als image_url ablehnen (HTTP 400, wie OpenAI)
}

func (f *fakeAI) Do(r *http.Request) (*http.Response, error) {
	b, _ := io.ReadAll(r.Body)
	var body map[string]any
	_ = json.Unmarshal(b, &body)
	f.mu.Lock()
	f.bodies = append(f.bodies, body)
	f.auth = append(f.auth, r.Header.Get("Authorization")+"|"+r.Header.Get("X-Extra")+"|"+r.URL.String())
	status, answer := f.status, f.answer
	if f.failFirst > 0 {
		f.failFirst--
		status = 503
	}
	reject := f.noPDFImage && bytes.Contains(b, []byte(`"url":"data:application/pdf`))
	f.mu.Unlock()
	if reject {
		return &http.Response{StatusCode: 400, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"Invalid content part type: image_url"}}`)), Header: http.Header{}}, nil
	}
	if status == 0 {
		status = 200
	}
	var out []byte
	if status == 200 {
		out, _ = json.Marshal(map[string]any{"model": "fake", "choices": []any{map[string]any{"message": map[string]any{"content": answer}}},
			"usage": map[string]any{"total_tokens": 42}})
	} else {
		out = []byte(`{"error":{"message":"Schlüssel ungültig"}}`)
	}
	return &http.Response{StatusCode: status, Body: io.NopCloser(bytes.NewReader(out)), Header: http.Header{}}, nil
}

type env struct {
	t   *testing.T
	m   *Module
	p   *module.Plugin
	h   *testHost
	ai  *fakeAI
	ctx context.Context
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
	h := &testHost{db: db, deny: map[string]bool{}}
	mod := New()
	fa := &fakeAI{}
	mod.http = fa
	mod.getenv = func(k string) string {
		if k == "TEST_AI_KEY" {
			return "env-key"
		}
		return ""
	}
	p := module.NewPlugin(module.Info{Name: Name, Version: "test"}, mod)
	if err := p.Err(); err != nil {
		t.Fatal(err)
	}
	hctx := sdk.WithHost(ctx, h)
	if err := p.Configure(hctx, sdk.Config{Host: h}); err != nil {
		t.Fatal(err)
	}
	resp, err := p.Handle(ctx, sdk.Request{Object: sdk.ObjectDBSchema, Action: sdk.ActionInit, Payload: sdk.SchemaInitRequest{Module: Name}})
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
	return &env{t: t, m: mod, p: p, h: h, ai: fa, ctx: sdk.WithCall(hctx, sdk.CallContext{RequestID: "test", UserID: "tester"})}
}

func (e *env) call(object, action string, payload any) (map[string]any, error) {
	resp, err := e.p.Handle(e.ctx, sdk.Request{Object: object, Action: action, Payload: payload})
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := sdk.Decode(resp.Payload, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (e *env) must(object, action string, payload any) map[string]any {
	e.t.Helper()
	out, err := e.call(object, action, payload)
	if err != nil {
		e.t.Fatalf("%s.%s: %v", object, action, err)
	}
	return out
}

// partnerRequest: Anfrage BP mit Partner, Adressen (eingebettet Adressdaten) und Kommunikation.
func (e *env) partnerRequest() {
	e.t.Helper()
	e.must(providerObject, "create", map[string]any{"data": map[string]any{"code": "TEST", "name": "Testdienst", "base_url": "https://ai.example/v1/",
		"model": "m1", "auth_method": "BEARER", "api_key": "geheim-123", "extra_headers": "X-Extra: ja"}})
	e.must(requestObject, "create", map[string]any{"data": map[string]any{"code": "BP", "name": "Geschäftspartner", "kind": kindObject,
		"task_text": "Ermittle den Absender.", "extra_text": "Firmen sind ORGANIZATION.", "max_values": 3}})
	for i, l := range []map[string]any{
		{"name": "partner", "object": "TPartner"},
		{"name": "addresses", "object": "TPartnerAddress", "parent": "partner", "relation": "adressen", "multiple": true},
		{"name": "address", "object": "TAddress", "parent": "addresses", "via_field": "address_id"},
		{"name": "contacts", "object": "TContact", "parent": "partner", "relation": "kommunikation", "multiple": true, "exclude": "is_default"},
	} {
		l["request"], l["line_no"] = "BP", i+1
		e.must(reqObjObject, "create", map[string]any{"data": l})
	}
}
