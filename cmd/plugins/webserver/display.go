package main

import (
	"net/http"
	"slices"

	"github.com/camel/coremesh/pkg/sdk"
	"github.com/camel/coremesh/pkg/sdk/metamodel"
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
//
// Das ist Darstellung, kein Schutz: Über die API bleiben die Felder sichtbar.

type displayRule struct {
	Conditions []struct {
		Field  string   `json:"field"`
		Values []string `json:"values"`
	} `json:"conditions"`
	Hidden   []string `json:"hidden"`
	Readonly []string `json:"readonly"`
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
// Triggern (die Maske wertet bei ihrer Änderung neu aus).
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

// displayFor: Felder, die die Regeln bei diesen Werten ausblenden bzw. sperren.
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
	}
	return hidden, readonly
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
// die Regeln ausblenden (Liste, Detail, Unterzeilen).
func (oc objectCtx) applyDisplay(rec record) {
	if len(oc.Display) == 0 {
		return
	}
	hidden, _ := oc.displayFor(recordValues(oc.Def, rec))
	if len(hidden) == 0 {
		return
	}
	list, _ := rec["_hidden_fields"].([]any)
	for k := range hidden {
		if !slices.Contains(list, any(k)) {
			list = append(list, k)
		}
	}
	rec["_hidden_fields"] = list
}
