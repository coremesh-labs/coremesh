package main

import (
	"fmt"
	"html/template"
	"net/http"
	"regexp"
	"strings"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// Textbausteine im Markdown-Editor (FieldDefinition.Snippets, metamodel.SnippetAction):
//
//	GET  /snippets?object=<Bausteine>&field=<Feld>  Leiste mit den Bausteinen (Action list des Objects)
//	POST /snippet  object, code, key, field + Formularwerte  →  Text (text/plain)
//	POST /markdown/preview?field=<Feld>             Vorschau des Felds als HTML
//
// Ein Baustein mit Object öffnet zuerst den Auswahldialog (/lookup?object=…&snippet=…);
// die Wahl fügt den Text an der Schreibmarke ein (coremeshSnippet im Layout).

var snippetRe = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*:[A-Za-z0-9_.\-]+$`)

type snippetButton struct {
	Code, Label, Object string
}

type snippetBar struct {
	Field, Source string
	Buttons       []snippetButton
}

func (s *server) snippets(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	object, field := q.Get("object"), q.Get("field")
	if !objectRe.MatchString(object) || !inputNameRe.MatchString(field) {
		s.fail(w, r, fmt.Errorf("%w: object und field erwartet", sdk.ErrInvalidArgument))
		return
	}
	src, err := s.definition(r, object)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	act, err := need(src, metamodel.KindList)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	resp, err := s.call(r, object, act.Name, map[string]any{"query": map[string]any{}})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	recs, err := records(resp.Payload)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	bar := snippetBar{Field: field, Source: object}
	for _, rec := range recs {
		b := snippetButton{Code: scalar(rec["code"]), Label: scalar(rec["label"]), Object: scalar(rec["object"])}
		if b.Code == "" || !snippetRe.MatchString(object+":"+b.Code) || (b.Object != "" && !objectRe.MatchString(b.Object)) {
			continue
		}
		if b.Label == "" {
			b.Label = b.Code
		}
		bar.Buttons = append(bar.Buttons, b)
	}
	s.render(w, r, http.StatusOK, "snippet-bar", bar, "", "")
}

func (s *server) snippet(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.fail(w, r, fmt.Errorf("%w: %v", sdk.ErrInvalidArgument, err))
		return
	}
	object, code := r.PostForm.Get("_snippet_object"), r.PostForm.Get("_snippet_code")
	if !snippetRe.MatchString(object + ":" + code) {
		s.fail(w, r, fmt.Errorf("%w: Baustein erwartet", sdk.ErrInvalidArgument))
		return
	}
	values := map[string]string{}
	for k, v := range r.PostForm {
		if !strings.HasPrefix(k, "_") && len(v) > 0 {
			values[k] = v[0]
		}
	}
	resp, err := s.call(r, object, metamodel.SnippetAction, map[string]any{"code": code, "key": r.PostForm.Get("_snippet_key"), "values": values})
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprint(w, s.T(r, "core.snippet.failed", err.Error()))
		return
	}
	text := ""
	if m, ok := resp.Payload.(map[string]any); ok {
		text = scalar(m["text"])
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, text)
}

func (s *server) markdownPreview(w http.ResponseWriter, r *http.Request) {
	field := r.URL.Query().Get("field")
	if !inputNameRe.MatchString(field) {
		s.fail(w, r, fmt.Errorf("%w: field erwartet", sdk.ErrInvalidArgument))
		return
	}
	src := r.FormValue(field)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// seit 0.62.0: Vorschau des Objects (FieldDefinition.Preview), z. B. die ganze Mail
	if object := r.URL.Query().Get("object"); object != "" {
		if !objectRe.MatchString(object) {
			s.fail(w, r, fmt.Errorf("%w: object erwartet", sdk.ErrInvalidArgument))
			return
		}
		values := map[string]string{}
		for k, v := range r.PostForm {
			if !strings.HasPrefix(k, "_") && len(v) > 0 {
				values[k] = v[0]
			}
		}
		resp, err := s.call(r, object, metamodel.PreviewAction, map[string]any{"field": field, "values": values})
		if err != nil {
			fmt.Fprintf(w, `<p class="error">%s</p>`, template.HTMLEscapeString(err.Error()))
			return
		}
		doc := ""
		if m, ok := resp.Payload.(map[string]any); ok {
			doc = scalar(m["html"])
		}
		// abgeschottet: keine Skripte, kein Zugriff auf die Anwendung
		fmt.Fprintf(w, `<iframe class="md-doc" sandbox="" referrerpolicy="no-referrer" srcdoc="%s"></iframe>`, template.HTMLEscapeString(doc))
		return
	}
	if strings.TrimSpace(src) == "" {
		fmt.Fprintf(w, `<p class="muted">%s</p>`, template.HTMLEscapeString(s.T(r, "core.markdown.empty")))
		return
	}
	fmt.Fprint(w, `<div class="md">`+markdownHTML(src)+`</div>`)
}
