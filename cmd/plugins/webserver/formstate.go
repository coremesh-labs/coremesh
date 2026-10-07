package main

import (
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/coremesh-lab/coremesh/pkg/sdk"
	"github.com/coremesh-lab/coremesh/pkg/sdk/metamodel"
)

// Masken: Welche Felder ein Formular zeigt, bestimmt nicht nur das Metamodell,
// sondern auch der aktuelle Stand der Eingaben:
//
//   - deklarativ: FieldDefinition.ShowIf / RequiredIf (ohne Plugin-Aufruf),
//   - im Plugin: ObjectDefinition.FormState – der WebServer schickt die Werte
//     (FormStateRequest) und erhält je Feld sichtbar, Pflicht, schreibgeschützt,
//     Wert und Auswahlwerte (FormState).
//
// Ändert sich ein Feld mit Trigger, lädt htmx nur den Formularinhalt neu
// (POST /m/{module}/{object}/_form → Block form-body). Beim Speichern gilt
// dieselbe Maske: Ausgeblendete Felder sind nie Pflicht und werden geleert.

// mask ist die wirksame Maske eines Formulars.
type mask struct {
	def      metamodel.ObjectDefinition // nur sichtbare Felder, Eigenschaften angepasst
	hidden   map[string]bool            // ausgeblendete (editierbare) Felder
	readonly map[string]bool            // von der Maske schreibgeschützt (Wert aus values)
	values   map[string]string          // Formularwerte inkl. Vorgaben des Plugins
	message  string
	denied   map[string]bool // ohne Änderungsrecht (Feldgruppe): nie mitschicken
}

func (s *server) mask(r *http.Request, oc objectCtx, mode, id string, values map[string]string, locked map[string]bool, acc fieldAccess) mask {
	vals := maps.Clone(values)
	if vals == nil {
		vals = map[string]string{}
	}
	var st metamodel.FormState
	if oc.Def.FormState != "" && (mode == "create" || mode == "edit") {
		req := metamodel.FormStateRequest{Mode: mode, ID: id, Values: vals, Locked: slices.Sorted(maps.Keys(locked))}
		if resp, err := s.call(r, oc.Object, oc.Def.FormState, req); err == nil {
			_ = sdk.Decode(resp.Payload, &st)
		} else {
			s.logError(r, "FormState", err)
		}
	}
	for k, fs := range st.Fields {
		if fs.Value != nil {
			vals[k] = *fs.Value
		}
	}
	m := mask{def: oc.Def, hidden: map[string]bool{}, readonly: map[string]bool{}, denied: map[string]bool{}, values: vals, message: st.Message}
	dispHidden, dispReadonly := oc.displayFor(displayValues(oc.Def, vals))
	m.def.Fields = nil
	for _, f := range oc.Def.Fields {
		if acc.hidden[f.Key] && !locked[f.Key] {
			continue // keine Leseberechtigung: weder anzeigen noch mitschicken
		}
		if acc.readonly[f.Key] && f.Editable {
			m.readonly[f.Key], m.denied[f.Key] = true, true
		}
		fs := st.Fields[f.Key]
		visible := f.ShowIf.Holds(vals)
		if f.RequiredIf != nil && f.RequiredIf.Holds(vals) {
			f.Required = true
		}
		if fs.Visible != nil {
			visible = *fs.Visible
		}
		if fs.Required != nil {
			f.Required = *fs.Required
		}
		if fs.ReadOnly != nil && *fs.ReadOnly && f.Editable {
			m.readonly[f.Key] = true // angezeigt, aber nicht änderbar; Wert kommt aus values
		}
		if fs.Options != nil {
			f.Options = fs.Options
		}
		// Darstellungsregeln: nur weiter einschränken; Pflichtfelder bleiben sichtbar.
		if dispHidden[f.Key] && visible && !f.Required && !locked[f.Key] {
			continue // nicht anzeigen, nicht mitschicken – der Wert bleibt
		}
		if dispReadonly[f.Key] && f.Editable && !locked[f.Key] {
			m.readonly[f.Key], m.denied[f.Key] = true, true
		}
		if !visible && !locked[f.Key] {
			if f.Editable {
				m.hidden[f.Key] = true
			}
			continue
		}
		m.def.Fields = append(m.def.Fields, f)
	}
	return m
}

// displayValues: Formularwerte für Bedingungen – Ja/Nein wie in Datensätzen.
func displayValues(d metamodel.ObjectDefinition, vals map[string]string) map[string]string {
	out := maps.Clone(vals)
	for _, f := range d.Fields {
		if f.Type == metamodel.TypeBoolean {
			out[f.Key] = strconv.FormatBool(vals[f.Key] == "on" || vals[f.Key] == "true")
		}
	}
	return out
}

