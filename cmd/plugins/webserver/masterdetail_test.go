package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// mdHost ergänzt fakeHost um ein Master-Detail-Szenario im Modul crm:
// Customer (Abschnitte, Relation "contacts") → CustomerContact (Lookup auf
// ContactKind, Lookup auf Address mit Bearbeiten-Link) und Address.
type mdHost struct {
	*fakeHost
}

var actions = []metamodel.ActionConfig{
	{Name: "list", Kind: metamodel.KindList, Label: "Übersicht"},
	{Name: "get", Kind: metamodel.KindItem, Label: "Anzeigen"},
	{Name: "create", Kind: metamodel.KindCreate, Label: "Neu"},
	{Name: "update", Kind: metamodel.KindUpdate, Label: "Bearbeiten"},
}

var mdDefs = map[string]metamodel.ObjectDefinition{
	"Customer": {Name: "Customer", Title: "Kunde", TitleField: "name", Actions: actions,
		Fields: []metamodel.FieldDefinition{
			{Key: "id", Label: "ID", Type: metamodel.TypeText},
			{Key: "name", Label: "Name", Type: metamodel.TypeText, Listable: true, Editable: true},
			{Key: "note", Label: "Notiz", Type: metamodel.TypeTextarea, Editable: true},
		},
		Sections: []metamodel.SectionDefinition{
			{Key: "base", Title: "Stammdaten", Fields: []string{"name"}},
			{Key: "contacts", Title: "Kontakte", Relation: &metamodel.Relation{Object: "CustomerContact", ForeignKey: "customer_id",
				Columns: []string{"kind_code", "address_id", "value"}}},
			{Key: "archive", Title: "Archiv", Collapsed: true, Fields: []string{"note"}},
		}},
	// Typ A: Zeitscheibe – „Beenden …“ mit Datumsdialog.
	"CustomerContact": {Name: "CustomerContact", Title: "Kontakt",
		Actions:   append(slices.Clone(actions), metamodel.ActionConfig{Name: "expire", Kind: metamodel.KindExpire, Label: "Beenden …"}),
		Lifecycle: metamodel.Lifecycle{Type: metamodel.LifecycleTimeSlice, ValidFrom: "valid_from", ValidTo: "valid_to"},
		Fields: []metamodel.FieldDefinition{
			{Key: "valid_from", Label: "Gültig ab", Type: metamodel.TypeDate},
			{Key: "valid_to", Label: "Gültig bis", Type: metamodel.TypeDate},
			{Key: "customer_id", Label: "Kunde", Type: metamodel.TypeText, Editable: true},
			{Key: "kind_code", Label: "Art", Type: metamodel.TypeText, Listable: true, Editable: true,
				Lookup: &metamodel.Lookup{Object: "ContactKind", ValueField: "code", LabelFields: []string{"description"}}},
			{Key: "address_id", Label: "Adresse", Type: metamodel.TypeText, Listable: true, Editable: true,
				Lookup: &metamodel.Lookup{Object: "Address", ValueField: "id", LabelFields: []string{"city"}}},
			{Key: "value", Label: "Wert", Type: metamodel.TypeText, Listable: true, Editable: true},
		}},
	"ContactKind": {Name: "ContactKind", Title: "Kontaktarten", Actions: actions[:1],
		Fields: []metamodel.FieldDefinition{
			{Key: "code", Label: "Code", Type: metamodel.TypeText, Listable: true},
			{Key: "description", Label: "Beschreibung", Type: metamodel.TypeText, Listable: true},
		}},
	// Typ C: weder Zeitscheibe noch Status-Flag – kein Ende möglich.
	"Address": {Name: "Address", Title: "Adressen", Actions: actions,
		Fields: []metamodel.FieldDefinition{{Key: "city", Label: "Ort", Type: metamodel.TypeText, Listable: true, Editable: true}}},
}

