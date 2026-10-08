package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// fakeHost spielt Catalog und ein Fachmodul "Partner". Die Action-Namen sind
// bewusst großgeschrieben: Der WebServer findet sie über Kind, nicht über Namen.
type fakeHost struct {
	mu    sync.Mutex
	calls []sdk.Request
	ctxs  []sdk.CallContext
	fail  map[string]error
}

var partnerDef = metamodel.ObjectDefinition{
	Name: "Partner", Title: "Geschäftspartner", Icon: "icon-users", TitleKey: "crm.Partner.title",
	Fields: []metamodel.FieldDefinition{
		{Key: "company_name", Label: "Firmenname", Type: metamodel.TypeText, Required: true, Listable: true, Editable: true},
		{Key: "email", Label: "E-Mail", Type: metamodel.TypeEmail, Listable: true, Editable: true},
		{Key: "employees", Label: "Mitarbeitende", Type: metamodel.TypeNumber, Editable: true},
		{Key: "active", Label: "Aktiv", Type: metamodel.TypeBoolean, Listable: true, Editable: true},
		{Key: "kind", Label: "Art", Type: metamodel.TypeSelect, Listable: true, Editable: true,
			Options: []metamodel.Option{{Value: "customer", Label: "Kunde"}, {Value: "supplier", Label: "Lieferant"}}},
		{Key: "created_at", Label: "Angelegt", Type: metamodel.TypeDate, Listable: true, Editable: false},
	},
	Actions: []metamodel.ActionConfig{
		{Name: "List", Kind: metamodel.KindList, Label: "Übersicht"},
		{Name: "Item", Kind: metamodel.KindItem, Label: "Anzeigen"},
		{Name: "Create", Kind: metamodel.KindCreate, Label: "Neu"},
		{Name: "Update", Kind: metamodel.KindUpdate, Label: "Bearbeiten"},
		{Name: "Deactivate", Kind: metamodel.KindDeactivate, Label: "Inaktivieren", Confirm: "Partner inaktivieren?"},
		{Name: "Notify", Kind: metamodel.KindCustom, Label: "Benachrichtigen"},
	},
	// Typ B: Status-Flag active – „Inaktivieren“ statt Löschen.
	Lifecycle: metamodel.Lifecycle{Type: metamodel.LifecycleStatus, StatusField: "active"},
}

// crmModule bündelt Partner; Fremd ist ein Object außerhalb des Moduls.
var crmModule = map[string]any{"name": "crm", "title": "CRM", "icon": "icon-crm", "available": true, "title_key": "crm.module.title",
	"objects": []any{map[string]any{"object": "Partner", "title": "Geschäftspartner", "section": "Stammdaten", "available": true}}, "services": []any{"Search"}}

