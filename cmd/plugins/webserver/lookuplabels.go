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
	cache := map[string]string{}    // Object|Wertfeld|Wert → Text
	tgts := map[string]*objectCtx{} // Object → Definition (nil = nicht lesbar)
	target := func(object string) *objectCtx {
		tgt, ok := tgts[object]
		if !ok {
			if oc, err := s.targetDef(r, object); err == nil {
				tgt = &oc
			}
			tgts[object] = tgt
		}
		return tgt
	}
	for _, f := range fields {
		if f.Lookup == nil || len(recs) == 0 {
			continue
		}
		// Je Datensatz der Lookup seiner Variante (Lookup.By, seit 0.65.0 – z. B. das
		// Objekt je Objektart); ohne By ist es für alle derselbe.
		type want struct {
			lk *metamodel.Lookup
			ck string
		}
		wants := make([]want, len(recs))
		distinct := map[string]*metamodel.Lookup{}
		for i, rec := range recs {
			v := scalar(rec[f.Key])
			if v == "" || labelsOf(rec)[f.Key] != "" {
				continue
			}
			lk := f.Lookup.For(func(k string) string { return scalar(rec[k]) })
			if lk.Object == "" || len(lk.LabelFields) == 0 {
				continue
			}
			ck := lk.Object + "|" + lk.ValueField + "|" + v
			wants[i] = want{lk, ck}
			distinct[ck] = lk
		}
		if len(distinct) > lookupLabelMax {
			continue
		}
		for i, rec := range recs {
			wt := wants[i]
			if wt.lk == nil {
				if f.Lookup.By == "" {
					// Text kam vom Modul: die Zelle zeigt ihn trotzdem bei technischer ID.
					if tgt := target(f.Lookup.Object); tgt != nil && tgt.Def.TechnicalID {
						rec[textCellsKey] = addMarkers(rec[textCellsKey], map[string]bool{f.Key: true})
					}
				}
				continue
			}
			tgt := target(wt.lk.Object)
			if tgt == nil {
				continue
			}
			if tgt.Def.TechnicalID {
				rec[textCellsKey] = addMarkers(rec[textCellsKey], map[string]bool{f.Key: true})
			}
			t, ok := cache[wt.ck]
			if !ok {
				t = s.lookupText(r, *tgt, wt.lk, scalar(rec[f.Key]))
				cache[wt.ck] = t
			}
			if t == "" {
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
