package main

import (
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// Filterleiste der Übersicht (ObjectDefinition.Filters). Ja/Nein-Felder und
// Felder mit Optionen werden zur Auswahl. Verweisfelder (Lookup) zeigen Text
// statt Schlüssel:
//
//   - kleine Ziele (bis filterChoiceLimit Einträge): Auswahl mit den lesbaren
//     Texten, z. B. Kataloge;
//   - große Ziele: Eingabe mit Auswahldialog (wie im Formular), daneben der
//     Text zum gewählten Schlüssel.
//
// Lookup.Filters wirken auch hier: Die Quelle ist dann ein anderer Filter der
// Leiste (z. B. company_code) – ohne Wert bleibt die Auswahl ungefiltert, gleiche
// Schlüssel erscheinen nur einmal.

var filterChoiceLimit = 50 // var: Tests

// filterField ist ein Feld der Filterleiste.
type filterField struct {
	metamodel.FieldDefinition
	Select     bool               // Auswahl statt Eingabe
	Choices    []metamodel.Option // Auswahl (Optionen oder kleines Lookup-Ziel)
	Search     bool               // großes Lookup-Ziel: Eingabe mit Auswahldialog
	ValueLabel string             // Search: Text zum gewählten Schlüssel
}

// filterFields baut die Filterleiste mit den Werten der Anfrage. Aufzurufen
// vor dem Entfernen ausgeblendeter Spalten – Filter bleiben auch dann.
func (s *server) filterFields(r *http.Request, oc objectCtx, q url.Values) []filterField {
	var out []filterField
	for _, k := range oc.Def.Filters {
		i := slices.IndexFunc(oc.Def.Fields, func(f metamodel.FieldDefinition) bool { return f.Key == k })
		if i < 0 {
			continue
		}
		ff := filterField{FieldDefinition: oc.Def.Fields[i], Choices: oc.Def.Fields[i].Options, Select: len(oc.Def.Fields[i].Options) > 0}
		if ff.Lookup != nil && len(ff.Choices) == 0 && ff.Type != metamodel.TypeBoolean {
			s.lookupChoices(r, oc, &ff, q)
		}
		out = append(out, ff)
	}
	return out
}

// lookupChoices: Auswahl bei kleinem Ziel, sonst Eingabe mit Dialog.
func (s *server) lookupChoices(r *http.Request, oc objectCtx, ff *filterField, q url.Values) {
	lk := ff.Lookup
	ff.Search = true
	if v := strings.TrimSpace(q.Get(ff.Key)); v != "" {
		ff.ValueLabel = s.lookupLabels(r, metamodel.ObjectDefinition{Fields: []metamodel.FieldDefinition{ff.FieldDefinition}},
			map[string]string{ff.Key: v}, nil)[ff.Key]
	}
	tgt, err := s.targetDef(r, lk.Object)
	if err != nil {
		return
	}
	act, err := need(tgt, metamodel.KindList)
	if err != nil {
		return
	}
	query := map[string]any{}
	for target, src := range lk.Filters {
		if v := s.filterValue(r, oc.Object, src, q); v != "" {
			query[target] = v
		}
	}
	resp, err := s.call(r, tgt.Object, act.Name, map[string]any{"query": query})
	if err != nil {
		return
	}
	recs, err := records(resp.Payload)
	if err != nil || len(recs) > filterChoiceLimit {
		return
	}
	var choices []metamodel.Option
	for _, rec := range recs {
		v := scalar(rec[lk.ValueField])
		if v == "" || slices.ContainsFunc(choices, func(o metamodel.Option) bool { return o.Value == v }) {
			continue
		}
		var parts []string
		for _, k := range lk.LabelFields {
			if t := scalar(rec[k]); t != "" {
				parts = append(parts, t)
			}
		}
		label := strings.Join(parts, " ")
		if label == "" {
			label = v
		}
		choices = append(choices, metamodel.Option{Value: v, Label: label})
	}
	// Gewählter Wert außerhalb der Auswahl (z. B. anderer Buchungskreis): bleibt wählbar.
	if v := strings.TrimSpace(q.Get(ff.Key)); v != "" && !slices.ContainsFunc(choices, func(o metamodel.Option) bool { return o.Value == v }) {
		label := ff.ValueLabel
		if label == "" {
			label = v
		}
		choices = append(choices, metamodel.Option{Value: v, Label: label})
	}
	ff.Choices, ff.Search, ff.Select = choices, false, true
}