func (h *fakeHost) Handle(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	h.mu.Lock()
	h.calls = append(h.calls, req)
	h.ctxs = append(h.ctxs, sdk.CallFromContext(ctx))
	err := h.fail[req.Object+"."+req.Action]
	h.mu.Unlock()
	if err != nil {
		return sdk.Response{}, err
	}
	acme := map[string]any{"id": "p/1", "company_name": "ACME <AG>", "email": "info@acme.ch",
		"employees": 42.0, "active": true, "kind": "customer", "created_at": "2026-10-05"}
	switch req.Object + "." + req.Action {
	case "Catalog.GetDefinition":
		return sdk.Response{Payload: map[string]any{"definition": partnerDef, "available": true}}, nil
	case "Catalog.Translations":
		p, _ := req.Payload.(map[string]any)
		dict := map[string]any{"crm.module.title": "CRM"}
		if p["locale"] == "zh-CN" {
			dict = map[string]any{"crm.module.title": "客户关系", "crm.Partner.title": "伙伴"}
		}
		return sdk.Response{Payload: map[string]any{"locale": p["locale"], "translations": dict}}, nil
	case "Account.UpdateProfile":
		return sdk.Response{Payload: req.Payload}, nil
	case "Catalog.GetModule":
		if p, _ := req.Payload.(map[string]any); p["module"] != "crm" {
			return sdk.Response{}, fmt.Errorf("%w: Modul %v", sdk.ErrNotFound, p["module"])
		}
		return sdk.Response{Payload: crmModule}, nil
	case "Catalog.ListActions":
		return sdk.Response{Payload: map[string]any{"object": "Partner", "actions": []any{
			map[string]any{"action": "List"}, map[string]any{"action": "Deactivate"}}}}, nil
	case "Catalog.ListModules":
		return sdk.Response{Payload: map[string]any{"modules": []any{crmModule}}}, nil
	case "Catalog.ListObjects":
		return sdk.Response{Payload: map[string]any{"objects": []any{
			map[string]any{"object": "Partner", "title": "Geschäftspartner", "defined": true, "available": true},
			map[string]any{"object": "DBSchema", "defined": false, "available": true},
		}}}, nil
	case "Partner.List":
		return sdk.Response{Payload: map[string]any{"items": []any{acme}}}, nil
	case "Partner.Item":
		return sdk.Response{Payload: acme}, nil
	case "Partner.Create", "Partner.Update":
		in := req.Payload.(map[string]any)
		rec := map[string]any{"id": "p/2"}
		for k, v := range in["data"].(map[string]any) {
			rec[k] = v
		}
		return sdk.Response{Payload: rec}, nil
	case "Partner.Deactivate":
		inactive := map[string]any{}
		for k, v := range acme {
			inactive[k] = v
		}
		inactive["active"] = false
		return sdk.Response{Payload: inactive}, nil
	case "Search.run":
		return sdk.Response{Payload: map[string]any{"hits": 1}}, nil
	case "Partner.Notify":
		return sdk.Response{Payload: map[string]any{"message": "Benachrichtigung verschickt"}}, nil
	}
	return sdk.Response{}, fmt.Errorf("%w: %s.%s", sdk.ErrUnimplemented, req.Object, req.Action)
}

func (h *fakeHost) Log(context.Context, sdk.LogLevel, string, map[string]string) error { return nil }
func (h *fakeHost) Query(context.Context, string, string, ...any) (*sdk.QueryResult, error) {
	return nil, sdk.ErrUnimplemented
}
func (h *fakeHost) Exec(context.Context, string, string, ...any) (sdk.ExecResult, error) {
	return sdk.ExecResult{}, sdk.ErrUnimplemented
}

func (h *fakeHost) find(action string) sdk.Request {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := len(h.calls) - 1; i >= 0; i-- {
		if h.calls[i].Action == action {
			return h.calls[i]
		}
	}
	return sdk.Request{}
}

func (h *fakeHost) last() sdk.Request {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.calls[len(h.calls)-1]
}

func newTestServer(t *testing.T, templatesDir string) (*server, *fakeHost) {
	t.Helper()
	views, err := newRenderer(templatesDir)
	if err != nil {
		t.Fatal(err)
	}
	h := &fakeHost{fail: map[string]error{}}
	ids := newMemIdentity()
	ids.add("tester", testPassword, "", "*.*")
	auth := newAuthService(ids, newMemStore(), time.Hour)
	_, token, _, err := auth.Login(context.Background(), "tester", testPassword, "test")
	if err != nil {
		t.Fatal(err)
	}
	s := newServer(h, views, settings{Title: "CoreMesh", Tenant: "t-42"}, auth)
	testTokens.Store(s, token)
	testIdentities.Store(s, ids)
	return s, h
}

const testPassword = "geheim-passwort-123"

// testTokens: Session-Token je Testserver; do() sendet es als Cookie mit.
var testTokens sync.Map

// testIdentities: memIdentity je Testserver.
var testIdentities sync.Map

// do sendet eine Anfrage als angemeldeter Testbenutzer.
func do(s *server, method, target string, form url.Values, htmx bool) *httptest.ResponseRecorder {
	return doAs(s, method, target, form, htmx, false)
}

// doAnon sendet eine Anfrage ohne Session.
func doAnon(s *server, method, target string, form url.Values, htmx bool) *httptest.ResponseRecorder {
	return doAs(s, method, target, form, htmx, true)
}

func doAs(s *server, method, target string, form url.Values, htmx, anonymous bool) *httptest.ResponseRecorder {
	var body *strings.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	} else {
		body = strings.NewReader("")
	}
	r := httptest.NewRequest(method, target, body)
	if form != nil {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if htmx {
		r.Header.Set("HX-Request", "true")
	}
	if tok, ok := testTokens.Load(s); ok && !anonymous {
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: tok.(string)})
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func mustContain(t *testing.T, body string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(body, p) {
			t.Errorf("fehlt: %q", p)
		}
	}
}

