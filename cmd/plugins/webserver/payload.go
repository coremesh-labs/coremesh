package main

import (
	"fmt"
	"net/mail"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// Payload-Konventionen zwischen WebServer und Fachmodulen (siehe README.md):
//
//	list    {"query": {<URL-Parameter>}}      → [record…] oder {"items": [record…]}
//	item    {"id": "<id>"}                    → record
//	create  {"data": {<Felder>}}              → record (mit "id")
//	update  {"id": "<id>", "data": {<Felder>}} → record
//	expire      {"id": "<id>", "valid_to": "JJJJ-MM-TT"} → record (Typ timeslice)
//	deactivate  {"id": "<id>"}                    → record (Typ status)
//	custom  {"id"?: "<id>", "data": {<Felder>}} → beliebig; "message" wird angezeigt
//
// Ein record ist ein JSON-Objekt. "_id" identifiziert ihn (bei Zeitscheiben inkl.
// Beginndatum, z. B. "4711|2026-01-01"); fehlt "_id", gilt "id". "id" ist der
// fachliche Schlüssel, auf den Verweise zeigen (z. B. bp_id eines Unter-Objects).

// record ist ein Datensatz eines Business-Objects.
type record map[string]any

func recordID(r record) string {
	for _, k := range []string{"_id", "id"} {
		if v, ok := r[k]; ok && v != nil {
			return scalar(v)
		}
	}
	return ""
}

// businessKey ist der fachliche Schlüssel ("id"), auf den andere Datensätze
// verweisen – bei Zeitscheiben ohne Beginndatum. Fehlt er, die Datensatz-ID.
func businessKey(r record) string {
	if v, ok := r["id"]; ok && v != nil {
		return scalar(v)
	}
	return recordID(r)
}

// records liest die Antwort einer list-Action.
func records(payload any) ([]record, error) {
	var items []any
	switch p := payload.(type) {
	case nil:
		return nil, nil
	case []any:
		items = p
	case map[string]any:
		list, ok := p["items"].([]any)
		if !ok {
			return nil, fmt.Errorf("list-Antwort: Liste oder {\"items\": [...]} erwartet")
		}
		items = list
	default:
		return nil, fmt.Errorf("list-Antwort: unerwarteter Typ %T", payload)
	}
	out := make([]record, 0, len(items))
	for i, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("list-Antwort: Eintrag %d ist kein Objekt", i)
		}
		out = append(out, m)
	}
	return out, nil
}

// asRecord liest die Antwort einer item/create/update-Action.
func asRecord(payload any) record {
	if m, ok := payload.(map[string]any); ok {
		return m
	}
	return record{}
}

// queryParams übernimmt alle URL-Parameter: einfache als String, mehrfache als Liste.
func queryParams(q url.Values) map[string]any {
	out := make(map[string]any, len(q))
	for k, v := range q {
		if len(v) == 1 {
			out[k] = v[0]
		} else {
			out[k] = slices.Clone(v)
		}
	}
	return out
}

// parseFields wandelt Formulardaten anhand des Metamodells in typisierte
// Werte. Nur Felder mit Editable: true werden übernommen – alles andere im
// Formular wird ignoriert (kein Mass Assignment). raw enthält die
// Eingaben für das erneute Anzeigen des Formulars, errs die Feldfehler.
func parseFields(d metamodel.ObjectDefinition, form url.Values) (data map[string]any, raw, errs map[string]string) {
	data, raw, errs = map[string]any{}, map[string]string{}, map[string]string{}
	for _, f := range d.Fields {
		if !f.Editable {
			continue
		}
		v := strings.TrimSpace(form.Get(f.Key))
		raw[f.Key] = v

		if f.Type == metamodel.TypeBoolean {
			data[f.Key] = v != "" // Checkbox: fehlt = false
			continue
		}
		if v == "" {
			if f.Required {
				errs[f.Key] = "core.validation.required"
			}
			data[f.Key] = nil
			continue
		}
		switch f.Type {
		case metamodel.TypeNumber:
			n, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 64)
			if err != nil {
				errs[f.Key] = "core.validation.number"
				continue
			}
			data[f.Key] = n
		case metamodel.TypeEmail:
			if a, err := mail.ParseAddress(v); err != nil || a.Address != v {
				errs[f.Key] = "core.validation.email"
				continue
			}
			data[f.Key] = v
		case metamodel.TypeDate:
			if _, err := time.Parse(time.DateOnly, v); err != nil {
				errs[f.Key] = "core.validation.date"
				continue
			}
			data[f.Key] = v
		case metamodel.TypeSelect:
			if !slices.ContainsFunc(f.Options, func(o metamodel.Option) bool { return o.Value == v }) {
				errs[f.Key] = "core.validation.option"
				continue
			}
			data[f.Key] = v
		default:
			data[f.Key] = v
		}
	}
	return data, raw, errs
}

// --- View-Modelle für die Templates ---------------------------------------