func (h mdHost) Handle(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	p, _ := req.Payload.(map[string]any)
	switch req.Object + "." + req.Action {
	case "Catalog.GetDefinition":
		if d, ok := mdDefs[p["object"].(string)]; ok {
			h.fakeHost.record(ctx, req)
			return sdk.Response{Payload: map[string]any{"definition": d, "available": true, "ui_module": "crm"}}, nil
		}
	case "Catalog.GetModule":
		objs := []any{}
		for _, o := range []string{"Customer", "CustomerContact", "ContactKind", "Address"} {
			objs = append(objs, map[string]any{"object": o, "title": mdDefs[o].Title, "available": true})
		}
		return sdk.Response{Payload: map[string]any{"name": "crm", "title": "CRM", "available": true, "objects": objs}}, nil
	case "Catalog.ListActions":
		return sdk.Response{Payload: map[string]any{"actions": []any{
			map[string]any{"action": "get"}, map[string]any{"action": "getAggregate"}}}}, nil
	case "Customer.get":
		rec := map[string]any{"id": "c1", "name": "Muster AG", "note": "geheim"}
		if lockedCustomer.Load() {
			rec["_locked"] = true
		}
		if m, ok := customerMarkers.Load().(map[string]any); ok {
			for k, v := range m {
				rec[k] = v
			}
			if _, hide := m["_hidden_fields"]; hide {
				delete(rec, "note") // wie crud: ohne Leserecht fehlt das Feld
			}
		}
		return sdk.Response{Payload: rec}, nil
	case "CustomerContact.list":
		h.fakeHost.record(ctx, req)
		return sdk.Response{Payload: map[string]any{"items": []any{map[string]any{
			"id": "k1", "customer_id": "c1", "kind_code": "MAIL", "address_id": "a1", "value": "info@muster.ch",
			"_labels": map[string]any{"kind_code": "E-Mail", "address_id": "Zürich"}}}}}, nil
	case "CustomerContact.formState":
		h.fakeHost.record(ctx, req)
		var in metamodel.FormStateRequest
		_ = sdk.Decode(req.Payload, &in)
		st := metamodel.FormState{Fields: map[string]metamodel.FieldState{}}
		if in.Values["kind_code"] == "TEL" {
			no, yes, v := false, true, "+41 "
			st.Fields["address_id"] = metamodel.FieldState{Visible: &no}
			st.Fields["value"] = metamodel.FieldState{Required: &yes}
			if in.Values["value"] == "" {
				st.Fields["value"] = metamodel.FieldState{Required: &yes, Value: &v}
			}
			st.Message = "Telefon: ohne Adresse"
		}
		return sdk.Response{Payload: st}, nil
	case "Address.list":
		h.fakeHost.record(ctx, req)
		return sdk.Response{Payload: map[string]any{"items": []any{map[string]any{"id": "a1", "city": "Zürich"}}}}, nil
	case "Account.Display":
		if byObject, ok := displayRulesForTest.Load().(map[string]any); ok && byObject[fmt.Sprint(p["object"])] != nil {
			return sdk.Response{Payload: map[string]any{"rules": byObject[fmt.Sprint(p["object"])]}}, nil
		}
		return sdk.Response{Payload: map[string]any{"rules": []any{}}}, nil
	case "Customer.update":
		h.fakeHost.record(ctx, req)
		return sdk.Response{Payload: map[string]any{"id": "c1"}}, nil
	case "Customer.lock":
		h.fakeHost.record(ctx, req)
		return sdk.Response{Payload: map[string]any{"message": "gesperrt"}}, nil
	case "Address.get":
		return sdk.Response{Payload: map[string]any{"id": "a1", "city": "Zürich"}}, nil
	case "CustomerContact.expire":
		h.fakeHost.record(ctx, req)
		if p["valid_to"] == "1999-01-01" {
			return sdk.Response{}, fmt.Errorf("%w: Enddatum liegt vor dem Beginn", sdk.ErrInvalidArgument)
		}
		return sdk.Response{Payload: map[string]any{"id": "k1", "valid_to": p["valid_to"]}}, nil
	case "CustomerContact.get":
		return sdk.Response{Payload: map[string]any{"id": "k1", "customer_id": "c1", "valid_from": "2026-01-01", "kind_code": "MAIL", "value": "info@muster.ch",
			"_labels": map[string]any{"kind_code": "E-Mail"}}}, nil
	case "CustomerContact.create", "CustomerContact.update":
		h.fakeHost.record(ctx, req)
		return sdk.Response{Payload: map[string]any{"id": "k2"}}, nil
	case "ContactKind.list":
		h.fakeHost.record(ctx, req)
		return sdk.Response{Payload: []any{
			map[string]any{"id": "MAIL", "code": "MAIL", "description": "E-Mail"},
			map[string]any{"id": "TEL", "code": "TEL", "description": "Telefon"},
		}}, nil
	}
	return h.fakeHost.Handle(ctx, req)
}

