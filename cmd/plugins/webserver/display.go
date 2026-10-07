package main

import (
	"maps"
	"net/http"
	"slices"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// Darstellungsregeln (Administration → Darstellung, gepflegt in iam): Je Object
// blenden Regeln Felder aus oder machen sie unänderbar – optional nur für
// Rollen und nur, wenn alle Bedingungen auf Feldwerte zutreffen. Account.Display
// liefert die Regeln des Benutzers; ausgewertet wird hier je Datensatz bzw.
// Formularstand, als letzte Schicht nach Metamodell, FormState und
// Feldberechtigungen:
//
//   - Liste, Detail, Unterzeilen: ausgeblendete Felder zeigen „—“ bzw. fehlen,
//     Spalten fehlen, wenn sie in keiner Zeile sichtbar sind.
//   - Formular: ausgeblendete Felder fehlen, unänderbare sind schreibgeschützt;
//     beide werden nicht mitgeschickt, ihr Wert bleibt. Felder in Bedingungen
//     werten die Maske bei Änderung neu aus. Pflichtfelder bleiben sichtbar.
//   - Spalten (columns): nur in Listen und Unterzeilen ausgeblendet; Detail und
//     Formular bleiben – erlaubt auch für Schlüssel- und Pflichtfelder.
//   - Abschnitte (sections): fehlen im Detail; ihre Felder gelten überall als
//     ausgeblendet.
//
// Das ist Darstellung, kein Schutz: Über die API bleiben die Felder sichtbar.

type displayRule struct {
	Conditions []struct {
		Field  string   `json:"field"`
		Values []string `json:"values"`
	} `json:"conditions"`
	Hidden   []string `json:"hidden"`
	Readonly []string `json:"readonly"`
	Columns  []string `json:"columns"`
	Sections []string `json:"sections"`
}

// holds: Treffen alle Bedingungen auf die Werte zu?
func (d displayRule) holds(values map[string]string) bool {
	for _, c := range d.Conditions {
		if !slices.Contains(c.Values, values[c.Field]) {
			return false
		}
	}
	return true
}

// withDisplay lädt die Regeln des Objects und macht Felder aus Bedingungen zu
// Triggern, damit das Formular bei ihrer Änderung neu ausgewertet wird.
func (s *server) withDisplay(r *http.Request, oc objectCtx) objectCtx {
	resp, err := s.call(r, "Account", "Display", map[string]any{"object": oc.Object})
	if err != nil {
		return oc // ohne iam oder Regeln: unverändert
	}
	var set struct {
		Rules []displayRule `json:"rules"`
	}
	if sdk.Decode(resp.Payload, &set) != nil || len(set.Rules) == 0 {
		return oc
	}
	oc.Display = set.Rules
	conds := map[string]bool{}
	for _, d := range set.Rules {
		for _, c := range d.Conditions {
			conds[c.Field] = true
		}
	}
	fields := slices.Clone(oc.Def.Fields)
	for i, f := range fields {
		if conds[f.Key] && f.Editable {
			fields[i].Trigger = true
		}
	}
	oc.Def.Fields = fields
	return oc
}

// displayFor: Felder, die die Regeln bei diesen Werten ausblenden bzw. sperren
// (einschließlich der Felder ausgeblendeter Abschnitte).
func (oc objectCtx) displayFor(values map[string]string) (hidden, readonly map[string]bool) {
	hidden, readonly = map[string]bool{}, map[string]bool{}
	for _, d := range oc.Display {
		if !d.holds(values) {
			continue
		}
		for _, k := range d.Hidden {
			hidden[k] = true
		}
		for _, k := range d.Readonly {
			readonly[k] = true
		}
		for _, sd := range oc.Def.Sections {
			if slices.Contains(d.Sections, sd.Key) {
				for _, k := range sd.Fields {
					hidden[k] = true
				}
			}
		}
	}
	return hidden, readonly
}

// displayExtra: Spalten bzw. Abschnitte, die die Regeln bei diesen Werten ausblenden.
func (oc objectCtx) displayExtra(values map[string]string) (columns, sections map[string]bool) {
	columns, sections = map[string]bool{}, map[string]bool{}
	for _, d := range oc.Display {
		if !d.holds(values) {
			continue
		}
		for _, k := range d.Columns {
			columns[k] = true
		}
		for _, k := range d.Sections {
			sections[k] = true
		}
	}
	return columns, sections
}

// recordValues: Werte eines Datensatzes als Text (für Bedingungen).
func recordValues(d metamodel.ObjectDefinition, rec record) map[string]string {
	out := map[string]string{}
	for _, f := range d.Fields {
		out[f.Key] = scalar(rec[f.Key])
	}
	return out
}

// applyDisplay ergänzt "_hidden_fields" eines Datensatzes um die Felder, die
// die Regeln ausblenden, und setzt "_hidden_sections" (Detailansicht).
func (oc objectCtx) applyDisplay(rec record) { oc.applyDisplayTo(rec, false) }

// applyListDisplay: wie applyDisplay, zusätzlich ohne die ausgeblendeten
// Spalten (Liste, Unterzeilen).
func (oc objectCtx) applyListDisplay(rec record) { oc.applyDisplayTo(rec, true) }

func (oc objectCtx) applyDisplayTo(rec record, list bool) {
	if len(oc.Display) == 0 {
		return
	}
	values := recordValues(oc.Def, rec)
	hidden, _ := oc.displayFor(values)
	columns, sections := oc.displayExtra(values)
	if list {
		maps.Copy(hidden, columns)
	}
	if len(sections) > 0 {
		rec["_hidden_sections"] = addMarkers(nil, sections)
	}
	if len(hidden) > 0 {
		rec["_hidden_fields"] = addMarkers(rec["_hidden_fields"], hidden)
	}
}

// addMarkers ergänzt eine Markerliste ("_hidden_fields" u. a.) um Schlüssel.
func addMarkers(cur any, keys map[string]bool) []any {
	list, _ := cur.([]any)
	for _, k := range slices.Sorted(maps.Keys(keys)) {
		if !slices.Contains(list, any(k)) {
			list = append(list, k)
		}
	}
	return list
}

// forRow: Für die einzelne Tabellenzeile nach dem Speichern dieselben Spalten
// wie in der Liste (Darstellungsregeln, Spalten ohne Leserecht).
func (oc objectCtx) forRow(rec record, viewParam string) objectCtx {
	if viewParam == "row" {
		oc.applyListDisplay(rec)
		oc.Def = withoutFields(oc.Def, hiddenEverywhere([]record{rec}))
	}
	return oc
}
