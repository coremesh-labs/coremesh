package main

import (
	"net/url"
	"testing"

	"github.com/camel/coremesh/pkg/sdk/metamodel"
	"github.com/camel/coremesh/pkg/sdk/tagservice"
)

func TestPeek(t *testing.T) {
	s, _ := newMDServer(t)

	// Kopfdaten: listable Felder, Link in die Detailansicht des Ziel-Moduls.
	b := do(s, "GET", "/peek/Address/a1", nil, true).Body.String()
	mustContain(t, b, "<small class=\"muted\">Adressen</small><strong>a1</strong>",
		"<dt>Ort</dt><dd>Zürich</dd>",
		`href="/m/crm/Address/a1" hx-get="/m/crm/Address/a1" hx-target="#main-content"`, ">Details öffnen →<")
	mustNotContain(t, b, "<html") // Fragment, kein Layout

	// TitleField als Überschrift, nicht noch einmal in den Kopfdaten.
	b = do(s, "GET", "/peek/Customer/c1", nil, true).Body.String()
	mustContain(t, b, "<strong>Muster AG</strong>", `href="/m/crm/Customer/c1"`)
	mustNotContain(t, b, "<dt>Name</dt>")

	// Ohne Leserecht auf das Ziel: Hinweis im Dialog statt Fehlerseite.
	ro, _ := newMDServerFor(t, "CustomerContact.*")
	w := do(ro, "GET", "/peek/Address/a1", nil, true)
	if w.Code != 200 {
		t.Fatalf("Status %d", w.Code)
	}
	mustContain(t, w.Body.String(), "Keine Berechtigung, diesen Datensatz zu lesen.")
	mustNotContain(t, w.Body.String(), "Zürich", "Details öffnen")

	if w := do(s, "GET", "/peek/kein-object/1", nil, true); w.Code != 422 {
		t.Fatalf("ungültiges Object: %d", w.Code)
	}
}

func TestPeekIconInViews(t *testing.T) {
	s, _ := newMDServer(t)
	// Liste und eingebettete Tabelle: Symbol neben jedem Verweis mit Wert.
	b := do(s, "GET", "/m/crm/Customer/c1/rel/contacts", nil, true).Body.String()
	mustContain(t, b,
		`<span class="peek" hx-get="/peek/ContactKind/MAIL" hx-trigger="mouseenter once delay:150ms, focusin once"`,
		`hx-get="/peek/Address/a1"`, `hx-target="find .peek-card"`, `title="Kopfdaten anzeigen"`)

	// Kein Lookup oder kein Wert: kein Symbol.
	f := metamodel.FieldDefinition{Key: "kind", Lookup: &metamodel.Lookup{Object: "ContactKind"}}
	if peekFor(record{"kind": ""}, f) != "" || peekFor(record{"kind": "X"}, metamodel.FieldDefinition{Key: "kind"}) != "" {
		t.Fatal("Symbol ohne Verweis")
	}
	if u := peekFor(record{"kind": "A/B"}, f); u != "/peek/ContactKind/A%2FB" {
		t.Fatalf("URL: %s", u)
	}
}

func TestHeaderFields(t *testing.T) {
	d := metamodel.ObjectDefinition{TitleField: "name", Fields: []metamodel.FieldDefinition{
		{Key: "id", Listable: true}, {Key: "name", Listable: true}, {Key: "pw", Type: metamodel.TypePassword, Listable: true}, {Key: "city"}},
		Sections: []metamodel.SectionDefinition{{Key: "rel", Relation: &metamodel.Relation{Object: "X"}}, {Key: "base", Fields: []string{"name", "city", "pw"}}}}
	var keys []string
	for _, f := range headerFields(d) {
		keys = append(keys, f.Key)
	}
	if len(keys) != 1 || keys[0] != "city" {
		t.Fatalf("Kopfdaten: %v", keys)
	}
}

// TestLookupByObject: Lookup ohne Metamodell-Feld (Verweis-Tags).
func TestLookupByObject(t *testing.T) {
	s, _ := newMDServer(t)
	b := do(s, "GET", "/lookup?object=ContactKind&field=v.KIND", nil, true).Body.String()
	mustContain(t, b, "<h2>Kontaktarten auswählen</h2>",
		`hx-get="/lookup?field=v.KIND&amp;object=ContactKind&rows=1"`,
		`data-field="v.KIND" data-value="MAIL"`)
	if w := do(s, "GET", "/lookup?object=ContactKind&field=v%22x", nil, true); w.Code != 422 {
		t.Fatalf("ungültiger Feldname: %d", w.Code)
	}
}

func TestTagEditorReference(t *testing.T) {
	s := newTagServer(t, "*.*")
	b := do(s, "GET", "/tags/Partner/p1", nil, true).Body.String()
	mustContain(t, b,
		`name="v.OBJ" value="a1"`,
		`hx-get="/lookup?object=Address&field=v.OBJ"`,
		`<span class="lookup-label" data-lookup-label="v.OBJ">Zürich</span>`,
		`hx-get="/peek/Address/a1"`)

	form := url.Values{"effectiveDate": {"2026-10-06"}, "validFrom": {"2026-10-06"}, "v.RISK": {"LOW"}, "v.OBJ": {" a2 "}}
	do(s, "POST", "/tags/Partner/p1", form, true)
	h := s.host.(tagsHost)
	req := h.find("set").Payload.(tagservice.SetRequest)
	if v := req.Values["OBJ"]; v == nil || v.Ref == nil || *v.Ref != "a2" {
		t.Fatalf("Verweis: %+v", v)
	}
	if sc := h.find("schema").Payload.(tagservice.GetRequest); sc.EntityID != "p1" {
		t.Fatalf("Schema ohne Datensatz (Bedingungen): %+v", sc)
	}
}