// record vermerkt einen Aufruf, den mdHost selbst beantwortet.
func (h *fakeHost) record(ctx context.Context, req sdk.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.calls = append(h.calls, req)
	h.ctxs = append(h.ctxs, sdk.CallFromContext(ctx))
}

func newMDServer(t *testing.T) (*server, *fakeHost) {
	t.Helper()
	return newMDServerFor(t, "*.*")
}

// newMDServerFor: wie newMDServer, mit den Berechtigungen perms.
func newMDServerFor(t *testing.T, perms ...string) (*server, *fakeHost) {
	t.Helper()
	views, err := newRenderer("")
	if err != nil {
		t.Fatal(err)
	}
	fh := &fakeHost{fail: map[string]error{}}
	ids := newMemIdentity()
	ids.add("tester", testPassword, "", perms...)
	auth := newAuthService(ids, newMemStore(), time.Hour)
	_, token, _, err := auth.Login(context.Background(), "tester", testPassword, "test")
	if err != nil {
		t.Fatal(err)
	}
	s := newServer(mdHost{fh}, views, settings{Title: "CoreMesh"}, auth)
	testTokens.Store(s, token)
	testIdentities.Store(s, ids)
	return s, fh
}

func TestDetailSections(t *testing.T) {
	s, _ := newMDServer(t)
	b := do(s, "GET", "/m/crm/Customer/c1", nil, true).Body.String()
	mustContain(t, b,
		`<span class="subtitle">Muster AG</span>`,               // TitleField
		`<details class="section" id="section-allgemein" open>`, // Felder ohne Abschnitt
		`<details class="section" id="section-base" open>`,      // Stammdaten
		`<details class="section" id="section-archive" >`,       // zugeklappt
		`hx-get="/m/crm/Customer/c1/rel/contacts"`,              // eingebettete Tabelle
		`hx-trigger="load, coremesh-changed from:body"`,         // lädt neu nach dem Speichern
		"<dt>Notiz</dt><dd>geheim</dd>",
	)
}

func TestRelationSection(t *testing.T) {
	s, h := newMDServer(t)
	b := do(s, "GET", "/m/crm/Customer/c1/rel/contacts", nil, true).Body.String()
	if p := h.find("list").Payload.(map[string]any); p["query"].(map[string]any)["customer_id"] != "c1" {
		t.Fatalf("Filter auf den Master: %v", p)
	}
	mustContain(t, b,
		"<th>Art</th><th>Adresse</th><th>Wert</th>",                               // Columns der Relation, ohne Fremdschlüssel
		"<td>E-Mail&#8288;<span class=\"peek\" hx-get=\"/peek/ContactKind/MAIL\"", // Label statt Code, mit Kopfdaten-Vorschau
		`hx-get="/m/crm/Address/a1/edit?view=refresh"`,                            // verknüpfte Adresse bearbeiten
		`hx-get="/m/crm/CustomerContact/new?customer_id=c1&amp;_lock=customer_id&amp;_view=refresh"`,
		`hx-get="/m/crm/CustomerContact/k1/edit?view=refresh&_lock=customer_id"`,
		`hx-get="/m/crm/CustomerContact/k1/end?view=refresh"`, // Typ A: Beenden …
	)
	mustNotContain(t, b, "/m/crm/ContactKind/MAIL/edit") // Kataloge (ValueField code): kein Bearbeiten-Link
	if w := do(s, "GET", "/m/crm/Customer/c1/rel/gibtsnicht", nil, true); w.Code != 404 {
		t.Fatalf("unbekannter Abschnitt: %d", w.Code)
	}
}

