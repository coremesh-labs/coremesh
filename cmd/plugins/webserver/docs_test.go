package main

import (
	"net/url"
	"slices"
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk/docservice"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// TestDocumentSection: Abschnitt „Dokumente“ – Liste, Link, Anhängen (auch
// mit Fehler), Entfernen über das Object Documents.
func TestDocumentSection(t *testing.T) {
	withDef(t, "Customer", func(d *metamodel.ObjectDefinition) {
		d.Sections = append(slices.Clone(d.Sections), metamodel.SectionDefinition{Key: "dokumente", Title: "Dokumente", Documents: true})
	})
	s, h := newMDServer(t)
	detail := do(s, "GET", "/m/crm/Customer/c1", nil, true).Body.String()
	mustContain(t, detail, `class="doc-section" hx-get="/docs/Customer/c1"`)

	b := do(s, "GET", "/docs/Customer/c1", nil, true).Body.String()
	mustContain(t, b, "AGB 2026", `<a href="https://dms.example/1"`, `hx-post="/docs/Customer/c1/d1/remove"`, `name="location"`)
	if req := h.find("list").Payload.(docservice.ListRequest); req.EntityType != "Customer" || req.EntityID != "c1" {
		t.Fatalf("ListRequest: %+v", req)
	}
	b = do(s, "POST", "/docs/Customer/c1", url.Values{"doc_type": {"AGB"}, "title": {""}, "location": {"x"}}, true).Body.String()
	mustContain(t, b, "Titel und Ablage sind Pflicht", `value="x"`)
	do(s, "POST", "/docs/Customer/c1", url.Values{"doc_type": {"AGB"}, "title": {"Vertrag"}, "location": {"v.pdf"}, "valid_from": {"2026-01-01"}}, true)
	if req := h.find("attach").Payload.(docservice.AttachRequest); req.Document.Title != "Vertrag" || req.Document.ValidFrom != "2026-01-01" {
		t.Fatalf("AttachRequest: %+v", req)
	}
	do(s, "POST", "/docs/Customer/c1/d1/remove", nil, true)
	if req := h.find("detach").Payload.(docservice.DetachRequest); req.ID != "d1" {
		t.Fatalf("DetachRequest: %+v", req)
	}
}
