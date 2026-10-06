package main

import (
	"bytes"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"math"
	"net/url"
	"path/filepath"
	"strconv"

	"github.com/camel/coremesh/pkg/sdk/metamodel"
)

//go:embed templates/*.html
var embeddedTemplates embed.FS

//go:embed static
var embeddedStatic embed.FS

// renderer hält alle Templates. Eingebettet sind die Standard-Templates;
// *.html aus templatesDir werden danach geparst und ersetzen jeden Block
// ({{define "name"}}), den sie neu definieren.
type renderer struct {
	t *template.Template
}

func newRenderer(templatesDir string) (*renderer, error) {
	t, err := template.New("").Funcs(funcs).ParseFS(embeddedTemplates, "templates/*.html")
	if err != nil {
		return nil, err
	}
	if templatesDir != "" {
		files, err := filepath.Glob(filepath.Join(templatesDir, "*.html"))
		if err != nil {
			return nil, err
		}
		if len(files) > 0 {
			if t, err = t.ParseFiles(files...); err != nil {
				return nil, err
			}
		}
	}
	return &renderer{t: t}, nil
}

// fragment rendert den Block name. Es wird erst in einen Puffer geschrieben,
// damit ein Fehler keine halbe Antwort hinterlässt.
func (r *renderer) fragment(w io.Writer, name string, data any) error {
	var buf bytes.Buffer
	if err := r.t.ExecuteTemplate(&buf, name, data); err != nil {
		return err
	}
	_, err := w.Write(buf.Bytes())
	return err
}

// pageData ist der Inhalt des Base-Layouts bei Full-Page-Requests.
type pageData struct {
	AppTitle string
	Title    string
	Active   string        // aktives Object in der Navigation
	Nav      []navItem     // aus Catalog.ListObjects
	Content  template.HTML // bereits gerendertes Fragment
	User     *user         // angemeldeter Benutzer
}

type navItem struct {
	Object string
	Title  string
	Icon   string
}

// page rendert das Fragment und bettet es in das Layout ein.
func (r *renderer) page(w io.Writer, fragment string, data any, page pageData) error {
	var buf bytes.Buffer
	if err := r.t.ExecuteTemplate(&buf, fragment, data); err != nil {
		return err
	}
	// Ausgabe von html/template – bereits korrekt escaped.
	page.Content = template.HTML(buf.String())
	return r.fragment(w, "layout", page)
}

var funcs = template.FuncMap{
	"listable": listable,
	"navCtx": func(n navItem, active string) any {
		return struct {
			navItem
			Active bool
		}{n, n.Object == active}
	},
	"value":      displayValue,
	"pathEscape": url.PathEscape,
	"domID":      func(id string) string { return hex.EncodeToString([]byte(id)) },
	"json": func(v any) string {
		b, _ := json.MarshalIndent(v, "", "  ")
		return string(b)
	},
}

func listable(d metamodel.ObjectDefinition) []metamodel.FieldDefinition {
	var out []metamodel.FieldDefinition
	for _, f := range d.Fields {
		if f.Listable {
			out = append(out, f)
		}
	}
	return out
}

// displayValue formatiert einen Feldwert zur Anzeige.
func displayValue(rec record, f metamodel.FieldDefinition) string {
	v, ok := rec[f.Key]
	if !ok || v == nil || f.Type == metamodel.TypePassword {
		return ""
	}
	switch f.Type {
	case metamodel.TypeBoolean:
		if b, _ := v.(bool); b {
			return "Ja"
		}
		return "Nein"
	case metamodel.TypeSelect:
		s := scalar(v)
		for _, o := range f.Options {
			if o.Value == s {
				return o.Label
			}
		}
		return s
	}
	return scalar(v)
}

// formValue liefert den Wert für ein Formularfeld.
func formValue(rec record, f metamodel.FieldDefinition) string {
	v, ok := rec[f.Key]
	if !ok || v == nil || f.Type == metamodel.TypePassword {
		return ""
	}
	if f.Type == metamodel.TypeBoolean {
		if b, _ := v.(bool); b {
			return "on"
		}
		return ""
	}
	return scalar(v)
}

func scalar(v any) string {
	switch v := v.(type) {
	case string:
		return v
	case float64:
		if v == math.Trunc(v) && math.Abs(v) < 1<<53 {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	}
	return fmt.Sprint(v)
}
