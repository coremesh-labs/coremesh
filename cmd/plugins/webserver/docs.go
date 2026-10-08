package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/docservice"
)

// Abschnitt „Dokumente“ der Detailansicht (SectionDefinition.Documents): die
// Dokumentverweise des Datensatzes über das Object Documents (pkg/sdk/
// docservice) – Plugin document oder ein DMS-Adapter mit derselben
// Schnittstelle.
//
//	GET  /docs/{entity}/{id}                 Liste und Formular (Fragment)
//	POST /docs/{entity}/{id}                 Dokument anhängen
//	POST /docs/{entity}/{id}/{doc}/remove    Verweis entfernen

// docSection sind die Daten des Templates "doc-section".
type docSection struct {
	URL         string
	Items       []docservice.Document
	Types       []docservice.DocType
	CanEdit     bool
	Error       string
	Unavailable string
	Form        docservice.Document
}

func docSectionURL(entityType, entityID string) string {
	return "/docs/" + entityType + "/" + url.PathEscape(entityID)
}

// Link: Ablage als Link, wenn sie wie eine URL aussieht.
func (docSection) Link(location string) string {
	if strings.HasPrefix(location, "http://") || strings.HasPrefix(location, "https://") {
		return location
	}
	return ""
}

func (s *server) docSectionGet(w http.ResponseWriter, r *http.Request) {
	s.renderDocs(w, r, docservice.Document{}, "")
}

func (s *server) renderDocs(w http.ResponseWriter, r *http.Request, form docservice.Document, formErr string) {
	entity, id := r.PathValue("entity"), r.PathValue("id")
	ds := docSection{URL: docSectionURL(entity, id), Form: form, Error: formErr}
	resp, err := s.call(r, docservice.Object, docservice.ActionList, docservice.ListRequest{EntityType: entity, EntityID: id})
	var l docservice.ListResponse
	if err == nil {
		err = sdk.Decode(resp.Payload, &l)
	}
	if err != nil {
		if status := errStatus(err); status >= 500 && status != http.StatusServiceUnavailable {
			s.logError(r, "Dokumente", err)
		}
		ds.Unavailable = s.T(r, "core.docs.unavailable")
		if code := errStatus(err); code == http.StatusForbidden || code == http.StatusNotFound || code == http.StatusUnprocessableEntity {
			parts := strings.SplitN(err.Error(), ": ", 2)
			ds.Unavailable = parts[len(parts)-1]
		}
	} else {
		ds.Items, ds.Types, ds.CanEdit = l.Items, l.Types, l.CanEdit
	}
	s.render(w, r, http.StatusOK, "doc-section", ds, "", "")
}

func (s *server) docSectionAttach(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.fail(w, r, fmt.Errorf("%w: %v", sdk.ErrInvalidArgument, err))
		return
	}
	f := r.PostForm
	d := docservice.Document{DocType: f.Get("doc_type"), Title: f.Get("title"), DocDate: f.Get("doc_date"), Location: f.Get("location"),
		ValidFrom: f.Get("valid_from"), ValidTo: f.Get("valid_to"), Note: f.Get("note")}
	_, err := s.call(r, docservice.Object, docservice.ActionAttach, docservice.AttachRequest{EntityType: r.PathValue("entity"),
		EntityID: r.PathValue("id"), Document: d})
	if err != nil {
		parts := strings.SplitN(err.Error(), ": ", 2)
		s.renderDocs(w, r, d, parts[len(parts)-1])
		return
	}
	s.renderDocs(w, r, docservice.Document{}, "")
}

func (s *server) docSectionRemove(w http.ResponseWriter, r *http.Request) {
	if _, err := s.call(r, docservice.Object, docservice.ActionDetach, docservice.DetachRequest{ID: r.PathValue("doc")}); err != nil {
		parts := strings.SplitN(err.Error(), ": ", 2)
		s.renderDocs(w, r, docservice.Document{}, parts[len(parts)-1])
		return
	}
	s.renderDocs(w, r, docservice.Document{}, "")
}
