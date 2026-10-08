package module

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// fakeModule ist ein konfigurierbares Test-Modul.
type fakeModule struct {
	desc    Descriptor
	routes  func(r *Router)
	schema  *Schema
	initErr error
	events  *[]string
	env     Env
}

func (f *fakeModule) Descriptor() Descriptor   { return f.desc }
func (f *fakeModule) RegisterRoutes(r *Router) { f.routes(r) }
func (f *fakeModule) Initialize(_ context.Context, env Env) error {
	*f.events = append(*f.events, "init "+f.desc.Name)
	f.env = env
	return f.initErr
}
func (f *fakeModule) Shutdown(context.Context) error {
	*f.events = append(*f.events, "shutdown "+f.desc.Name)
	return nil
}

type withSchema struct{ *fakeModule }

func (w withSchema) Schema() Schema { return *w.schema }

func def(name string) metamodel.ObjectDefinition {
	return metamodel.ObjectDefinition{Name: name, Title: name,
		Fields:  []metamodel.FieldDefinition{{Key: "name", Label: "Name", Type: metamodel.TypeText}},
		Actions: []metamodel.ActionConfig{{Name: "list", Kind: metamodel.KindList, Label: "Übersicht"}}}
}

func echo(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	return sdk.Response{Payload: req.Object + "." + req.Action}, nil
}

func simple(name string, events *[]string, objects ...string) *fakeModule {
	return &fakeModule{desc: Descriptor{Name: name, Title: strings.ToUpper(name)}, events: events, routes: func(r *Router) {
		for _, o := range objects {
			r.Object(o).Describe(def(o)).Handle("list", echo)
		}
	}}
}

// recHost zeichnet Host-Aufrufe auf.
type recHost struct {
	mu   sync.Mutex
	logs []string
	dbs  []string
	reqs []sdk.Request
}

func (h *recHost) Handle(_ context.Context, req sdk.Request) (sdk.Response, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.reqs = append(h.reqs, req)
	return sdk.Response{Payload: "ok"}, nil
}
func (h *recHost) Log(_ context.Context, _ sdk.LogLevel, msg string, f map[string]string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.logs = append(h.logs, msg+" module="+f["module"]+" n="+f["n"])
	return nil
}
func (h *recHost) Query(_ context.Context, db, q string, _ ...any) (*sdk.QueryResult, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.dbs = append(h.dbs, db)
	return &sdk.QueryResult{}, nil
}
func (h *recHost) Exec(context.Context, string, string, ...any) (sdk.ExecResult, error) {
	return sdk.ExecResult{}, nil
}

func TestRoutingManifestDescribe(t *testing.T) {
	var ev []string
	sales := simple("sales", &ev, "Order", "Invoice")
	sales.routes = func(r *Router) {
		r.Object("Order").Describe(def("Order")).Handle("list", echo).Handle("approve", echo)
		r.Object("Invoice").Section("Belege").Describe(def("Invoice")).Handle("list", echo)
		r.Object("Internal").Handle("sync", echo) // ohne Metamodell: nur API
	}
	p := NewPlugin(Info{Name: "erp", Version: "1.0.0"}, sales, simple("stock", &ev, "Item"))
	if err := p.Err(); err != nil {
		t.Fatal(err)
	}

	m, err := p.Manifest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var caps []string
	for _, c := range m.Capabilities {
		caps = append(caps, c.Object+":"+strings.Join(c.Actions, ","))
	}
	if got := strings.Join(caps, " "); got != "Order:list,approve Invoice:list Internal:sync Item:list Catalog:Describe" {
		t.Fatalf("Capabilities: %s", got)
	}

	resp, err := p.Handle(context.Background(), sdk.Request{Object: "Order", Action: "approve"})
	if err != nil || resp.Payload != "Order.approve" {
		t.Fatalf("Routing: %v %v", resp.Payload, err)
	}
	if _, err := p.Handle(context.Background(), sdk.Request{Object: "Order", Action: "delete"}); !errors.Is(err, sdk.ErrUnimplemented) {
		t.Fatalf("unbekannte Action: %v", err)
	}
	if _, err := p.Handle(context.Background(), sdk.Request{Object: sdk.ObjectDBSchema, Action: sdk.ActionInit}); !errors.Is(err, sdk.ErrUnimplemented) {
		t.Fatalf("DBSchema.Init ohne Schema: %v", err)
	}

	resp, _ = p.Handle(context.Background(), sdk.Request{Object: sdk.ObjectCatalog, Action: sdk.ActionDescribe})
	d := resp.Payload.(metamodel.DescribeResponse)
	if len(d.Objects) != 3 || len(d.Modules) != 2 {
		t.Fatalf("Describe: %+v", d)
	}
	if o := d.Modules[0].Objects; len(o) != 2 || o[1].Object != "Invoice" || o[1].Section != "Belege" {
		t.Fatalf("Modul sales: %+v", d.Modules[0])
	}
}

