package main

import (
	"net/url"
	"slices"
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
	"github.com/coremesh-labs/coremesh/pkg/sdk/textservice"
)

// TestTextSection: Abschnitt „Texte“ – Liste, fehlende Pflichttexte, Bearbeiten,
// Speichern (auch mit Fehler), Entfernen über das Object Texts.
func TestTextSection(t *testing.T) {
	withDef(t, "Customer", func(d *metamodel.ObjectDefinition) {
		d.Sections = append(slices.Clone(d.Sections), metamodel.SectionDefinition{Key: "texte", Title: "Texte", Texts: true})
	})
	s, h := newMDServer(t)
	detail := do(s, "GET", "/m/crm/Customer/c1", nil, true).Body.String()
	mustContain(t, detail, `class="text-section" hx-get="/texts/Customer/c1"`)

	b := do(s, "GET", "/texts/Customer/c1", nil, true).Body.String()
	mustContain(t, b, "Key with the caretaker", "Englisch", "Beschreibung *", `hx-post="/texts/Customer/c1/t1/remove"`, `name="content"`, "Beschreibung</p>", "… – 2026-12-31", `name="valid_from"`)
	if req := h.find("list").Payload.(textservice.ListRequest); req.EntityType != "Customer" || req.EntityID != "c1" {
		t.Fatalf("ListRequest: %+v", req)
	}
	b = do(s, "GET", "/texts/Customer/c1?edit=t1", nil, true).Body.String()
	mustContain(t, b, `name="id" value="t1"`, ">Key with the caretaker</textarea>", `name="valid_to" value="2026-12-31"`)
	b = do(s, "POST", "/texts/Customer/c1", url.Values{"category": {"NOTE"}, "language": {"de"}, "content": {""}}, true).Body.String()
	mustContain(t, b, "Text ist leer")
	do(s, "POST", "/texts/Customer/c1", url.Values{"category": {"NOTE"}, "language": {"de"}, "position": {"2"}, "content": {"Hund erlaubt"}, "valid_from": {"2027-01-01"}}, true)
	if req := h.find("save").Payload.(textservice.Text); req.Content != "Hund erlaubt" || req.Position != 2 || req.EntityID != "c1" || req.ValidFrom != "2027-01-01" || req.ValidTo != "" {
		t.Fatalf("Save: %+v", req)
	}
	do(s, "POST", "/texts/Customer/c1/t1/remove", nil, true)
	if req := h.find("remove").Payload.(textservice.RemoveRequest); req.ID != "t1" {
		t.Fatalf("RemoveRequest: %+v", req)
	}
}
