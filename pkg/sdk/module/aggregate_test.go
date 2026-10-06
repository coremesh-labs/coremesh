package module

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/camel/coremesh/pkg/sdk"
	"github.com/camel/coremesh/pkg/sdk/metamodel"
)

// memCRM ist ein Modul mit Master Account und Detail Contact (Relation
// "contacts" über account_id), gespeichert im Speicher.
type memCRM struct {
	Base
	mu   sync.Mutex
	rows map[string]map[string]map[string]any // Object → id → Datensatz
	seq  int
}

func newMemCRM() *memCRM {
	return &memCRM{rows: map[string]map[string]map[string]any{"Account": {}, "Contact": {}}}
}

func (m *memCRM) Descriptor() Descriptor { return Descriptor{Name: "crm", Title: "CRM"} }

var crud = []metamodel.ActionConfig{
	{Name: "list", Kind: metamodel.KindList, Label: "Liste"},
	{Name: "get", Kind: metamodel.KindItem, Label: "Anzeigen"},
	{Name: "create", Kind: metamodel.KindCreate, Label: "Neu"},
	{Name: "update", Kind: metamodel.KindUpdate, Label: "Ändern"},
}

// Account endet über ein Status-Flag, Contact über die Zeitscheibe.
func (m *memCRM) RegisterRoutes(r *Router) {
	account := metamodel.ObjectDefinition{Name: "Account", Title: "Konto",
		Actions: append(slices.Clone(crud), metamodel.ActionConfig{Name: "deactivate", Kind: metamodel.KindDeactivate, Label: "Inaktivieren"}),
		Fields: []metamodel.FieldDefinition{
			{Key: "name", Label: "Name", Type: metamodel.TypeText},
			{Key: "active", Label: "Aktiv", Type: metamodel.TypeBoolean},
		},
		Lifecycle: metamodel.Lifecycle{Type: metamodel.LifecycleStatus, StatusField: "active"},
		Sections: []metamodel.SectionDefinition{
			{Key: "base", Title: "Stammdaten", Fields: []string{"name"}},
			{Key: "contacts", Title: "Kontakte", Relation: &metamodel.Relation{Object: "Contact", ForeignKey: "account_id"}},
		}}
	contact := metamodel.ObjectDefinition{Name: "Contact", Title: "Kontakt",
		Actions: append(slices.Clone(crud), metamodel.ActionConfig{Name: "expire", Kind: metamodel.KindExpire, Label: "Beenden"}),
		Fields: []metamodel.FieldDefinition{
			{Key: "account_id", Label: "Konto", Type: metamodel.TypeText},
			{Key: "email", Label: "E-Mail", Type: metamodel.TypeText},
			{Key: "valid_from", Label: "Gültig ab", Type: metamodel.TypeDate},
			{Key: "valid_to", Label: "Gültig bis", Type: metamodel.TypeDate},
		},
		Lifecycle: metamodel.Lifecycle{Type: metamodel.LifecycleTimeSlice, ValidFrom: "valid_from", ValidTo: "valid_to"}}
	for _, d := range []metamodel.ObjectDefinition{account, contact} {
		obj := d.Name
		o := r.Object(obj).Describe(d).
			Handle("list", func(ctx context.Context, req sdk.Request) (sdk.Response, error) { return m.list(obj, req) }).
			Handle("get", func(ctx context.Context, req sdk.Request) (sdk.Response, error) { return m.get(obj, req) }).
			Handle("create", func(ctx context.Context, req sdk.Request) (sdk.Response, error) { return m.write(obj, "", req) }).
			Handle("update", func(ctx context.Context, req sdk.Request) (sdk.Response, error) {
				return m.write(obj, fmt.Sprint(req.Payload.(map[string]any)["id"]), req)
			})
		end := func(field string, value func(map[string]any) (any, error)) HandlerFunc {
			return func(ctx context.Context, req sdk.Request) (sdk.Response, error) {
				p := req.Payload.(map[string]any)
				v, err := value(p)
				if err != nil {
					return sdk.Response{}, err
				}
				m.mu.Lock()
				defer m.mu.Unlock()
				m.rows[obj][fmt.Sprint(p["id"])][field] = v
				return sdk.Response{}, nil
			}
		}
		if obj == "Account" {
			o.Handle("deactivate", end("active", func(map[string]any) (any, error) { return false, nil }))
		} else {
			o.Handle("expire", end("valid_to", func(p map[string]any) (any, error) {
				if p["valid_to"] == "" {
					return nil, fmt.Errorf("%w: valid_to fehlt", sdk.ErrInvalidArgument)
				}
				return p["valid_to"], nil
			}))
		}
	}
}