func TestRegistrationErrors(t *testing.T) {
	var ev []string
	cases := map[string][]Module{
		"ungültig":            {simple("Sales", &ev, "Order")},
		"Modul sales doppelt": {simple("sales", &ev, "Order"), simple("sales", &ev, "Item")},
		"gehört bereits zu":   {simple("sales", &ev, "Order"), simple("stock", &ev, "Order")},
		"ist reserviert":      {simple("sales", &ev, "Catalog")},
		"keine Objects":       {simple("sales", &ev)},
		"doppelt registriert": {&fakeModule{desc: Descriptor{Name: "sales", Title: "S"}, events: &ev, routes: func(r *Router) {
			r.Object("Order").Handle("list", echo).Handle("list", echo)
		}}},
		"passt nicht": {&fakeModule{desc: Descriptor{Name: "sales", Title: "S"}, events: &ev, routes: func(r *Router) {
			r.Object("Order").Describe(def("Invoice")).Handle("list", echo)
		}}},
	}
	for want, mods := range cases {
		t.Run(want, func(t *testing.T) {
			p := NewPlugin(Info{Name: "erp", Version: "1"}, mods...)
			if err := p.Err(); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("Fehler mit %q erwartet: %v", want, err)
			}
			if _, err := p.Manifest(context.Background()); err == nil {
				t.Fatal("Manifest muss scheitern")
			}
		})
	}
}

func TestSchemaIsolation(t *testing.T) {
	var ev []string
	table := func(name string) string { return "table \"" + name + "\" {\n  schema = schema.main\n}\n" }
	sales := withSchema{simple("sales", &ev, "Order")}
	sales.schema = &Schema{HCL: table("erp__sales_order"), Seed: []sdk.SchemaSeed{{Table: "erp__sales_order"}}}
	stock := withSchema{simple("stock", &ev, "Item")}
	stock.schema = &Schema{HCL: table("erp__stock_item")}

	p := NewPlugin(Info{Name: "erp", Version: "1"}, sales, stock)
	if err := p.Err(); err != nil {
		t.Fatal(err)
	}
	resp, err := p.Handle(context.Background(), sdk.Request{Object: sdk.ObjectDBSchema, Action: sdk.ActionInit, Payload: sdk.SchemaInitRequest{Module: "erp"}})
	if err != nil {
		t.Fatal(err)
	}
	si := resp.Payload.(sdk.SchemaInitResponse)
	if strings.Count(si.Schema, `schema "main" {}`) != 1 || !strings.Contains(si.Schema, "erp__stock_item") || len(si.Seed) != 1 {
		t.Fatalf("Schema: %s", si.Schema)
	}

	// Mehrere Module mit Schema: Tabellen nur mit Modul-Präfix.
	stock.schema = &Schema{HCL: table("erp__sales_order_lines")}
	if err := NewPlugin(Info{Name: "erp", Version: "1"}, sales, stock).Err(); err == nil || !strings.Contains(err.Error(), `muss mit "erp__stock_" beginnen`) {
		t.Fatalf("fremde Tabelle: %v", err)
	}
	stock.schema = &Schema{HCL: table("erp__stock_item"), Seed: []sdk.SchemaSeed{{Table: "erp__sales_order"}}}
	if err := NewPlugin(Info{Name: "erp", Version: "1"}, sales, stock).Err(); err == nil || !strings.Contains(err.Error(), "fremde Tabelle") {
		t.Fatalf("fremder Seed: %v", err)
	}
	stock.schema = &Schema{HCL: "schema \"main\" {}\n" + table("erp__stock_item")}
	if err := NewPlugin(Info{Name: "erp", Version: "1"}, stock).Err(); err == nil || !strings.Contains(err.Error(), "schema-Block") {
		t.Fatalf("schema-Block: %v", err)
	}
}