func TestRelationForms(t *testing.T) {
	s, h := newMDServer(t)

	// Neu aus dem Abschnitt: Fremdschlüssel fest (hidden), Ziel #modal.
	b := do(s, "GET", "/m/crm/CustomerContact/new?customer_id=c1&_lock=customer_id&_view=refresh", nil, true).Body.String()
	mustContain(t, b,
		`<input type="hidden" name="customer_id" value="c1">`,
		`<input type="hidden" name="_lock" value="customer_id">`,
		`<input type="hidden" name="_view" value="refresh">`,
		`hx-post="/m/crm/CustomerContact" hx-target="#modal" hx-swap="innerHTML"`,
		`hx-get="/lookup?from=CustomerContact&field=kind_code"`, // Lookup-Schaltfläche
	)
	mustNotContain(t, b, `<span>Kunde`)

	form := url.Values{"customer_id": {"c1"}, "kind_code": {"TEL"}, "value": {"044 123 45 67"}, "_view": {"refresh"}, "_lock": {"customer_id"}}
	w := do(s, "POST", "/m/crm/CustomerContact", form, true)
	if w.Code != 200 || w.Header().Get("HX-Trigger") != "coremesh-changed" || !strings.Contains(w.Body.String(), "Kontakt angelegt") {
		t.Fatalf("Anlegen im Abschnitt: %d %v %s", w.Code, w.Header(), w.Body.String())
	}
	if d := h.find("create").Payload.(map[string]any)["data"].(map[string]any); d["customer_id"] != "c1" || d["kind_code"] != "TEL" {
		t.Fatalf("Payload: %v", d)
	}

	// Bearbeiten: Label des Lookup-Werts im Formular.
	b = do(s, "GET", "/m/crm/CustomerContact/k1/edit?view=refresh&_lock=customer_id", nil, true).Body.String()
	mustContain(t, b, `hx-put="/m/crm/CustomerContact/k1" hx-target="#modal"`, `data-lookup-label="kind_code">E-Mail</span>`)
	w = do(s, "PUT", "/m/crm/CustomerContact/k1", form, true)
	if w.Code != 200 || w.Header().Get("HX-Trigger") != "coremesh-changed" {
		t.Fatalf("Ändern im Abschnitt: %d %v", w.Code, w.Header())
	}
}

func TestLookupDialog(t *testing.T) {
	s, h := newMDServer(t)
	b := do(s, "GET", "/lookup?from=CustomerContact&field=kind_code", nil, true).Body.String()
	mustContain(t, b,
		"<h2>Kontaktarten auswählen</h2>",
		`hx-get="/lookup?field=kind_code&amp;from=CustomerContact&rows=1"`,
		`data-field="kind_code" data-value="MAIL" data-label="E-Mail"`,
		`data-value="TEL" data-label="Telefon"`,
	)

	// Suche: nur Zeilen; der WebServer filtert auch, wenn das Ziel q ignoriert.
	b = do(s, "GET", "/lookup?from=CustomerContact&field=kind_code&rows=1&q=tele", nil, true).Body.String()
	mustContain(t, b, `data-value="TEL"`)
	mustNotContain(t, b, `data-value="MAIL"`, "<h2>")
	if q := h.find("list").Payload.(map[string]any)["query"].(map[string]any); q["q"] != "tele" {
		t.Fatalf("Suche an das Ziel: %v", q)
	}

	for path, want := range map[string]int{
		"/lookup?from=CustomerContact&field=value": 404, // kein Lookup-Feld
		"/lookup?from=customer&field=x":            422, // ungültiges Object
	} {
		if w := do(s, "GET", path, nil, true); w.Code != want {
			t.Errorf("%s: %d, erwartet %d", path, w.Code, want)
		}
	}
}