func mustNotContain(t *testing.T, body string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if strings.Contains(body, p) {
			t.Errorf("unerwartet: %q", p)
		}
	}
}

func TestListFullPageVsFragment(t *testing.T) {
	s, h := newTestServer(t, "")

	full := do(s, "GET", "/m/crm/Partner?page=2", nil, false)
	if full.Code != 200 {
		t.Fatalf("Status %d: %s", full.Code, full.Body)
	}
	body := full.Body.String()
	mustContain(t, body,
		"<!doctype html>", "htmx.min.js", `id="main-content"`, `id="toast-container"`,
		`href="/m/crm/Partner" hx-get="/m/crm/Partner"`, // Sidebar aus dem Catalog
		`class="active"`,                           // aktives Object
		"<th>Firmenname</th>", "<th>Angelegt</th>", // listable
		"ACME &lt;AG&gt;", // escaped
		"Kunde", "Ja",     // select-Label, boolean
		`id="row-702f31"`, // hex("p/1")
		`/m/crm/Partner/p%2F1`,
	)
	mustNotContain(t, body, "<th>Mitarbeitende</th>", "DBSchema") // nicht listable; ohne Metamodell

	// List-Action wird über Kind gefunden; URL-Parameter landen in query.
	if req := h.find("List"); req.Action != "List" || req.Payload.(map[string]any)["query"].(map[string]any)["page"] != "2" {
		t.Fatalf("List-Aufruf: %+v", req)
	}
	// Jede HTTP-Anfrage ist eine eigene Wurzelanfrage mit Mandant aus den Settings.
	if c := h.ctxs[len(h.ctxs)-1]; c.TenantID != "t-42" || c.RequestID == "" || c.Metadata["ingress"] != "webserver" {
		t.Fatalf("CallContext: %+v", c)
	}

	frag := do(s, "GET", "/m/crm/Partner", nil, true)
	fb := frag.Body.String()
	mustContain(t, fb, `<section id="list"`, `hx-trigger="coremesh-changed from:body"`)
	mustNotContain(t, fb, "<!doctype html>", "<aside")
	if !slices.Contains(frag.Header().Values("Vary"), "HX-Request") {
		t.Error("Vary: HX-Request fehlt")
	}
}

func TestNewFormGeneratesFields(t *testing.T) {
	s, _ := newTestServer(t, "")
	b := do(s, "GET", "/m/crm/Partner/new", nil, true).Body.String()
	mustContain(t, b,
		`<dialog open class="modal">`,
		`hx-post="/m/crm/Partner" hx-target="#rows" hx-swap="beforeend"`,
		`<input type="text" name="company_name" value=""`, "required",
		`<input type="email" name="email"`,
		`<input type="number" name="employees"`, `step="any"`,
		`<input type="checkbox" name="active"`,
		`<select name="kind"`, `<option value="supplier"`, `>Lieferant</option>`,
	)
	mustNotContain(t, b, `name="created_at"`) // nicht editierbar → nicht im Neu-Formular

	page := do(s, "GET", "/m/crm/Partner/new", nil, false).Body.String()
	mustContain(t, page, "<!doctype html>", `<form method="post" action="/m/crm/Partner"`)
	mustNotContain(t, page, "hx-post=") // ohne JS: normales POST
}

func TestCreate(t *testing.T) {
	s, h := newTestServer(t, "")
	form := url.Values{
		"company_name": {"Neue AG"}, "email": {"neu@ag.ch"}, "employees": {"12,5"},
		"active": {"on"}, "kind": {"supplier"},
		"created_at": {"1999-01-01"}, "evil": {"x"}, // nicht editierbar / unbekannt → ignoriert
	}
	w := do(s, "POST", "/m/crm/Partner", form, true)
	if w.Code != 200 {
		t.Fatalf("Status %d: %s", w.Code, w.Body)
	}
	data := h.last().Payload.(map[string]any)["data"].(map[string]any)
	if data["employees"] != 12.5 || data["active"] != true || data["kind"] != "supplier" {
		t.Fatalf("typisierte Daten: %v", data)
	}
	if _, ok := data["created_at"]; ok {
		t.Fatal("nicht editierbares Feld übernommen")
	}
	if _, ok := data["evil"]; ok {
		t.Fatal("unbekanntes Feld übernommen")
	}
	mustContain(t, w.Body.String(),
		`<tr id="row-702f32"`, "Neue AG", // neue Zeile (hex("p/2"))
		`<div id="modal" hx-swap-oob="innerHTML"></div>`, // Dialog schließen
		`hx-swap-oob="beforeend:#toast-container"`, "angelegt",
	)

	// Ohne HTMX: Post/Redirect/Get.
	if w := do(s, "POST", "/m/crm/Partner", form, false); w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/m/crm/Partner" {
		t.Fatalf("Redirect: %d %s", w.Code, w.Header().Get("Location"))
	}
}