// dynamic: Das Formular hat eine Maske, die sich mit den Eingaben ändert.
func dynamic(d metamodel.ObjectDefinition) bool {
	return d.FormState != "" || slices.ContainsFunc(d.Fields, func(f metamodel.FieldDefinition) bool { return f.Trigger })
}

// formValues liest die Werte der Felder aus einem Formular.
func formValues(d metamodel.ObjectDefinition, form url.Values) map[string]string {
	out := map[string]string{}
	for _, f := range d.Fields {
		if v, ok := form[f.Key]; ok && len(v) > 0 {
			out[f.Key] = strings.TrimSpace(v[0])
		}
	}
	return out
}

// parseMasked wertet ein abgeschicktes Formular mit der Maske aus: Vorgaben
// des Plugins gehen vor, ausgeblendete Felder werden geleert (nil).
func (s *server) parseMasked(r *http.Request, oc objectCtx, mode, id string) (map[string]any, map[string]string, map[string]string) {
	locked, _ := lockedFields(oc.Def, r.PostForm.Get("_lock"))
	acc := decodeAccess(r.PostForm.Get("_access"))
	if mode == "create" {
		acc = s.createAccess(r, oc.Def)
	}
	m := s.mask(r, oc, mode, id, formValues(oc.Def, r.PostForm), locked, acc)
	for k, v := range m.values {
		r.PostForm.Set(k, v)
	}
	data, raw, errs := parseFields(m.def, r.PostForm)
	for k := range m.hidden {
		data[k] = nil
	}
	for k := range m.denied {
		delete(data, k)
	}
	return data, raw, errs
}

// lookupLabels: lesbarer Text der Lookup-Werte eines Formulars (wenn der
// Datensatz keine _labels mitbringt, z. B. nach einer Neuauswertung).
func (s *server) lookupLabels(r *http.Request, d metamodel.ObjectDefinition, values, known map[string]string) map[string]string {
	out := maps.Clone(known)
	if out == nil {
		out = map[string]string{}
	}
	for _, f := range d.Fields {
		v := values[f.Key]
		if f.Lookup == nil || v == "" || out[f.Key] != "" || len(f.Lookup.LabelFields) == 0 {
			continue
		}
		tgt, err := s.targetDef(r, f.Lookup.Object)
		if err != nil {
			continue
		}
		act, err := need(tgt, metamodel.KindList)
		if err != nil {
			continue
		}
		resp, err := s.call(r, tgt.Object, act.Name, map[string]any{"query": map[string]any{f.Lookup.ValueField: v}})
		if err != nil {
			continue
		}
		recs, _ := records(resp.Payload)
		for _, rec := range recs {
			if scalar(rec[f.Lookup.ValueField]) != v {
				continue
			}
			var parts []string
			for _, k := range f.Lookup.LabelFields {
				if t := scalar(rec[k]); t != "" {
					parts = append(parts, t)
				}
			}
			out[f.Key] = strings.Join(parts, " ")
			break
		}
	}
	return out
}

// POST /m/{module}/{object}/_form – Formularinhalt für die aktuellen Eingaben
// neu aufbauen (Feld mit Trigger geändert).
func (s *server) formRefresh(w http.ResponseWriter, r *http.Request) {
	oc, err := s.loadObject(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.fail(w, r, err)
		return
	}
	values := formValues(oc.Def, r.PostForm)
	var v view
	if r.PostForm.Get("_mode") == "edit" {
		v = s.editView(r, oc, record{"id": r.PostForm.Get("_rid")}, values, nil, r.PostForm.Get("_view"))
	} else {
		v = s.createView(r, oc, values, nil)
	}
	s.render(w, r, http.StatusOK, "form-body", v, "", "")
}

// formGroup ist eine Feldgruppe des Formulars.
type formGroup struct {
	Title  string
	Fields []fieldCtx
}

// FormGroups gliedert die Felder nach FieldDefinition.Group (Reihenfolge der
// ersten Nennung; Felder ohne Gruppe vorne).
func (v view) FormGroups() []formGroup {
	var out []formGroup
	idx := map[string]int{}
	for _, f := range v.FormFields {
		i, ok := idx[f.Field.Group]
		if !ok {
			i = len(out)
			idx[f.Field.Group] = i
			out = append(out, formGroup{Title: f.Field.Group})
		}
		out[i].Fields = append(out[i].Fields, f)
	}
	slices.SortStableFunc(out, func(a, b formGroup) int {
		if (a.Title == "") == (b.Title == "") {
			return 0
		}
		if a.Title == "" {
			return -1
		}
		return 1
	})
	return out
}
