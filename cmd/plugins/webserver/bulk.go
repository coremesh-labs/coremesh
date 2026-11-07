package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// maxBulk begrenzt die Zeilen einer Sammelaktion (eine Anfrage, nacheinander ausgeführt).
const maxBulk = 500

// POST /action/{module}/{object}/{name}/_bulk (seit 0.55.0): Aktion am Datensatz
// (ActionConfig.Bulk) für die angekreuzten Zeilen der Übersicht (_ids), eine nach der
// anderen. Fehler einer Zeile halten die anderen nicht auf; das Ergebnis nennt
// sie mit der Bezeichnung der Zeile. Danach lädt die Liste neu.
func (s *server) bulkAction(w http.ResponseWriter, r *http.Request) {
	oc, act, err := s.loadAction(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if !act.Record || !act.Bulk || len(act.Fields) > 0 {
		s.fail(w, r, fmt.Errorf("%w: %q ist keine Sammelaktion", sdk.ErrInvalidArgument, act.Name))
		return
	}
	if err := r.ParseForm(); err != nil {
		s.fail(w, r, fmt.Errorf("%w: %v", sdk.ErrInvalidArgument, err))
		return
	}
	ids := bulkIDs(r.PostForm["_ids"])
	if len(ids) == 0 {
		s.fail(w, r, fmt.Errorf("%w: %s", sdk.ErrInvalidArgument, s.T(r, "core.bulk.none")))
		return
	}
	if len(ids) > maxBulk {
		s.fail(w, r, fmt.Errorf("%w: höchstens %d Zeilen auf einmal", sdk.ErrInvalidArgument, maxBulk))
		return
	}
	s.runBulk(w, r, oc, act, ids, map[string]any{})
}

// bulkIDs: angekreuzte Zeilen ohne leere und doppelte.
func bulkIDs(in []string) []string {
	var ids []string
	seen := map[string]bool{}
	for _, id := range in {
		if id = strings.TrimSpace(id); id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

// bulkView: Formular einer Sammelaktion mit Feldern (seit 0.59.0) – Maske wie für die
// erste Zeile, die Zeilen als _ids.
func (s *server) bulkView(r *http.Request, oc objectCtx, act *metamodel.ActionConfig, ids []string, values, errs map[string]string) view {
	v := s.actionView(r, oc, act, ids[0], values, errs)
	v.ActionID, v.BulkIDs = "", ids
	v.FormTitle = s.T(r, "core.bulk.form_title", oc.Def.Title, act.Label, len(ids))
	return v
}

// runBulk: Aktion für jede Zeile mit denselben Eingaben; Ergebnis mit den Fehlern je Zeile.
func (s *server) runBulk(w http.ResponseWriter, r *http.Request, oc objectCtx, act *metamodel.ActionConfig, ids []string, data map[string]any) {
	if len(ids) > maxBulk {
		s.fail(w, r, fmt.Errorf("%w: höchstens %d Zeilen auf einmal", sdk.ErrInvalidArgument, maxBulk))
		return
	}
	table := &resultTable{Columns: []string{s.T(r, "core.bulk.entry"), s.T(r, "core.bulk.error")}}
	ok := 0
	for _, id := range ids {
		if r.Context().Err() != nil {
			break
		}
		_, err := s.call(r, oc.Object, act.Name, map[string]any{"id": id, "data": data})
		if err == nil {
			ok++
			continue
		}
		if !errors.Is(err, sdk.ErrInvalidArgument) && !errors.Is(err, sdk.ErrFailedPrecondition) &&
			!errors.Is(err, sdk.ErrNotFound) && !errors.Is(err, sdk.ErrPermissionDenied) && !errors.Is(err, sdk.ErrAlreadyExists) {
			s.logError(r, "Sammelaktion "+oc.Object+"."+act.Name, err)
		}
		table.Rows = append(table.Rows, []string{s.rowLabel(r, oc, id), err.Error()})
	}
	level := "success"
	if ok < len(ids) {
		level = "warning"
	}
	msg := s.T(r, "core.bulk.done", act.Label, ok, len(ids))
	v := view{objectCtx: oc, Action: *act, Modal: isHTMX(r), Message: msg, Toast: &toast{Level: level, Message: msg}}
	if len(table.Rows) > 0 {
		v.ResultTable = table
	}
	w.Header().Set("HX-Trigger", "coremesh-changed")
	s.render(w, r, http.StatusOK, "result", v, act.Label, oc.Object)
}

// rowLabel: Bezeichnung einer Zeile für Meldungen – die ersten Spalten der Übersicht
// (wie angezeigt), sonst die id.
func (s *server) rowLabel(r *http.Request, oc objectCtx, id string) string {
	item := oc.Has["item"]
	if item == nil {
		return id
	}
	resp, err := s.call(r, oc.Object, item.Name, map[string]any{"id": id})
	if err != nil {
		return id
	}
	rec, ok := resp.Payload.(map[string]any)
	if !ok {
		return id
	}
	var parts []string
	for _, f := range listableFields(oc.Def) {
		if v := strings.TrimSpace(fmt.Sprint(rec[f.Key])); v != "" && rec[f.Key] != nil {
			parts = append(parts, v)
		}
		if len(parts) == 3 {
			break
		}
	}
	if len(parts) == 0 {
		return id
	}
	return strings.Join(parts, " · ")
}