func TestCreateValidation(t *testing.T) {
	s, h := newTestServer(t, "")
	n := len(h.calls)
	w := do(s, "POST", "/m/crm/Partner", url.Values{"email": {"kein-mail"}, "employees": {"viele"}, "kind": {"x"}}, true)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("Status %d", w.Code)
	}
	if w.Header().Get("HX-Retarget") != "#modal" {
		t.Fatal("422 muss auf den Dialog umlenken")
	}
	mustContain(t, w.Body.String(), "Pflichtfeld", "Ungültige E-Mail-Adresse", "Zahl erwartet", "Ungültige Auswahl",
		`value="kein-mail"`) // Eingabe bleibt erhalten
	for _, c := range h.calls[n:] {
		if c.Action == "Create" {
			t.Fatal("Create trotz Validierungsfehler aufgerufen")
		}
	}
}

func TestDetailEditUpdate(t *testing.T) {
	s, h := newTestServer(t, "")

	detail := do(s, "GET", "/m/crm/Partner/p%2F1", nil, true).Body.String()
	mustContain(t, detail, `<section id="detail"`, "<dt>Mitarbeitende</dt><dd>42</dd>", `hx-get="/m/crm/Partner/p%2F1/end?view=detail"`)
	if req := h.last(); req.Action != "Item" || req.Payload.(map[string]any)["id"] != "p/1" {
		t.Fatalf("Item-Aufruf: %+v", req)
	}

	edit := do(s, "GET", "/m/crm/Partner/p%2F1/edit?view=row", nil, true).Body.String()
	mustContain(t, edit,
		`hx-put="/m/crm/Partner/p%2F1" hx-target="#row-702f31" hx-swap="outerHTML"`,
		`name="_view" value="row"`,
		`value="ACME &lt;AG&gt;"`, `<option value="customer" selected>`,
		`name="created_at" value="2026-10-05"`, "readonly", // nicht editierbar → schreibgeschützt
	)

	upd := url.Values{"company_name": {"ACME Holding"}, "kind": {"customer"}, "_view": {"row"}}
	w := do(s, "PUT", "/m/crm/Partner/p%2F1", upd, true)
	mustContain(t, w.Body.String(), "<tr id=", "ACME Holding", "gespeichert")
	if req := h.last(); req.Action != "Update" || req.Payload.(map[string]any)["id"] != "p/1" {
		t.Fatalf("Update-Aufruf: %+v", req)
	}

	upd.Set("_view", "detail")
	mustContain(t, do(s, "PUT", "/m/crm/Partner/p%2F1", upd, true).Body.String(), `<section id="detail"`)

	// Ohne JavaScript: POST + _method=PUT → Redirect auf die Detailansicht.
	upd.Set("_method", "PUT")
	if w := do(s, "POST", "/m/crm/Partner/p%2F1", upd, false); w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/m/crm/Partner/p%2F1" {
		t.Fatalf("Method-Override: %d %s", w.Code, w.Header().Get("Location"))
	}
}

