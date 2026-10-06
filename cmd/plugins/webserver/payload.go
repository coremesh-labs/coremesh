package main

import (
	"fmt"
	"net/mail"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/camel/coremesh/pkg/sdk/metamodel"
)

// Payload-Konventionen zwischen WebServer und Fachmodulen (siehe README.md):
//
//	list    {"query": {<URL-Parameter>}}      → [record…] oder {"items": [record…]}
//	item    {"id": "<id>"}                    → record
//	create  {"data": {<Felder>}}              → record (mit "id")
//	update  {"id": "<id>", "data": {<Felder>}} → record
//	delete  {"id": "<id>"}                    → beliebig
//	custom  {"id"?: "<id>", "data": {<Felder>}} → beliebig; "message" wird angezeigt
//
// Ein record ist ein JSON-Objekt; der Schlüssel "id" identifiziert ihn.

// record ist ein Datensatz eines Business-Objects.
type record map[string]any

func recordID(r record) string {
	if v, ok := r["id"]; ok && v != nil {
		return scalar(v)
	}
	return ""
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
				errs[f.Key] = "Pflichtfeld"
			}
			data[f.Key] = nil
			continue
		}
		switch f.Type {
		case metamodel.TypeNumber:
			n, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 64)
			if err != nil {
				errs[f.Key] = "Zahl erwartet"
				continue
			}
			data[f.Key] = n
		case metamodel.TypeEmail:
			if a, err := mail.ParseAddress(v); err != nil || a.Address != v {
				errs[f.Key] = "Ungültige E-Mail-Adresse"
				continue
			}
			data[f.Key] = v
		case metamodel.TypeDate:
			if _, err := time.Parse(time.DateOnly, v); err != nil {
				errs[f.Key] = "Datum im Format JJJJ-MM-TT erwartet"
				continue
			}
			data[f.Key] = v
		case metamodel.TypeSelect:
			if !slices.ContainsFunc(f.Options, func(o metamodel.Option) bool { return o.Value == v }) {
				errs[f.Key] = "Ungültige Auswahl"
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
	Module    string // Namensraum des Moduls
	Object    string
	URL       string // /m/{module}/{object}
	ActionURL string // /action/{module}/{object}
	Def       metamodel.ObjectDefinition
	Has       map[string]*metamodel.ActionConfig // Kind → Action (list, item, create, update, delete)
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
	Mode       string // create | edit | action
	Modal      bool   // im Dialog (HTMX) oder als eigene Seite
	FormTitle  string
	FormAction string
	Target     string // HTMX-Ziel der Antwort (edit)
	ViewParam  string // row | detail – wohin die Antwort von update gehört
	CancelURL  string
	ActionID   string // id für custom-Actions
	FormFields []fieldCtx
	FormError  string

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
}

var inputTypes = map[metamodel.FieldType]string{
	metamodel.TypeText: "text", metamodel.TypeEmail: "email", metamodel.TypeNumber: "number", metamodel.TypeDate: "date",
	metamodel.TypePassword: "password",
}

// buildFields erstellt die Formularfelder. create/action zeigen nur
// bearbeitbare Felder; edit zeigt alle, nicht bearbeitbare schreibgeschützt.
func buildFields(d metamodel.ObjectDefinition, mode string, values, errs map[string]string) []fieldCtx {
	var out []fieldCtx
	for _, f := range d.Fields {
		if !f.Editable && mode != "edit" {
			continue
		}
		it := inputTypes[f.Type]
		if it == "" {
			it = "text"
		}
		out = append(out, fieldCtx{
			Field: f, Type: string(f.Type), InputType: it,
			Value: values[f.Key], Error: errs[f.Key], ReadOnly: !f.Editable,
		})
	}
	return out
}
