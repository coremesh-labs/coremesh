package main

import (
	"net/http"
	"slices"
	"strings"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// Feldberechtigungen (Feldgruppen, siehe crud.Access): Das Modul entfernt
// Felder ohne Leserecht aus den Datensätzen und markiert sie
// ("_hidden_fields"), Felder ohne Änderungsrecht ebenso ("_readonly_fields").
// Der WebServer
//
//   - blendet in Detail und Formular ausgeblendete Felder aus und zeigt nicht
//     änderbare schreibgeschützt,
//   - zeigt in Listen „—“ und lässt eine Spalte weg, wenn sie in keiner Zeile
//     sichtbar ist,
//   - schickt beim Speichern beide nie mit – so bleibt ihr Wert erhalten.
//
// Verbindlich prüft das Modul; die Marker wandern im Formular als _access mit.
// Für die Neuanlage gibt es keinen Datensatz: Dann zählt, ob eine Berechtigung
// die Feldgruppe überhaupt erlauben kann (readFields/changeFields).

type fieldAccess struct {
	hidden, readonly map[string]bool
}

func markerList(rec record, key string) map[string]bool {
	out := map[string]bool{}
	if l, ok := rec[key].([]any); ok {
		for _, v := range l {
			if s, ok := v.(string); ok {
				out[s] = true
			}
		}
	}
	return out
}

// accessOf liest die Marker eines Datensatzes.
func accessOf(rec record) fieldAccess {
	return fieldAccess{hidden: markerList(rec, "_hidden_fields"), readonly: markerList(rec, "_readonly_fields")}
}

func (a fieldAccess) empty() bool { return len(a.hidden) == 0 && len(a.readonly) == 0 }

// encode/decodeAccess: "iban,bic|credit_limit" (ausgeblendet|schreibgeschützt).
func (a fieldAccess) encode() string {
	if a.empty() {
		return ""
	}
	keys := func(m map[string]bool) string {
		var out []string
		for k := range m {
			out = append(out, k)
		}
		slices.Sort(out)
		return strings.Join(out, ",")
	}
	return keys(a.hidden) + "|" + keys(a.readonly)
}

func decodeAccess(s string) fieldAccess {
	a := fieldAccess{hidden: map[string]bool{}, readonly: map[string]bool{}}
	h, ro, _ := strings.Cut(s, "|")
	for _, k := range strings.Split(h, ",") {
		if k != "" {
			a.hidden[k] = true
		}
	}
	for _, k := range strings.Split(ro, ",") {
		if k != "" {
			a.readonly[k] = true
		}
	}
	return a
}

// createAccess: Feldgruppen bei der Neuanlage. Ausgeblendet, wenn keine
// Berechtigung die Gruppe sehen lässt; schreibgeschützt, wenn keine sie ändern
// lässt. Buchungskreis und weitere Werte prüft das Modul beim Speichern.
func (s *server) createAccess(r *http.Request, d metamodel.ObjectDefinition) fieldAccess {
	a := fieldAccess{hidden: map[string]bool{}, readonly: map[string]bool{}}
	if d.Authorization == nil || len(d.Authorization.FieldGroups) == 0 {
		return a
	}
	grants := func(action string) *sdk.GrantSet {
		resp, err := s.call(r, "Account", "Granted", map[string]any{"object": d.Name, "action": action})
		if err != nil {
			return &sdk.GrantSet{} // im Zweifel ausblenden – das Modul entscheidet
		}
		var g sdk.GrantSet
		if sdk.Decode(resp.Payload, &g) != nil {
			return &sdk.GrantSet{}
		}
		return &g
	}
	read, change := grants(metamodel.ActionReadFields), grants(metamodel.ActionChangeFields)
	for _, g := range d.Authorization.FieldGroups {
		for _, k := range g.Fields {
			switch {
			case !mayAllowGroup(read, g.Key):
				a.hidden[k] = true
			case !mayAllowGroup(change, g.Key):
				a.readonly[k] = true
			}
		}
	}
	return a
}

// mayAllowGroup: Kann irgendeine Regel die Feldgruppe erlauben (ohne die
// übrigen Werte zu kennen)?
func mayAllowGroup(g *sdk.GrantSet, group string) bool {
	return slices.ContainsFunc(g.Rules, func(r sdk.GrantRule) bool {
		vals, ok := r.Fields[metamodel.FieldGroupAttr]
		return !ok || slices.ContainsFunc(vals, func(v sdk.ValueRange) bool { return v.Matches(group) })
	})
}

// withoutFields: Definition ohne die Felder (Detailansicht eines Datensatzes).
func withoutFields(d metamodel.ObjectDefinition, hidden map[string]bool) metamodel.ObjectDefinition {
	if len(hidden) == 0 {
		return d
	}
	d.Fields = slices.DeleteFunc(slices.Clone(d.Fields), func(f metamodel.FieldDefinition) bool { return hidden[f.Key] })
	secs := slices.Clone(d.Sections)
	for i, sec := range secs {
		if len(sec.Fields) > 0 {
			sec.Fields = slices.DeleteFunc(slices.Clone(sec.Fields), func(k string) bool { return hidden[k] })
			secs[i] = sec
		}
	}
	d.Sections = secs
	return d
}

// hiddenEverywhere: Felder, die in keiner Zeile sichtbar sind (Spalten weglassen).
func hiddenEverywhere(rows []record) map[string]bool {
	out := map[string]bool{}
	for i, r := range rows {
		h := markerList(r, "_hidden_fields")
		if i == 0 {
			out = h
			continue
		}
		for k := range out {
			if !h[k] {
				delete(out, k)
			}
		}
	}
	return out
}

// hiddenIn: Ist das Feld im Datensatz ausgeblendet?
func hiddenIn(rec record, key string) bool { return markerList(rec, "_hidden_fields")[key] }
