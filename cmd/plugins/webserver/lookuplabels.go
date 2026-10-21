package main

import (
	"net/http"
	"slices"
	"strings"

	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// Texte der Verweise (seit 0.41.0): Liefert ein Modul zu einem Feld mit Lookup keinen
// Text in "_labels" – etwa weil das Ziel in einem anderen Modul liegt, z. B. der
// Benutzer (IAM) in „Geändert von“ –, holt der WebServer ihn beim Ziel: je Wert einmal
// je Anfrage, höchstens lookupLabelMax Werte je Feld. Angezeigt werden die LabelFields
// des Lookups, getrennt durch „ · “ (z. B. „admin · Administrator“). Fehlt das Recht
// oder das Ziel, bleibt der Schlüssel stehen.
//
// Listen zeigen bei Verweisen den Schlüssel (der Text steht im Tooltip) – außer das
// Ziel hat eine technische ID (ObjectDefinition.TechnicalID, z. B. Benutzer): Dann
// steht der Text in der Zelle (Marker "_text_cells").

const lookupLabelMax = 50

const textCellsKey = "_text_cells"

// textCell: Zelle zeigt den Text des Verweises statt des Schlüssels.
func textCell(rec record, key string) bool {
	list, _ := rec[textCellsKey].([]any)
	return slices.Contains(list, any(key)) && labelsOf(rec)[key] != ""
}

func (s *server) fillLookupLabels(r *http.Request, fields []metamodel.FieldDefinition, recs ...record) {
	cache := map[string]string{} // Object|Wert → Text
	tgts := map[string]*objectCtx{} // Object → Definition (nil = nicht lesbar)
	for _, f := range fields {
		if f.Lookup == nil || len(f.Lookup.LabelFields) == 0 || f.Lookup.By != "" {
			continue
		}
		var values []string
		seen := map[string]bool{}
		for _, rec := range recs {
			v := scalar(rec[f.Key])
			if v == "" || seen[v] || labelsOf(rec)[f.Key] != "" {
				continue
			}
			seen[v] = true
			values = append(values, v)
		}
		if len(values) > lookupLabelMax || len(recs) == 0 {
			continue
		}
		tgt, ok := tgts[f.Lookup.Object]
		if !ok {
			if oc, err := s.targetDef(r, f.Lookup.Object); err == nil {
				tgt = &oc
			}
			tgts[f.Lookup.Object] = tgt
		}
		if tgt == nil {
			continue
		}
		if tgt.Def.TechnicalID {
			for _, rec := range recs {
				rec[textCellsKey] = addMarkers(rec[textCellsKey], map[string]bool{f.Key: true})
			}
		}
		for _, v := range values {
			ck := f.Lookup.Object + "|" + f.Lookup.ValueField + "|" + v
			if _, ok := cache[ck]; !ok {
				cache[ck] = s.lookupText(r, *tgt, f.Lookup, v)
			}
		}
		for _, rec := range recs {
			t := cache[f.Lookup.Object+"|"+f.Lookup.ValueField+"|"+scalar(rec[f.Key])]
			if t == "" || labelsOf(rec)[f.Key] != "" {
				continue
			}
			labels, _ := rec["_labels"].(map[string]any)
			if labels == nil {
				labels = map[string]any{}
				rec["_labels"] = labels
			}
			labels[f.Key] = t
		}
	}
}

// lookupText: Text eines Werts beim Ziel – get bei ValueField id, sonst list mit Filter.
func (s *server) lookupText(r *http.Request, tgt objectCtx, lk *metamodel.Lookup, v string) string {
	var recs []record
	if lk.ValueField == "id" {
		if rec, err := s.fetchRecord(r, tgt, v); err == nil {
			recs = []record{rec}
		}
	} else if act, err := need(tgt, metamodel.KindList); err == nil {
		if resp, err := s.call(r, tgt.Object, act.Name, map[string]any{"query": map[string]any{lk.ValueField: v}}); err == nil {
			recs, _ = records(resp.Payload)
		}
	}
	for _, rec := range recs {
		if scalar(rec[lk.ValueField]) != v {
			continue
		}
		var parts []string
		for _, k := range lk.LabelFields {
			if t := strings.TrimSpace(scalar(rec[k])); t != "" && t != v {
				parts = append(parts, t)
			}
		}
		return strings.Join(parts, " · ")
	}
	return ""
}

// listableFields: Spalten der Übersicht.
func listableFields(d metamodel.ObjectDefinition) []metamodel.FieldDefinition {
	var out []metamodel.FieldDefinition
	for _, f := range d.Fields {
		if f.Listable {
			out = append(out, f)
		}
	}
	return out
}
