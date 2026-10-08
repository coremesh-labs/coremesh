package main

import (
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// withDef ersetzt eine Testdefinition für die Dauer eines Tests.
func withDef(t *testing.T, name string, change func(*metamodel.ObjectDefinition)) {
	orig := mdDefs[name]
	t.Cleanup(func() { mdDefs[name] = orig })
	d := orig
	d.Fields = slices.Clone(orig.Fields)
	change(&d)
	mdDefs[name] = d
}

func field(d *metamodel.ObjectDefinition, key string) *metamodel.FieldDefinition {
	for i := range d.Fields {
		if d.Fields[i].Key == key {
			return &d.Fields[i]
		}
	}
	return nil
}

// TestActionFormDefaults: Aktion mit FormState – Vorbelegung und Hinweis beim Öffnen.
func TestActionFormDefaults(t *testing.T) {
	withDef(t, "CustomerContact", func(d *metamodel.ObjectDefinition) {
		d.FormState = "formState"
		d.Actions = append(slices.Clone(d.Actions),
			metamodel.ActionConfig{Name: "remind", Kind: metamodel.KindCustom, Label: "Erinnern …", Fields: []string{"value"}, FormState: true},
			metamodel.ActionConfig{Name: "plain", Kind: metamodel.KindCustom, Label: "Ohne …", Fields: []string{"value"}})
	})
	s, h := newMDServer(t)
	b := do(s, "GET", "/action/crm/CustomerContact/remind?id=k1", nil, true).Body.String()
	mustContain(t, b, `name="value" value="2026-10-08"`, `Letzte Erinnerung: 01.10.2026`)
	if req := h.find("formState").Payload.(metamodel.FormStateRequest); req.Mode != "action" || req.Action != "remind" || req.ID != "k1" {
		t.Fatalf("FormStateRequest: %+v", req)
	}
	n := len(h.calls)
	b = do(s, "GET", "/action/crm/CustomerContact/plain", nil, true).Body.String()
	mustNotContain(t, b, `2026-10-08`)
	for _, c := range h.calls[n:] {
		if c.Action == "formState" {
			t.Fatal("Aktion ohne FormState fragt die Maske ab")
		}
	}
}

func TestFormStateMask(t *testing.T) {
	withDef(t, "CustomerContact", func(d *metamodel.ObjectDefinition) {
		d.FormState = "formState"
		field(d, "kind_code").Trigger = true
		field(d, "address_id").Group = "Zuordnung"
		field(d, "value").Group = "Kontakt"
	})
	s, h := newMDServer(t)

	// Neues Formular: Gruppen, Trigger am Auslöserfeld, Formularwerte für den Lookup-Dialog.
	b := do(s, "GET", "/m/crm/CustomerContact/new?customer_id=c1&_lock=customer_id&_view=refresh", nil, true).Body.String()
	mustContain(t, b, `<legend>Zuordnung</legend>`, `<legend>Kontakt</legend>`, `name="_mode" value="create"`,
		`name="kind_code" value="" autocomplete="off" hx-post="/m/crm/CustomerContact/_form" hx-trigger="change"`,
		`hx-include="closest form"`, `name="address_id"`)
	if req := h.find("formState").Payload.(metamodel.FormStateRequest); req.Mode != "create" || req.Values["customer_id"] != "c1" ||
		!slices.Contains(req.Locked, "customer_id") {
		t.Fatalf("FormStateRequest: %+v", req)
	}

	// Neuauswertung: Telefon blendet die Adresse aus, Wert wird Pflicht und vorbelegt.
	form := url.Values{"_mode": {"create"}, "_lock": {"customer_id"}, "_view": {"refresh"}, "customer_id": {"c1"}, "kind_code": {"TEL"}}
	b = do(s, "POST", "/m/crm/CustomerContact/_form", form, true).Body.String()
	mustContain(t, b, `class="form-body"`, `Telefon: ohne Adresse`, `name="value" value="&#43;41 "`)
	mustNotContain(t, b, `name="address_id"`, "<dialog")
	if !strings.Contains(b, `Wert <abbr`) {
		t.Fatalf("Pflicht fehlt: %s", b)
	}

	// Speichern mit derselben Maske: Adresse ausgeblendet → geleert, Wert Pflicht.
	form.Set("address_id", "a1")
	form.Set("value", "")
	form.Set("kind_code", "TEL")
	do(s, "POST", "/m/crm/CustomerContact", form, true) // Vorbelegung des Plugins füllt den Wert
	p := h.find("create").Payload.(map[string]any)["data"].(map[string]any)
	if p["address_id"] != nil || p["value"] != "+41" {
		t.Fatalf("gespeichert: %v", p)
	}
}

func TestDeclarativeRules(t *testing.T) {
	withDef(t, "CustomerContact", func(d *metamodel.ObjectDefinition) {
		field(d, "kind_code").Trigger = true
		field(d, "address_id").ShowIf = &metamodel.Condition{Field: "kind_code", Values: []string{"MAIL", "POST"}}
		field(d, "value").RequiredIf = &metamodel.Condition{Field: "kind_code", Values: []string{"MAIL"}}
	})
	s, _ := newMDServer(t)
	b := do(s, "GET", "/m/crm/CustomerContact/new?kind_code=TEL", nil, true).Body.String()
	mustNotContain(t, b, `name="address_id"`)
	b = do(s, "POST", "/m/crm/CustomerContact/_form", url.Values{"kind_code": {"MAIL"}}, true).Body.String()
	mustContain(t, b, `name="address_id"`, `Wert <abbr`)
	// Ausgeblendetes Pflichtfeld schlägt beim Speichern nicht an.
	withDef(t, "CustomerContact", func(d *metamodel.ObjectDefinition) {
		f := field(d, "address_id")
		f.Required, f.ShowIf = true, &metamodel.Condition{Field: "kind_code", Values: []string{"MAIL"}}
	})
	if w := do(s, "POST", "/m/crm/CustomerContact", url.Values{"kind_code": {"TEL"}, "value": {"1"}}, true); w.Code == 422 {
		t.Fatalf("ausgeblendetes Pflichtfeld: %s", w.Body.String())
	}
}

func TestLookupFilterFromForm(t *testing.T) {
	withDef(t, "CustomerContact", func(d *metamodel.ObjectDefinition) {
		field(d, "customer_id").Lookup = &metamodel.Lookup{Object: "Customer", ValueField: "id", LabelFields: []string{"name"}}
		f := field(d, "address_id")
		l := *f.Lookup
		l.Filters = map[string]string{"city": "customer_id.name", "kind": "kind_code"}
		f.Lookup = &l
	})
	s, h := newMDServer(t)
	b := do(s, "GET", "/lookup?from=CustomerContact&field=address_id&customer_id=c1&kind_code=MAIL", nil, true).Body.String()
	q := h.find("list").Payload.(map[string]any)["query"].(map[string]any)
	if q["city"] != "Muster AG" || q["kind"] != "MAIL" {
		t.Fatalf("Filter: %v", q)
	}
	// Die Suche im Dialog behält die aufgelösten Filter.
	mustContain(t, b, "filter.city=Muster&#43;AG")
}

func TestListFilters(t *testing.T) {
	withDef(t, "CustomerContact", func(d *metamodel.ObjectDefinition) {
		d.Filters, d.Search = []string{"kind_code", "value"}, true
		field(d, "kind_code").Options = []metamodel.Option{{Value: "MAIL", Label: "E-Mail"}, {Value: "TEL", Label: "Telefon"}}
	})
	s, h := newMDServer(t)
	b := do(s, "GET", "/m/crm/CustomerContact?kind_code=MAIL&q=muster", nil, true).Body.String()
	mustContain(t, b, `class="list-filters"`, `name="q" value="muster"`, `<option value="MAIL" selected>E-Mail</option>`,
		`hx-get="/m/crm/CustomerContact?kind_code=MAIL&amp;q=muster"`, ">Filter zurücksetzen<")
	q := h.find("list").Payload.(map[string]any)["query"].(map[string]any)
	if q["kind_code"] != "MAIL" || q["q"] != "muster" {
		t.Fatalf("Query: %v", q)
	}
}

func TestHiddenRecordActions(t *testing.T) {
	oc := objectCtx{Custom: []metamodel.ActionConfig{{Name: "lock", Record: true}, {Name: "unlock", Record: true}, {Name: "load"}}}
	got := recordActions(oc, record{"_hidden_actions": []any{"unlock"}})
	if len(got) != 1 || got[0].Name != "lock" {
		t.Fatalf("Aktionen: %v", got)
	}
}

// TestLookupFilters: Verweisfelder in der Filterleiste zeigen Texte – kleine
// Ziele als Auswahl, große als Eingabe mit Auswahldialog und Text zum Wert.
func TestLookupFilters(t *testing.T) {
	withDef(t, "CustomerContact", func(d *metamodel.ObjectDefinition) {
		d.Filters = []string{"kind_code", "address_id"}
	})
	s, _ := newMDServer(t)
	b := do(s, "GET", "/m/crm/CustomerContact?kind_code=TEL", nil, true).Body.String()
	mustContain(t, b, `<option value="MAIL" >E-Mail</option>`, `<option value="TEL" selected>Telefon</option>`,
		`<option value="a1" >Zürich</option>`)

	old := filterChoiceLimit
	filterChoiceLimit = 1
	t.Cleanup(func() { filterChoiceLimit = old })
	b = do(s, "GET", "/m/crm/CustomerContact?kind_code=TEL", nil, true).Body.String()
	mustContain(t, b, `name="kind_code" value="TEL"`, `hx-get="/lookup?from=CustomerContact&field=kind_code"`,
		`<span class="lookup-label" data-lookup-label="kind_code">Telefon</span>`,
		`<option value="a1" >Zürich</option>`) // Adresse: ein Eintrag, weiter Auswahl
}

// TestLookupFilterCompositeKey: Liefert das Lookup-Ziel den Datensatz nicht
// über get (zusammengesetzter Schlüssel wie Buchungskreis|Code), sucht der
// WebServer ihn in dessen Liste.
func TestLookupFilterCompositeKey(t *testing.T) {
	withDef(t, "CustomerContact", func(d *metamodel.ObjectDefinition) {
		f := field(d, "address_id")
		l := *f.Lookup
		l.Filters = map[string]string{"city": "kind_code.description"}
		f.Lookup = &l
	})
	s, h := newMDServer(t)
	do(s, "GET", "/lookup?from=CustomerContact&field=address_id&kind_code=TEL", nil, true)
	var q map[string]any
	for _, c := range h.calls {
		if c.Object == "Address" && c.Action == "list" {
			q = c.Payload.(map[string]any)["query"].(map[string]any)
		}
	}
	if q["city"] != "Telefon" {
		t.Fatalf("Filter aus der Liste des Ziels: %v", q)
	}
}
