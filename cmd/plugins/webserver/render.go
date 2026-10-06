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
	"github.com/camel/coremesh/pkg/sdk/module"
)

//go:embed templates/*.html
var embeddedTemplates embed.FS

//go:embed static
var embeddedStatic embed.FS

// renderer hält alle Templates. Eingebettet sind die Standard-Templates;
// *.html aus templatesDir werden danach geparst und ersetzen jeden Block
// ({{define "name"}}), den sie neu definieren.
//
// Je Sprache gibt es eine Kopie der Templates, in der die Funktionen t (Text
// übersetzen), locale (Sprache) und value (Ja/Nein) an diese Sprache gebunden
// sind. Überschriebene Blöcke nutzen dieselben Funktionen.
type renderer struct {
	t        *template.Template
	byLocale map[string]*template.Template
}

func newRenderer(templatesDir string) (*renderer, error) {
	core, err := module.LoadTranslations(coreFiles, "i18n")
	if err != nil {
		return nil, err
	}
	t, err := template.New("").Funcs(funcs).Funcs(localeFuncs(core, metamodel.LocaleDE)).ParseFS(embeddedTemplates, "templates/*.html")
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
	r := &renderer{t: t, byLocale: map[string]*template.Template{}}
	for _, loc := range metamodel.Locales {
		c, err := t.Clone()
		if err != nil {
			return nil, err
		}
		r.byLocale[loc] = c.Funcs(localeFuncs(core, loc))
	}
	return r, nil
}

// localeFuncs sind die an eine Sprache gebundenen Template-Funktionen.
func localeFuncs(core metamodel.Translations, loc string) template.FuncMap {
	t := func(key string, args ...any) string {
		s := core.Lookup(loc, key, key)
		if len(args) > 0 {
			return fmt.Sprintf(s, args...)
		}
		return s
	}
	return template.FuncMap{
		"t":      t,
		"locale": func() string { return loc },
		"value": func(rec record, f metamodel.FieldDefinition) string {
			return displayValueIn(rec, f, t("core.app.yes"), t("core.app.no"))
		},
	}
}

func (r *renderer) tmpl(loc string) *template.Template {
	if t, ok := r.byLocale[loc]; ok {
		return t
	}
	return r.t
}

// fragment rendert den Block name. Es wird erst in einen Puffer geschrieben,
// damit ein Fehler keine halbe Antwort hinterlässt.
func (r *renderer) fragment(loc string, w io.Writer, name string, data any) error {
	var buf bytes.Buffer
	if err := r.tmpl(loc).ExecuteTemplate(&buf, name, data); err != nil {
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
	Nav      []navModule   // Module aus Catalog.ListModules; das aktive mit seinen Objects
	Content  template.HTML // bereits gerendertes Fragment
	User     *user         // angemeldeter Benutzer
	Path     string        // aktueller Pfad (Rücksprung des Sprachwählers)
}

// navItem ist ein Object in der Navigation des aktiven Moduls.
type navItem struct {
	Object string
	Title  string
	Icon   string
	URL    string // /m/{module}/{object}
	Active bool
}

// page rendert das Fragment und bettet es in das Layout ein.
func (r *renderer) page(loc string, w io.Writer, fragment string, data any, page pageData) error {
	var buf bytes.Buffer
	if err := r.tmpl(loc).ExecuteTemplate(&buf, fragment, data); err != nil {
		return err
	}
	// Ausgabe von html/template – bereits korrekt escaped.
	page.Content = template.HTML(buf.String())
	return r.fragment(loc, w, "layout", page)
}

var funcs = template.FuncMap{
	"listable":    listable,
	"raw":         func(rec record, key string) string { return scalar(rec[key]) },
	"locales":     func() []string { return metamodel.Locales },
	"tagFieldCtx": func(ed *tagEditor, f tagField) tagFieldCtx { return tagFieldCtx{Ed: ed, F: f} },
	"historyCtx": func(url string, on bool, target, swap string) historyCtx {
		return historyCtx{URL: url, On: on, Target: target, Swap: swap}
	},
	"sectionCtx": func(v view, s sectionView) sectionCtx { return sectionCtx{View: v, Section: s} },
	"relRow":     func(rv relationView, rec record) relRow { return relRow{Rel: rv, Row: rec, ID: recordID(rec)} },
	"peekFor":    peekFor,
	"peekURL":    peekURL,
	"pathEscape": url.PathEscape,
	"domID":      domID,
	"endBtn":     newEndBtn,
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
// displayValue formatiert mit deutschen Ja/Nein-Texten (Tests, Rückfall);
// Templates nutzen displayValueIn mit den Texten der Sprache.
func displayValue(rec record, f metamodel.FieldDefinition) string {
	return displayValueIn(rec, f, "Ja", "Nein")
}

func displayValueIn(rec record, f metamodel.FieldDefinition, yes, no string) string {
	v, ok := rec[f.Key]
	if !ok || v == nil || f.Type == metamodel.TypePassword {
		return ""
	}
	// Lesbarer Text eines Verweises (vom Modul in "_labels" geliefert).
	if l, ok := labelsOf(rec)[f.Key]; ok && l != "" {
		return l
	}
	switch f.Type {
	case metamodel.TypeBoolean:
		if b, _ := v.(bool); b {
			return yes
		}
		return no
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
	case nil:
		return ""
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

// domID macht aus einer id einen gültigen Teil einer DOM-ID.
func domID(id string) string { return hex.EncodeToString([]byte(id)) }