func TestAPIObjectMetadata(t *testing.T) {
	s, _ := newMDServer(t)
	r := httptest.NewRequest("GET", "/api/v1/crm/CustomerContact", nil)
	tok, _ := testTokens.Load(s)
	r.Header.Set("Cookie", sessionCookie+"="+tok.(string))
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	var out struct {
		Lookups []struct {
			Field, Object string
		}
		Aggregate bool
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if len(out.Lookups) != 2 || out.Lookups[0].Field != "kind_code" || out.Lookups[0].Object != "ContactKind" || !out.Aggregate {
		t.Fatalf("Metadaten: %s", w.Body.String())
	}
}

// TestLifecycleUI: Ende-Komponente je Typ – Datumsdialog (A), kein Button und
// 405 (C). Typ B prüft TestDeleteAndCustomAction.
func TestLifecycleUI(t *testing.T) {
	s, h := newMDServer(t)

	// Typ A: Datumsdialog, nie mit Tagesdatum vorbelegt; frühestens gültig ab.
	dlg := do(s, "GET", "/m/crm/CustomerContact/k1/end?view=refresh", nil, true).Body.String()
	mustContain(t, dlg, "Gültigkeit beenden – Kontakt", `<input type="date" name="valid_to" value="" min="2026-01-01" required autofocus>`,
		`hx-post="/m/crm/CustomerContact/k1/end" hx-target="#modal" hx-swap="innerHTML"`)

	w := do(s, "POST", "/m/crm/CustomerContact/k1/end", url.Values{"_view": {"refresh"}}, true)
	if w.Code != 422 || !strings.Contains(w.Body.String(), "Bitte das Enddatum wählen") {
		t.Fatalf("ohne Datum: %d", w.Code)
	}
	w = do(s, "POST", "/m/crm/CustomerContact/k1/end", url.Values{"_view": {"refresh"}, "valid_to": {"1999-01-01"}}, true)
	if w.Code != 422 || !strings.Contains(w.Body.String(), "vor dem Beginn") || w.Header().Get("HX-Retarget") != "#modal" {
		t.Fatalf("Fehler des Moduls: %d %s", w.Code, w.Body.String())
	}
	w = do(s, "POST", "/m/crm/CustomerContact/k1/end", url.Values{"_view": {"refresh"}, "valid_to": {"2026-03-31"}}, true)
	if w.Code != 200 || w.Header().Get("HX-Trigger") != "coremesh-changed" || !strings.Contains(w.Body.String(), "beendet zum 2026-03-31") {
		t.Fatalf("expire: %d %v", w.Code, w.Header())
	}
	if p := h.find("expire").Payload.(map[string]any); p["id"] != "k1" || p["valid_to"] != "2026-03-31" {
		t.Fatalf("Payload: %v", p)
	}

	// Typ C: kein Button, /end und DELETE antworten mit 405.
	detail := do(s, "GET", "/m/crm/Address/a1", nil, true).Body.String()
	mustNotContain(t, detail, "/end", "Löschen", "Inaktivieren", "Beenden")
	for _, tc := range []struct{ method, path string }{
		{"GET", "/m/crm/Address/a1/end"}, {"POST", "/m/crm/Address/a1/end"}, {"DELETE", "/m/crm/Address/a1"},
	} {
		if w := do(s, tc.method, tc.path, url.Values{}, true); w.Code != 405 || !strings.Contains(w.Body.String(), "weder gelöscht noch deaktiviert") {
			t.Errorf("%s %s: %d", tc.method, tc.path, w.Code)
		}
	}
}

func TestLifecycleAPI(t *testing.T) {
	s, _ := newMDServer(t)
	tok, _ := testTokens.Load(s)
	call := func(method, path, body string) (int, map[string]any) {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Cookie", sessionCookie+"="+tok.(string))
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}
	if code, out := call("POST", "/api/v1/crm/Address/delete", `{"id":"a1"}`); code != 405 || !strings.Contains(out["error"].(string), "weder gelöscht noch deaktiviert") {
		t.Fatalf("delete per API: %d %v", code, out)
	}
	for obj, want := range map[string]string{"CustomerContact": "timeslice|expire", "Address": "immutable|"} {
		_, out := call("GET", "/api/v1/crm/"+obj, "")
		lc := out["lifecycle"].(map[string]any)
		if got := lc["type"].(string) + "|" + lc["end_action"].(string); got != want {
			t.Errorf("%s: lifecycle %s, erwartet %s", obj, got, want)
		}
	}
}

// TestTimeSliceRecordID: Links auf den Datensatz nutzen _id (mit Beginndatum),
// eingebettete Abschnitte den fachlichen Schlüssel id.
func TestTimeSliceRecordID(t *testing.T) {
	rec := record{"_id": "c1|2026-01-01", "id": "c1"}
	if recordID(rec) != "c1|2026-01-01" || businessKey(rec) != "c1" || recordID(record{"id": "x"}) != "x" || businessKey(record{"_id": "y"}) != "y" {
		t.Fatal("recordID/businessKey")
	}
	v := view{objectCtx: newObjectCtx("crm", "Customer", mdDefs["Customer"]), Record: rec}
	for _, s := range v.Sections() {
		if s.Relation != nil && s.URL != "/m/crm/Customer/c1/rel/contacts" {
			t.Fatalf("Abschnitt: %s", s.URL)
		}
	}
	if v.ID() != "c1|2026-01-01" {
		t.Fatalf("Datensatz-ID: %s", v.ID())
	}
}

// lockedCustomer: Customer.get liefert "_locked" (TestLockedRecord).
var lockedCustomer atomic.Bool

// TestNestedRelationDetailLink: Hat das Unter-Object selbst eingebettete
// Unter-Objects, führt „Anzeigen“ in der Zeile zu seiner Detailseite.
func TestNestedRelationDetailLink(t *testing.T) {
	s, _ := newMDServer(t)
	mustNotContain(t, do(s, "GET", "/m/crm/Customer/c1/rel/contacts", nil, true).Body.String(), `href="/m/crm/CustomerContact/k1"`)

	old := mdDefs["CustomerContact"]
	t.Cleanup(func() { mdDefs["CustomerContact"] = old })
	d := old
	d.Sections = []metamodel.SectionDefinition{{Key: "addr", Title: "Adressen", Relation: &metamodel.Relation{Object: "Address", ForeignKey: "contact_id"}}}
	mdDefs["CustomerContact"] = d
	s, _ = newMDServer(t)
	mustContain(t, do(s, "GET", "/m/crm/Customer/c1/rel/contacts", nil, true).Body.String(), `<a href="/m/crm/CustomerContact/k1">Anzeigen</a>`)
}

// customerMarkers: Customer.get liefert Feldberechtigungen (TestFieldAccessUI).
var customerMarkers atomic.Value

// TestFieldAccessUI: ausgeblendete Felder fehlen in Detail und Formular,
// schreibgeschützte werden angezeigt, aber nie mitgeschickt.
func TestFieldAccessUI(t *testing.T) {
	t.Cleanup(func() { customerMarkers.Store(map[string]any{}) })
	s, h := newMDServer(t)

	customerMarkers.Store(map[string]any{"_hidden_fields": []any{"note"}})
	mustNotContain(t, do(s, "GET", "/m/crm/Customer/c1", nil, false).Body.String(), "geheim", "<dt>Notiz</dt>")
	edit := do(s, "GET", "/m/crm/Customer/c1/edit", nil, true).Body.String()
	mustNotContain(t, edit, `name="note"`)
	mustContain(t, edit, `name="_access" value="note|"`)

	customerMarkers.Store(map[string]any{"_readonly_fields": []any{"note"}})
	edit = do(s, "GET", "/m/crm/Customer/c1/edit", nil, true).Body.String()
	mustContain(t, edit, "geheim", `name="_access" value="|note"`)
	form := url.Values{"name": {"Muster AG"}, "note": {"manipuliert"}, "_access": {"|note"}}
	if w := do(s, "PUT", "/m/crm/Customer/c1", form, true); w.Code >= 400 {
		t.Fatalf("Speichern: %d %s", w.Code, w.Body.String())
	}
	data := h.find("update").Payload.(map[string]any)["data"].(map[string]any)
	if _, sent := data["note"]; sent || data["name"] != "Muster AG" {
		t.Fatalf("gesendet: %v", data)
	}
}

func TestHiddenEverywhere(t *testing.T) {
	rows := []record{
		{"_hidden_fields": []any{"iban", "bic"}},
		{"_hidden_fields": []any{"iban"}},
	}
	if got := hiddenEverywhere(rows); len(got) != 1 || !got["iban"] {
		t.Fatalf("%v", got)
	}
}

// displayRulesForTest: Account.Display je Object (TestDisplayRulesUI u. a.).
var displayRulesForTest atomic.Value

// TestDisplayRulesUI: Regel „wenn Name = Muster AG: Notiz ausblenden“ wirkt in
// Detail und Formular, das Bedingungsfeld wertet die Maske neu aus.
func TestDisplayRulesUI(t *testing.T) {
	t.Cleanup(func() { displayRulesForTest.Store(map[string]any{}) })
	displayRulesForTest.Store(map[string]any{"Customer": []any{map[string]any{
		"conditions": []any{map[string]any{"field": "name", "values": []any{"Muster AG"}}},
		"hidden":     []any{"note"},
	}}})
	s, h := newMDServer(t)
	mustNotContain(t, do(s, "GET", "/m/crm/Customer/c1", nil, false).Body.String(), "geheim")

	edit := do(s, "GET", "/m/crm/Customer/c1/edit", nil, true).Body.String()
	mustNotContain(t, edit, `name="note"`)
	mustContain(t, edit, `hx-post="/m/crm/Customer/_form"`) // Name ist jetzt Trigger

	// Anderer Name: Bedingung trifft nicht zu, Notiz erscheint.
	form := url.Values{"_mode": {"edit"}, "_rid": {"c1"}, "name": {"Andere AG"}, "note": {""}}
	mustContain(t, do(s, "POST", "/m/crm/Customer/_form", form, true).Body.String(), `name="note"`)

	// Speichern mit zutreffender Bedingung: Notiz geht nicht mit (Wert bleibt).
	if w := do(s, "PUT", "/m/crm/Customer/c1", url.Values{"name": {"Muster AG"}}, true); w.Code >= 400 {
		t.Fatalf("Speichern: %d", w.Code)
	}
	if _, sent := h.find("update").Payload.(map[string]any)["data"].(map[string]any)["note"]; sent {
		t.Fatal("ausgeblendete Notiz mitgeschickt")
	}
}

// TestRelationMatch: Bei zusammengesetzten Schlüsseln hängt das Unter-Object
// über Felder des Masters (Relation.Match) – Filter und Vorbelegung.
func TestRelationMatch(t *testing.T) {
	old := mdDefs["Customer"]
	t.Cleanup(func() { mdDefs["Customer"] = old })
	d := old
	d.Sections = slices.Clone(old.Sections)
	for i, sec := range d.Sections {
		if sec.Relation != nil {
			rel := *sec.Relation
			rel.Match = map[string]string{"customer_id": "name"}
			d.Sections[i].Relation = &rel
		}
	}
	mdDefs["Customer"] = d
	s, h := newMDServer(t)
	b := do(s, "GET", "/m/crm/Customer/c1/rel/contacts", nil, true).Body.String()
	if q := h.find("list").Payload.(map[string]any)["query"].(map[string]any); q["customer_id"] != "Muster AG" {
		t.Fatalf("Filter aus dem Master-Feld: %v", q)
	}
	mustContain(t, b, `hx-get="/m/crm/CustomerContact/new?customer_id=Muster%20AG&amp;_lock=customer_id&amp;_view=refresh"`)
}

// TestDisplayColumnsAndSections: „Spalte ausblenden“ wirkt nur in Listen und
// Unterzeilen (Detail und Formular behalten das Feld), „Abschnitt ausblenden“
// nimmt den Abschnitt aus dem Detail und seine Felder aus dem Formular.
func TestDisplayColumnsAndSections(t *testing.T) {
	t.Cleanup(func() { displayRulesForTest.Store(map[string]any{}) })
	displayRulesForTest.Store(map[string]any{
		"Customer": []any{map[string]any{
			"conditions": []any{map[string]any{"field": "name", "values": []any{"Muster AG"}}},
			"sections":   []any{"archive"},
		}},
		"CustomerContact": []any{map[string]any{"columns": []any{"kind_code"}}},
	})
	s, _ := newMDServer(t)
	rel := do(s, "GET", "/m/crm/CustomerContact/k1", nil, true).Body.String()
	mustContain(t, rel, "E-Mail") // Detail: Spalte ausgeblendet, Feld bleibt
	b := do(s, "GET", "/m/crm/Customer/c1/rel/contacts", nil, true).Body.String()
	mustNotContain(t, b, "<th>Art</th>", "E-Mail")
	mustContain(t, b, "<th>Wert</th>", "info@muster.ch")

	detail := do(s, "GET", "/m/crm/Customer/c1", nil, true).Body.String()
	mustNotContain(t, detail, "Archiv", "geheim")
	mustContain(t, detail, "Kontakte")
	mustNotContain(t, do(s, "GET", "/m/crm/Customer/c1/edit", nil, true).Body.String(), `name="note"`)
}