func (m *memCRM) list(obj string, req sdk.Request) (sdk.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	q, _ := req.Payload.(map[string]any)["query"].(map[string]any)
	items := []any{}
	for _, id := range sortedKeys(m.rows[obj]) {
		rec := m.rows[obj][id]
		if v, ok := q["account_id"]; !ok || rec["account_id"] == v {
			items = append(items, maps.Clone(rec))
		}
	}
	return sdk.Response{Payload: map[string]any{"items": items}}, nil
}

func (m *memCRM) get(obj string, req sdk.Request) (sdk.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.rows[obj][fmt.Sprint(req.Payload.(map[string]any)["id"])]
	if !ok {
		return sdk.Response{}, sdk.ErrNotFound
	}
	return sdk.Response{Payload: maps.Clone(rec)}, nil
}

func (m *memCRM) write(obj, id string, req sdk.Request) (sdk.Response, error) {
	data := req.Payload.(map[string]any)["data"].(map[string]any)
	if email, ok := data["email"]; ok && !strings.Contains(fmt.Sprint(email), "@") {
		return sdk.Response{}, fmt.Errorf("%w: E-Mail %v", sdk.ErrInvalidArgument, email)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if id == "" {
		m.seq++
		id = fmt.Sprintf("%s-%d", strings.ToLower(obj), m.seq)
		m.rows[obj][id] = map[string]any{"id": id}
	}
	rec, ok := m.rows[obj][id]
	if !ok {
		return sdk.Response{}, sdk.ErrNotFound
	}
	maps.Copy(rec, data)
	return sdk.Response{Payload: maps.Clone(rec)}, nil
}

func sortedKeys(m map[string]map[string]any) []string { return slices.Sorted(maps.Keys(m)) }

// txHost leitet Handle wie der Dispatcher an das Plugin zurück und bildet
// Transaktionen über Schnappschüsse des Speichers nach.
type txHost struct {
	recHost
	p       *Plugin
	crm     *memCRM
	snap    map[string]map[string]map[string]any
	commits int
}

func (h *txHost) Handle(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	return h.p.Handle(ctx, req)
}

func (h *txHost) BeginTx(context.Context, string, sql.TxOptions) (string, error) {
	h.crm.mu.Lock()
	defer h.crm.mu.Unlock()
	h.snap = map[string]map[string]map[string]any{}
	for obj, rows := range h.crm.rows {
		h.snap[obj] = map[string]map[string]any{}
		for id, rec := range rows {
			h.snap[obj][id] = maps.Clone(rec)
		}
	}
	return "tx-1", nil
}

func (h *txHost) CommitTx(context.Context, string) error { h.commits++; return nil }

func (h *txHost) RollbackTx(context.Context, string) error {
	h.crm.mu.Lock()
	defer h.crm.mu.Unlock()
	h.crm.rows = h.snap
	return nil
}

func setupCRM(t *testing.T) (*Plugin, *txHost, context.Context) {
	t.Helper()
	crm := newMemCRM()
	p := NewPlugin(Info{Name: "crm", Version: "1"}, crm)
	if err := p.Err(); err != nil {
		t.Fatal(err)
	}
	h := &txHost{p: p, crm: crm}
	ctx := sdk.WithHost(context.Background(), h)
	if err := p.Configure(ctx, sdk.Config{Host: h}); err != nil {
		t.Fatal(err)
	}
	return p, h, ctx
}

func agg(t *testing.T, p *Plugin, ctx context.Context, action string, payload any) (map[string]any, error) {
	t.Helper()
	resp, err := p.Handle(ctx, sdk.Request{Object: "Account", Action: action, Payload: payload})
	m, _ := resp.Payload.(map[string]any)
	return m, err
}

func contacts(out map[string]any) []any {
	return out["relations"].(map[string]any)["contacts"].([]any)
}

func TestAggregateCascadeSave(t *testing.T) {
	p, h, ctx := setupCRM(t)

	m, _ := p.Manifest(ctx)
	if !strings.Contains(fmt.Sprint(m.Capabilities), "getAggregate saveAggregate") {
		t.Fatalf("Aggregat-Actions fehlen im Manifest: %v", m.Capabilities)
	}

	// Neuanlage: Master + zwei Kontakte in einer Transaktion.
	out, err := agg(t, p, ctx, ActionSaveAggregate, map[string]any{
		"data":      map[string]any{"name": "ACME"},
		"relations": map[string]any{"contacts": map[string]any{"create": []any{map[string]any{"email": "a@acme.ch"}, map[string]any{"email": "b@acme.ch"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	id := out["record"].(map[string]any)["id"].(string)
	if cs := contacts(out); len(cs) != 2 || cs[0].(map[string]any)["account_id"] != id || h.commits != 1 {
		t.Fatalf("Neuanlage: %v (commits %d)", out, h.commits)
	}
	first := contacts(out)[0].(map[string]any)["id"]
	second := contacts(out)[1].(map[string]any)["id"]

	// Beenden (Enddatum), Ändern, Anlegen in einem Aufruf – gelöscht wird nichts.
	out, err = agg(t, p, ctx, ActionSaveAggregate, map[string]any{
		"id": id, "data": map[string]any{"name": "ACME AG"},
		"relations": map[string]any{"contacts": map[string]any{
			"expire": []any{map[string]any{"id": first, "valid_to": "2026-12-31"}},
			"update": []any{map[string]any{"id": second, "data": map[string]any{"email": "info@acme.ch"}}},
			"create": []any{map[string]any{"email": "neu@acme.ch"}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var emails []string
	for _, c := range contacts(out) {
		c := c.(map[string]any)
		emails = append(emails, fmt.Sprint(c["email"], "|", c["valid_to"]))
	}
	if out["record"].(map[string]any)["name"] != "ACME AG" || strings.Join(emails, ",") != "a@acme.ch|2026-12-31,info@acme.ch|<nil>,neu@acme.ch|<nil>" {
		t.Fatalf("Batch: %v", out)
	}

	// Fehler in einem Unter-Object: alles zurückgerollt, auch die Master-Änderung.
	_, err = agg(t, p, ctx, ActionSaveAggregate, map[string]any{
		"id": id, "data": map[string]any{"name": "Kaputt"},
		"relations": map[string]any{"contacts": map[string]any{"create": []any{map[string]any{"email": "ohne-at"}}}},
	})
	if !errors.Is(err, sdk.ErrInvalidArgument) || !strings.Contains(err.Error(), "Relation contacts: create[0]") {
		t.Fatalf("Fehler erwartet: %v", err)
	}
	out, _ = agg(t, p, ctx, ActionGetAggregate, map[string]any{"id": id})
	if out["record"].(map[string]any)["name"] != "ACME AG" || len(contacts(out)) != 3 {
		t.Fatalf("Rollback: %v", out)
	}
}

func TestAggregateGuards(t *testing.T) {
	p, _, ctx := setupCRM(t)
	a, _ := agg(t, p, ctx, ActionSaveAggregate, map[string]any{"data": map[string]any{"name": "A"},
		"relations": map[string]any{"contacts": map[string]any{"create": []any{map[string]any{"email": "x@a.ch"}}}}})
	b, _ := agg(t, p, ctx, ActionSaveAggregate, map[string]any{"data": map[string]any{"name": "B"}})
	foreign := contacts(a)[0].(map[string]any)["id"]
	bID := b["record"].(map[string]any)["id"]

	// Kontakte eines anderen Masters lassen sich über das Aggregat nicht ändern.
	_, err := agg(t, p, ctx, ActionSaveAggregate, map[string]any{"id": bID,
		"relations": map[string]any{"contacts": map[string]any{"expire": []any{map[string]any{"id": foreign, "valid_to": "2026-12-31"}}}}})
	if !errors.Is(err, sdk.ErrInvalidArgument) || !strings.Contains(err.Error(), "gehört nicht zu") {
		t.Fatalf("fremdes Unter-Object: %v", err)
	}
	if _, err := agg(t, p, ctx, ActionSaveAggregate, map[string]any{"id": bID, "relations": map[string]any{"gibtsnicht": map[string]any{}}}); !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("unbekannte Relation: %v", err)
	}
	// Physisches Löschen gibt es nicht – auch nicht im Aggregat.
	_, err = agg(t, p, ctx, ActionSaveAggregate, map[string]any{"id": bID, "relations": map[string]any{"contacts": map[string]any{"delete": []any{foreign}}}})
	if !errors.Is(err, sdk.ErrInvalidArgument) || !strings.Contains(err.Error(), `"expire" (Enddatum)`) {
		t.Fatalf("delete im Aggregat: %v", err)
	}
	if _, err := agg(t, p, ctx, ActionSaveAggregate, map[string]any{}); !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("ohne id und data: %v", err)
	}
}

// Eine Relation darf nur auf Objects desselben Moduls zeigen.
func TestAggregateRelationOutsideModule(t *testing.T) {
	var ev []string
	bad := simple("sales", &ev, "Order")
	bad.routes = func(r *Router) {
		d := def("Order")
		d.Sections = []metamodel.SectionDefinition{{Key: "items", Title: "Positionen", Relation: &metamodel.Relation{Object: "Item", ForeignKey: "order_id"}}}
		r.Object("Order").Describe(d).Handle("list", echo)
	}
	err := NewPlugin(Info{Name: "erp", Version: "1"}, bad, simple("stock", &ev, "Item")).Err()
	if err == nil || !strings.Contains(err.Error(), "kein beschriebenes Object dieses Moduls") {
		t.Fatalf("Relation über Modulgrenze: %v", err)
	}
}