func TestLifecycleAndInjection(t *testing.T) {
	var ev []string
	sales, stock := simple("sales", &ev, "Order"), simple("stock", &ev, "Item")
	p := NewPlugin(Info{Name: "erp", Version: "1.0.0"}, sales, stock)
	h := &recHost{}
	settings := map[string]any{
		"database": "erp",
		"modules":  map[string]any{"stock": map[string]any{"database": "warehouse", "limit": 5.0}},
	}
	if err := p.Configure(sdk.WithHost(context.Background(), h), sdk.Config{Settings: settings, Host: h}); err != nil {
		t.Fatal(err)
	}

	// DB je Modul, Konfiguration je Modul.
	if sales.env.DB.Name() != "erp" || stock.env.DB.Name() != "warehouse" || stock.env.Module != "stock" || stock.env.Plugin != "erp" {
		t.Fatalf("Env: %+v / %+v", sales.env, stock.env)
	}
	var cfg struct {
		Limit int `json:"limit"`
	}
	if err := stock.env.Config(&cfg); err != nil || cfg.Limit != 5 {
		t.Fatalf("Config: %+v %v", cfg, err)
	}

	// Ressourcen funktionieren auch ohne Host im ctx (eigene Goroutinen).
	bg := context.Background()
	if _, err := stock.env.DB.Query(bg, "SELECT 1"); err != nil {
		t.Fatal(err)
	}
	if _, err := stock.env.Services.Call(bg, "CompanyCode", "get", nil); err != nil || h.reqs[0].Object != "CompanyCode" {
		t.Fatalf("Services: %v %+v", err, h.reqs)
	}
	stock.env.Log.Info("bereit", "n", 3)
	if h.dbs[0] != "warehouse" || len(h.logs) != 1 || h.logs[0] != "bereit module=stock n=3" {
		t.Fatalf("DB/Log: %v %v", h.dbs, h.logs)
	}

	if err := p.Configure(context.Background(), sdk.Config{Host: h}); !errors.Is(err, sdk.ErrFailedPrecondition) {
		t.Fatalf("zweites Configure: %v", err)
	}
	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(ev, ", "); got != "init sales, init stock, shutdown stock, shutdown sales" {
		t.Fatalf("Reihenfolge: %s", got)
	}
}

func TestInitializeFailureRollsBack(t *testing.T) {
	var ev []string
	sales, stock := simple("sales", &ev, "Order"), simple("stock", &ev, "Item")
	stock.initErr = errors.New("kaputt")
	p := NewPlugin(Info{Name: "erp", Version: "1"}, sales, stock)
	err := p.Configure(context.Background(), sdk.Config{Host: &recHost{}})
	if err == nil || !strings.Contains(err.Error(), "Modul stock: Initialize: kaputt") {
		t.Fatalf("Fehler: %v", err)
	}
	if got := strings.Join(ev, ", "); got != "init sales, init stock, shutdown sales" {
		t.Fatalf("Reihenfolge: %s", got)
	}

	// Unbekannte Modul-Konfiguration ist ein Tippfehler in der Host-Config.
	err = NewPlugin(Info{Name: "erp", Version: "1"}, simple("sales", &ev, "Order")).Configure(context.Background(),
		sdk.Config{Settings: map[string]any{"modules": map[string]any{"sale": map[string]any{}}}, Host: &recHost{}})
	if !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("unbekanntes Modul in settings: %v", err)
	}
}

func (h *recHost) Read(context.Context, sdk.Request, sdk.RowWriter) (sdk.ReadEnd, error) {
	return sdk.ReadEnd{}, sdk.ErrUnimplemented
}

// withMigrate: Modul mit Migrator (scheitert beim ersten Versuch).
type withMigrate struct {
	*fakeModule
	calls int
}

func (w *withMigrate) Migrate(context.Context) error {
	w.calls++
	if w.calls == 1 {
		return errors.New("noch nicht")
	}
	return nil
}

// TestMigrator: Migrate läuft einmal je Prozess vor der ersten Anfrage an das
// Modul; scheitert es, scheitert die Anfrage, die nächste versucht es erneut.
func TestMigrator(t *testing.T) {
	var ev []string
	m := &withMigrate{fakeModule: simple("sales", &ev, "Order")}
	other := simple("stock", &ev, "Item")
	p := NewPlugin(Info{Name: "erp", Version: "1"}, m, other)
	if err := p.Configure(context.Background(), sdk.Config{Host: &recHost{}}); err != nil {
		t.Fatal(err)
	}
	if m.calls != 0 {
		t.Fatal("Migrate vor der ersten Anfrage")
	}
	if _, err := p.Handle(context.Background(), sdk.Request{Object: "Item", Action: "list"}); err != nil || m.calls != 0 {
		t.Fatalf("anderes Modul: %v, calls %d", err, m.calls)
	}
	_, err := p.Handle(context.Background(), sdk.Request{Object: "Order", Action: "list"})
	if err == nil || !strings.Contains(err.Error(), "Modul sales: Migration: noch nicht") {
		t.Fatalf("erster Versuch: %v", err)
	}
	for i := 0; i < 3; i++ {
		if _, err := p.Handle(context.Background(), sdk.Request{Object: "Order", Action: "list"}); err != nil {
			t.Fatal(err)
		}
	}
	if m.calls != 2 {
		t.Fatalf("Migrate %d-mal statt 2", m.calls)
	}
}