// objectCtx ist ein Business-Object mit Metamodell und den Actions je Kind.
type objectCtx struct {
	tr func(key string, args ...any) string // Framework-Texte in der Sprache der Anfrage (siehe T)

	Module    string // Namensraum des Moduls
	Object    string
	URL       string // /m/{module}/{object}
	ActionURL string // /action/{module}/{object}
	Def       metamodel.ObjectDefinition
	Has       map[string]*metamodel.ActionConfig // Kind → Action (list, item, create, update, delete)
	Display   []displayRule                      // Darstellungsregeln des Benutzers (display.go)
	Custom    []metamodel.ActionConfig           // Kind custom
	Denied    map[string]bool                    // Kinds bzw. custom-Namen ohne Berechtigung
}

func newObjectCtx(module, object string, d metamodel.ObjectDefinition) objectCtx {
	oc := objectCtx{Module: module, Object: object, URL: moduleURL(module, object), ActionURL: actionURL(module, object),
		Def: d, Has: map[string]*metamodel.ActionConfig{}}
	for i := range d.Actions {
		a := &d.Actions[i]
		if a.Kind == metamodel.KindCustom {
			oc.Custom = append(oc.Custom, *a)
		} else if _, dup := oc.Has[string(a.Kind)]; !dup {
			oc.Has[string(a.Kind)] = a
		}
	}
	return oc
}

// visibleFor entfernt Actions, die der Benutzer nicht aufrufen darf – die
// Buttons erscheinen dann nicht. Verbindlich prüft der Dispatcher.
func (oc objectCtx) visibleFor(u *user) objectCtx {
	if u == nil {
		return oc
	}
	has, denied := map[string]*metamodel.ActionConfig{}, map[string]bool{}
	for kind, a := range oc.Has {
		if u.Can(oc.Object, a.Name) {
			has[kind] = a
		} else {
			denied[kind] = true
		}
	}
	var custom []metamodel.ActionConfig
	for _, a := range oc.Custom {
		if u.Can(oc.Object, a.Name) {
			custom = append(custom, a)
		} else {
			denied["custom:"+a.Name] = true
		}
	}
	oc.Denied = denied
	oc.Has, oc.Custom = has, custom
	return oc
}

// ObjectActions sind die custom-Actions der Übersicht, RecordActions die der
// Detailansicht (ActionConfig.Record).
func (oc objectCtx) ObjectActions() []metamodel.ActionConfig { return oc.customs(false) }
func (oc objectCtx) RecordActions() []metamodel.ActionConfig { return oc.customs(true) }

func (oc objectCtx) customs(record bool) []metamodel.ActionConfig {
	var out []metamodel.ActionConfig
	for _, a := range oc.Custom {
		if a.Record == record {
			out = append(out, a)
		}
	}
	return out
}

func (oc objectCtx) custom(name string) *metamodel.ActionConfig {
	for i := range oc.Custom {
		if oc.Custom[i].Name == name {
			return &oc.Custom[i]
		}
	}
	return nil
}

// view ist das Datenmodell aller Fragmente.
type view struct {
	objectCtx
	Rows   []record
	Record record

	// Formulare
	Mode         string // create | edit | action
	Modal        bool   // im Dialog (HTMX) oder als eigene Seite
	FormTitle    string
	FormAction   string
	Target       string // HTMX-Ziel der Antwort (edit)
	ViewParam    string // row | detail – wohin die Antwort von update gehört
	CancelURL    string
	ActionID     string // id für custom-Actions
	FormFields   []fieldCtx
	FormError    string
	Filter       map[string]string // Liste: aktive Filter (q und ObjectDefinition.Filters)
	FilterBar    []filterField     // Liste: Felder der Filterleiste
	ListURL      string            // Liste: URL mit den aktiven Filtern (Neuladen)
	FormMessage  string            // Hinweis der Maske (FormState)
	FormStateURL string            // Neuauswertung der Maske (leer = statisch)
	History      bool              // Liste inkl. beendeter / inaktiver Einträge (?includeHistory=true)

	// Ende-Dialog (Lebenszyklus): timeslice → Datum, status → Bestätigung
	EndKind   string // timeslice | status
	EndDate   string // vorgeschlagenes/eingegebenes Enddatum (nie automatisch heute)
	EndMin    string // frühestes Enddatum (gültig ab)
	EndSubmit string // Text des Buttons, wenn die Ende-Action einen eigenen Text hat
	Locked    string // _lock: feste Felder (Komma-Liste), wandert im Formular mit
	Access    string // _access: Feldberechtigungen des Datensatzes (fieldaccess.go)

	// Ergebnisse
	Action  metamodel.ActionConfig
	Result  any
	Message string
	Toast   *toast
}

// ID ist die id des aktuellen Datensatzes.
func (v view) ID() string { return recordID(v.Record) }

// Row liefert die Sicht auf eine Tabellenzeile.
func (v view) Row(r record) view {
	v.Record = r
	return v
}

type toast struct {
	Level   string // success | error
	Message string
}

