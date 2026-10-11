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
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
	"github.com/coremesh-labs/coremesh/pkg/sdk/module"
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
			if hiddenIn(rec, f.Key) {
				return "—" // keine Berechtigung für die Feldgruppe
			}
			return displayValueIn(rec, f, t("core.app.yes"), t("core.app.no"))
		},
		// Listen (seit 0.36.0): bei Verweisen der Schlüssel (kurz, lernt man), der Text als Tooltip
		"cell": func(rec record, f metamodel.FieldDefinition) template.HTML {
			if hiddenIn(rec, f.Key) {
				return "—"
			}
			if f.Lookup != nil {
				if v, ok := rec[f.Key]; ok && v != nil && scalar(v) != "" {
					return template.HTML(template.HTMLEscapeString(scalar(v)))
				}
			}
			if f.Type == metamodel.TypeSelect && labelsOf(rec)[f.Key] == "" {
				return selectCell(rec, f)
			}
			return template.HTML(template.HTMLEscapeString(displayValueIn(rec, f, t("core.app.yes"), t("core.app.no"))))
		},
		"cellTitle": func(rec record, f metamodel.FieldDefinition) string {
			if hiddenIn(rec, f.Key) {
				return ""
			}
			return displayValueIn(rec, f, t("core.app.yes"), t("core.app.no"))
		},
	}
}

// cellMaxRunes: Auswahltexte bis zu dieser Länge stehen in Listen ausgeschrieben.
const cellMaxRunes = 14

// selectCell: Auswahlwert in Listen – kurzer Text ausgeschrieben, langer als
// Abkürzung (Option.Short, Text vor der Klammer, sonst Schlüssel) mit ⓘ (Tooltip).
func selectCell(rec record, f metamodel.FieldDefinition) template.HTML {
	v, ok := rec[f.Key]
	if !ok || v == nil {
		return ""
	}
	vals := []string{scalar(v)}
	if f.Multiple {
		vals = strings.Split(scalar(v), ",")
	}
	var parts []string
	for _, x := range vals {
		x = strings.TrimSpace(x)
		if x == "" {
			continue
		}
		label, short := x, ""
		for _, o := range f.Options {
			if o.Value == x {
				label, short = o.Label, o.Short
			}
		}
		if utf8.RuneCountInString(label) <= cellMaxRunes {
			parts = append(parts, template.HTMLEscapeString(label))
			continue
		}
		abbr := short
		if abbr == "" {
			if s := shortLabel(label); utf8.RuneCountInString(s) <= cellMaxRunes {
				abbr = s
			} else {
				abbr = x
			}
		}
		parts = append(parts, template.HTMLEscapeString(abbr)+`&#8288;<span class="info" title="`+template.HTMLEscapeString(label)+`">ⓘ</span>`)
	}
	return template.HTML(strings.Join(parts, ", "))
}

// shortLabel: Spaltenkopf ohne Erläuterung in Klammern („Fällig am (leer = …)“ → „Fällig am“).
func shortLabel(s string) string {
	if i := strings.Index(s, " ("); i > 0 {
		return s[:i]
	}
	return s
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
	"shortLabel":  shortLabel,
	"raw":         func(rec record, key string) string { return scalar(rec[key]) },
	"locales":     func() []string { return metamodel.Locales },
	"tagFieldCtx": func(ed *tagEditor, f tagField) tagFieldCtx { return tagFieldCtx{Ed: ed, F: f} },
	"historyCtx": func(url string, on bool, target, swap string) historyCtx {
		return historyCtx{URL: url, On: on, Target: target, Swap: swap}
	},
	"sectionCtx":    func(v view, s sectionView) sectionCtx { return sectionCtx{View: v, Section: s} },
	"relRow":        func(rv relationView, rec record) relRow { return relRow{Rel: rv, Row: rec, ID: recordID(rec)} },
	"peekFor":       peekFor,
	"locked":        locked,
	"recordActions": recordActions,
	"recordAction": func(url, id string, a metamodel.ActionConfig, class string) recordActionCtx {
		return recordActionCtx{URL: url, ID: id, Action: a, Class: class}
	},
	"peekURL":    peekURL,
	"pathEscape": url.PathEscape,
	"domID":      domID,
	"endBtn":     newEndBtn,
	"numeric":    func(s string) bool { return numericRe.MatchString(s) },
	// Mehrfachauswahl: Wert in der Komma-Liste, sichtbare Zeilen der Liste.
	"inList":     func(v, list string) bool { return v != "" && slices.Contains(strings.Split(list, ","), v) },
	"selectSize": func(opts []metamodel.Option) int { return min(max(len(opts), 2), 8) },
	"json": func(v any) string {
		b, _ := json.MarshalIndent(v, "", "  ")
		return string(b)
	},
}

// numericRe: Betrag oder Zahl (rechtsbündig in Ergebnistabellen), z. B. -1.234,56 oder 300.00.
var numericRe = regexp.MustCompile(`^-?([0-9]{1,3}([.,' ][0-9]{3})+|[0-9]+)([.,][0-9]+)?$`)

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
		if f.Multiple {
			var labels []string
			for _, x := range strings.Split(s, ",") {
				l := x
				for _, o := range f.Options {
					if o.Value == x {
						l = o.Label
					}
				}
				if l != "" {
					labels = append(labels, l)
				}
			}
			return strings.Join(labels, ", ")
		}
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
