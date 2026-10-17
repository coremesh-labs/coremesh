package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/textservice"
)

// Abschnitt „Texte“ der Detailansicht (SectionDefinition.Texts): die Texte des
// Datensatzes über das Object Texts (pkg/sdk/textservice – Plugin tag, Modul
// textmanagement): Kategorie, Position, Sprache, Inhalt; fehlende Pflichttexte.
//
//	GET  /texts/{entity}/{id}[?edit=<text>]   Liste und Formular (Fragment)
//	POST /texts/{entity}/{id}                 Text anlegen bzw. ändern (id im Formular)
//	POST /texts/{entity}/{id}/{text}/remove   Text entfernen

// textSection sind die Daten des Templates "text-section".
type textSection struct {
	URL         string
	Items       []textservice.Text
	Categories  []textservice.Category
	Languages   []textservice.Language
	Missing     []textservice.Category
	CanEdit     bool
	Error       string
	Unavailable string
	Form        textservice.Text
}

func textSectionURL(entityType, entityID string) string {
	return "/texts/" + entityType + "/" + url.PathEscape(entityID)
}

// LanguageName: Bezeichnung des Sprachschlüssels.
func (s textSection) LanguageName(code string) string {
	for _, l := range s.Languages {
		if l.Code == code {
			return l.Name
		}
	}
	return code
}

func (s *server) textSectionGet(w http.ResponseWriter, r *http.Request) {
	form := textservice.Text{}
	if id := r.URL.Query().Get("edit"); id != "" {
		form.ID = id
	}
	s.renderTexts(w, r, form, "")
}

func (s *server) renderTexts(w http.ResponseWriter, r *http.Request, form textservice.Text, formErr string) {
	entity, id := r.PathValue("entity"), r.PathValue("id")
	ts := textSection{URL: textSectionURL(entity, id), Form: form, Error: formErr}
	resp, err := s.call(r, textservice.Object, textservice.ActionList, textservice.ListRequest{EntityType: entity, EntityID: id})
	var l textservice.ListResponse
	if err == nil {
		err = sdk.Decode(resp.Payload, &l)
	}
	if err != nil {
		if status := errStatus(err); status >= 500 && status != http.StatusServiceUnavailable {
			s.logError(r, "Texte", err)
		}
		ts.Unavailable = s.T(r, "core.texts.unavailable")
		if code := errStatus(err); code == http.StatusForbidden || code == http.StatusNotFound || code == http.StatusUnprocessableEntity {
			parts := strings.SplitN(err.Error(), ": ", 2)
			ts.Unavailable = parts[len(parts)-1]
		}
	} else {
		ts.Items, ts.Categories, ts.Languages, ts.Missing, ts.CanEdit = l.Items, l.Categories, l.Languages, l.Missing, l.CanEdit
		// Bearbeiten: Formular mit dem gewählten Text vorbelegen
		if form.ID != "" && form.Content == "" {
			for _, it := range l.Items {
				if it.ID == form.ID {
					ts.Form = it
				}
			}
		}
		if ts.Form.Language == "" {
			ts.Form.Language = localeFrom(r)
		}
	}
	s.render(w, r, http.StatusOK, "text-section", ts, "", "")
}

func (s *server) textSectionSave(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.fail(w, r, fmt.Errorf("%w: %v", sdk.ErrInvalidArgument, err))
		return
	}
	f := r.PostForm
	pos, _ := strconv.Atoi(strings.TrimSpace(f.Get("position")))
	t := textservice.Text{ID: f.Get("id"), EntityType: r.PathValue("entity"), EntityID: r.PathValue("id"), Category: f.Get("category"),
		Position: pos, Language: f.Get("language"), Content: f.Get("content")}
	if _, err := s.call(r, textservice.Object, textservice.ActionSave, t); err != nil {
		parts := strings.SplitN(err.Error(), ": ", 2)
		s.renderTexts(w, r, t, parts[len(parts)-1])
		return
	}
	s.renderTexts(w, r, textservice.Text{}, "")
}

func (s *server) textSectionRemove(w http.ResponseWriter, r *http.Request) {
	if _, err := s.call(r, textservice.Object, textservice.ActionRemove, textservice.RemoveRequest{ID: r.PathValue("text")}); err != nil {
		parts := strings.SplitN(err.Error(), ": ", 2)
		s.renderTexts(w, r, textservice.Text{}, parts[len(parts)-1])
		return
	}
	s.renderTexts(w, r, textservice.Text{}, "")
}