func TestDeleteAndCustomAction(t *testing.T) {
	s, h := newTestServer(t, "")
	// Physisches Löschen gibt es nicht: 405 mit Hinweis auf den Lebenszyklus.
	w := do(s, "DELETE", "/m/crm/Partner/p%2F1", nil, true)
	if w.Code != 405 || !strings.Contains(w.Body.String(), "nicht gelöscht, sondern inaktiviert") {
		t.Fatalf("DELETE: %d %s", w.Code, w.Body.String())
	}

	// Typ B: Bestätigungsdialog, dann deactivate; die Zeile zeigt „Inaktiv“.
	dlg := do(s, "GET", "/m/crm/Partner/p%2F1/end?view=row", nil, true).Body.String()
	mustContain(t, dlg, "Partner inaktivieren?", `hx-post="/m/crm/Partner/p%2F1/end" hx-target="#row-702f31" hx-swap="outerHTML"`)
	mustNotContain(t, dlg, `type="date"`)
	w = do(s, "POST", "/m/crm/Partner/p%2F1/end", url.Values{"_view": {"row"}}, true)
	mustContain(t, w.Body.String(), `<tr id="row-702f31">`, `class="badge inactive"`, "Geschäftspartner inaktiviert")
	if p := h.find("Deactivate").Payload.(map[string]any); p["id"] != "p/1" {
		t.Fatalf("deactivate: %v", p)
	}

	form := do(s, "GET", "/action/crm/Partner/Notify?id=p1", nil, true).Body.String()
	mustContain(t, form, `hx-post="/action/crm/Partner/Notify" hx-target="#modal"`, `name="_id" value="p1"`)

	w = do(s, "POST", "/action/crm/Partner/Notify", url.Values{"_id": {"p1"}, "company_name": {"x"}}, true)
	mustContain(t, w.Body.String(), "Benachrichtigung verschickt")
	if w.Header().Get("HX-Trigger") != "coremesh-changed" {
		t.Fatal("HX-Trigger fehlt")
	}
	if p := h.last().Payload.(map[string]any); p["id"] != "p1" || p["data"] == nil {
		t.Fatalf("custom-Payload: %v", p)
	}
	if w := do(s, "GET", "/action/crm/Partner/Unbekannt", nil, true); w.Code != http.StatusNotFound {
		t.Fatalf("unbekannte Action: %d", w.Code)
	}
}

func TestErrors(t *testing.T) {
	s, h := newTestServer(t, "")
	h.fail["Partner.Item"] = fmt.Errorf("%w: Partner 9", sdk.ErrNotFound)

	w := do(s, "GET", "/m/crm/Partner/9", nil, true)
	if w.Code != http.StatusNotFound || w.Header().Get("HX-Retarget") != "#toast-container" {
		t.Fatalf("HTMX-Fehler: %d %q", w.Code, w.Header().Get("HX-Retarget"))
	}
	mustContain(t, w.Body.String(), `class="toast error"`, "Partner 9")

	page := do(s, "GET", "/m/crm/Partner/9", nil, false)
	mustContain(t, page.Body.String(), "<!doctype html>", "Fehler 404")

	if w := do(s, "GET", "/m/crm/kleingeschrieben", nil, false); w.Code != http.StatusNotFound {
		t.Fatalf("ungültiger Object-Name: %d", w.Code)
	}
	h.fail["Partner.List"] = fmt.Errorf("Datenbank weg")
	if w := do(s, "GET", "/m/crm/Partner", nil, true); w.Code != 500 || strings.Contains(w.Body.String(), "Datenbank weg") {
		t.Fatalf("interne Fehler nicht nach außen geben: %d %s", w.Code, w.Body)
	}
}

func TestTemplateOverride(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "custom.html"), []byte(
		`{{define "brand"}}<a class="brand" href="/">Mein Portal</a>{{end}}`+
			`{{define "row-actions"}}<a href="/eigene/{{.ID}}">Eigene Aktion</a>{{end}}`), 0o644)
	s, _ := newTestServer(t, dir)
	b := do(s, "GET", "/m/crm/Partner", nil, false).Body.String()
	mustContain(t, b, "Mein Portal", "Eigene Aktion")
	mustNotContain(t, b, ">Anzeigen<")
}

func TestStatic(t *testing.T) {
	s, _ := newTestServer(t, "")
	w := do(s, "GET", "/static/app.css", nil, false)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "--accent") {
		t.Fatalf("app.css: %d", w.Code)
	}
}

func (h *fakeHost) Read(_ context.Context, req sdk.Request, _ sdk.RowWriter) (sdk.ReadEnd, error) {
	return sdk.ReadEnd{}, fmt.Errorf("%w: %s.%s", sdk.ErrUnimplemented, req.Object, req.Action)
}