// fieldCtx ist ein Formularfeld mit Wert und Fehler.
type fieldCtx struct {
	Field     metamodel.FieldDefinition
	Type      string
	InputType string
	Value     string
	Error     string
	ReadOnly  bool
	Hidden    bool   // festes Feld (z. B. Fremdschlüssel im Master-Detail), als type=hidden
	Object    string // Object des Formulars (Lookup-Dialog: /lookup?from=…)
	Label     string // lesbarer Text eines Lookup-Werts
	Trigger   string // URL der Neuauswertung, wenn das Feld die Maske beeinflusst
}

var inputTypes = map[metamodel.FieldType]string{
	metamodel.TypeText: "text", metamodel.TypeEmail: "email", metamodel.TypeNumber: "number", metamodel.TypeDate: "date",
	metamodel.TypePassword: "password",
}

// buildFields erstellt die Formularfelder. create/action zeigen nur
// bearbeitbare Felder; edit zeigt alle, nicht bearbeitbare schreibgeschützt.
//
// formOpts: labels sind die lesbaren Texte der Lookup-Felder (aus "_labels"),
// locked die Felder, die als verstecktes Feld fest mitgehen – etwa der
// Fremdschlüssel eines Unter-Objects in der Master-Detail-Ansicht.
func buildFields(d metamodel.ObjectDefinition, mode string, values, errs map[string]string, opts formOpts) []fieldCtx {
	var out []fieldCtx
	for _, f := range d.Fields {
		if !f.Editable && mode != "edit" && !opts.locked[f.Key] {
			continue
		}
		it := inputTypes[f.Type]
		if it == "" {
			it = "text"
		}
		out = append(out, fieldCtx{
			Field: f, Type: string(f.Type), InputType: it, Object: d.Name,
			Value: values[f.Key], Error: errs[f.Key], ReadOnly: !f.Editable || opts.readonly[f.Key],
			Hidden: opts.locked[f.Key], Label: opts.labels[f.Key],
			Trigger: triggerURL(f, opts.trigger),
		})
	}
	return out
}

type formOpts struct {
	labels   map[string]string
	locked   map[string]bool
	readonly map[string]bool // von der Maske schreibgeschützt
	trigger  string          // URL der Neuauswertung (formStateURL)
}

// lockedFields liest _lock (Komma-Liste von Feld-Keys) aus Query oder Formular.
func lockedFields(d metamodel.ObjectDefinition, raw string) (map[string]bool, string) {
	locked := map[string]bool{}
	var keys []string
	for _, k := range strings.Split(raw, ",") {
		k = strings.TrimSpace(k)
		if slices.ContainsFunc(d.Fields, func(f metamodel.FieldDefinition) bool { return f.Key == k }) && !locked[k] {
			locked[k] = true
			keys = append(keys, k)
		}
	}
	return locked, strings.Join(keys, ",")
}

// labelsOf liest "_labels" eines Datensatzes.
func labelsOf(rec record) map[string]string {
	out := map[string]string{}
	if m, ok := rec["_labels"].(map[string]any); ok {
		for k, v := range m {
			if s, ok := v.(string); ok {
				out[k] = s
			}
		}
	}
	return out
}

// actionFields beschränkt das Formular einer custom-Action auf ActionConfig.Fields
// (in deren Reihenfolge); ohne Angabe bleiben alle Felder.
func actionFields(all []fieldCtx, keys []string) []fieldCtx {
	if len(keys) == 0 {
		return all
	}
	var out []fieldCtx
	for _, k := range keys {
		for _, f := range all {
			if f.Field.Key == k {
				out = append(out, f)
			}
		}
	}
	return out
}

// actionDef: das Metamodell mit nur den Feldern einer custom-Action (Auswertung des Formulars).
func actionDef(d metamodel.ObjectDefinition, keys []string) metamodel.ObjectDefinition {
	if len(keys) == 0 {
		return d
	}
	fields := d.Fields
	d.Fields = nil
	for _, f := range fields {
		if slices.Contains(keys, f.Key) {
			d.Fields = append(d.Fields, f)
		}
	}
	return d
}

// recordActionCtx sind die Daten des Blocks record-action.
type recordActionCtx struct {
	URL, ID, Class string
	Action         metamodel.ActionConfig
}

// formStateURL: Neuauswertung der Maske ("" = Formular ohne dynamische Maske).
func formStateURL(oc objectCtx) string {
	if !dynamic(oc.Def) {
		return ""
	}
	return oc.URL + "/_form"
}

// triggerURL: Ein Feld löst die Neuauswertung aus, wenn es als Trigger markiert
// ist – oder jedes Feld, wenn das Plugin die Maske bestimmt (FormState).
func triggerURL(f metamodel.FieldDefinition, url string) string {
	if url == "" || !f.Trigger {
		return ""
	}
	return url
}

// FilterFields sind die Felder der Filterleiste der Übersicht (filters.go).
func (v view) FilterFields() []filterField { return v.FilterBar }
