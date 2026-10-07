package main

import (
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/camel/coremesh/pkg/sdk"
	"github.com/camel/coremesh/pkg/sdk/metamodel"
)

// Lebenszyklus statt Löschen.
//
// Physisch gelöscht wird nichts. Wie ein Datensatz endet, steht im
// Metamodell (ObjectDefinition.Lifecycle):
//
//	timeslice → Button „Beenden …“: Dialog mit Datumswähler, Action vom Kind expire {id, valid_to}
//	status    → Button „Inaktivieren“: Bestätigungsdialog, Action vom Kind deactivate {id}
//	immutable → kein Button; DELETE und /end antworten mit 405
//
//	GET  /m/{module}/{object}/{id}/end?view=row|detail|refresh   Dialog
//	POST /m/{module}/{object}/{id}/end                           ausführen
//
// Die Antwort richtet sich nach dem Ort des Buttons (view): Tabellenzeile,
// Detailansicht oder eingebetteter Abschnitt (Master-Detail).

var errMethodNotAllowed = errors.New("method not allowed")

// deleteNotAllowed erklärt, warum DELETE abgelehnt wird und was stattdessen gilt.
func deleteNotAllowed(oc objectCtx) error {
	switch oc.Def.Lifecycle.Kind() {
	case metamodel.LifecycleTimeSlice:
		return fmt.Errorf("%w: %s", errMethodNotAllowed, oc.T("core.lifecycle.timeslice", oc.Def.Title))
	case metamodel.LifecycleStatus:
		return fmt.Errorf("%w: %s", errMethodNotAllowed, oc.T("core.lifecycle.status", oc.Def.Title))
	}
	return fmt.Errorf("%w: %s", errMethodNotAllowed, oc.T("core.lifecycle.immutable", oc.Def.Title))
}

// endAction liefert die Ende-Action des Objects oder einen Fehler (405 bei
// immutable, 403 ohne Berechtigung).
func endAction(oc objectCtx) (*metamodel.ActionConfig, error) {
	kind := oc.Def.Lifecycle.EndAction()
	if kind == "" {
		return nil, deleteNotAllowed(oc)
	}
	if a := oc.Has[string(kind)]; a != nil {
		return a, nil
	}
	if oc.Denied[string(kind)] {
		return nil, fmt.Errorf("%w: keine Berechtigung, %s zu beenden", sdk.ErrPermissionDenied, oc.Def.Title)
	}
	return nil, fmt.Errorf("%w: %s bietet keine Action vom Kind %s an", sdk.ErrUnimplemented, oc.Def.Title, kind)
}

// endBtn sind die Daten des Blocks end-button.
type endBtn struct {
	URL, ID, View, Class string
	Action               *metamodel.ActionConfig // nil = kein Button (immutable oder keine Berechtigung)
	Inactive             bool                    // status: bereits inaktiv
}

func newEndBtn(oc objectCtx, rec record, view, class string) endBtn {
	b := endBtn{URL: oc.URL, ID: recordID(rec), View: view, Class: class}
	if kind := oc.Def.Lifecycle.EndAction(); kind != "" && !locked(rec) {
		b.Action = oc.Has[string(kind)]
	}
	if l := oc.Def.Lifecycle; l.Kind() == metamodel.LifecycleStatus {
		// Boolean: false = inaktiv; Auswahlfeld: InactiveValue (z. B. DEPRECATED).
		v := rec[l.StatusField]
		b.Inactive = v == false || (l.InactiveValue != "" && scalar(v) == l.InactiveValue)
	}
	return b
}

func (s *server) endView(r *http.Request, oc objectCtx, rec record, act *metamodel.ActionConfig, viewParam string) view {
	if viewParam != "row" && viewParam != "refresh" {
		viewParam = "detail"
	}
	id := recordID(rec)
	target := map[string]string{"row": "#row-" + domID(id), "detail": "#detail", "refresh": "#modal"}[viewParam]
	v := view{
		objectCtx: oc, Record: rec, Mode: "end", Modal: isHTMX(r), ViewParam: viewParam, Target: target,
		Action: *act, EndKind: string(oc.Def.Lifecycle.Kind()),
		FormAction: oc.URL + "/" + pathEscape(id) + "/end", CancelURL: oc.URL + "/" + pathEscape(id),
	}
	title := oc.Def.Title
	if t := v.RecordTitle(); t != "" {
		title += " " + t
	}
	if v.EndKind == string(metamodel.LifecycleTimeSlice) {
		v.FormTitle = s.T(r, "core.end.title_timeslice", title)
		v.EndMin = scalar(rec[oc.Def.Lifecycle.ValidFrom])
	} else {
		v.FormTitle = s.T(r, "core.end.title_status", title)
	}
	return v
}

// GET /m/{module}/{object}/{id}/end
func (s *server) endForm(w http.ResponseWriter, r *http.Request) {
	oc, rec, err := s.loadItem(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	act, err := endAction(oc)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	v := s.endView(r, oc, rec, act, r.URL.Query().Get("view"))
	s.render(w, r, http.StatusOK, "end", v, v.FormTitle, oc.Object)
}

// POST /m/{module}/{object}/{id}/end
func (s *server) end(w http.ResponseWriter, r *http.Request) {
	oc, rec, err := s.loadItem(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	act, err := endAction(oc)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.fail(w, r, fmt.Errorf("%w: %v", sdk.ErrInvalidArgument, err))
		return
	}
	id := recordID(rec)
	v := s.endView(r, oc, rec, act, r.PostForm.Get("_view"))
	payload := map[string]any{"id": id}
	msg := s.T(r, "core.toast.deactivated", oc.Def.Title)
	if v.EndKind == string(metamodel.LifecycleTimeSlice) {
		v.EndDate = r.PostForm.Get("valid_to")
		if v.EndDate == "" {
			s.endAgain(w, r, v, s.T(r, "core.end.date_missing"))
			return
		}
		payload["valid_to"] = v.EndDate
		msg = s.T(r, "core.toast.expired", oc.Def.Title, v.EndDate)
	}
	resp, err := s.call(r, oc.Object, act.Name, payload)
	if err != nil {
		if errors.Is(err, sdk.ErrInvalidArgument) {
			s.endAgain(w, r, v, err.Error())
			return
		}
		s.fail(w, r, err)
		return
	}
	switch {
	case !isHTMX(r):
		http.Redirect(w, r, oc.URL+"/"+pathEscape(id), http.StatusSeeOther)
	case v.ViewParam == "refresh":
		s.refreshed(w, r, msg)
	default:
		saved := asRecord(resp.Payload)
		if recordID(saved) == "" {
			saved = rec
		}
		out := view{objectCtx: oc, Record: saved, ViewParam: v.ViewParam, Toast: &toast{Level: "success", Message: msg}}
		s.render(w, r, http.StatusOK, "updated", out, "", "")
	}
}

// endAgain zeigt den Dialog mit Fehler erneut (422, bei HTMX in #modal).
func (s *server) endAgain(w http.ResponseWriter, r *http.Request, v view, formError string) {
	v.FormError = formError
	if isHTMX(r) {
		w.Header().Set("HX-Retarget", "#modal")
		w.Header().Set("HX-Reswap", "innerHTML")
	}
	s.render(w, r, http.StatusUnprocessableEntity, "end", v, v.FormTitle, v.Object)
}

// Ctx macht den objectCtx eines view in Templates zugänglich (endBtn).
func (v view) Ctx() objectCtx { return v.objectCtx }

// endActionName: Name der Ende-Action (für die Metadaten), "" bei immutable.
func endActionName(oc objectCtx) string {
	kind := oc.Def.Lifecycle.EndAction()
	for _, a := range oc.Def.Actions {
		if kind != "" && a.Kind == kind {
			return a.Name
		}
	}
	return ""
}

// HasHistory: Das Object hat Zeitscheibe oder Status-Flag – Listen zeigen
// standardmäßig nur gültige bzw. aktive Einträge und bieten den Schalter an.
func (oc objectCtx) HasHistory() bool {
	return oc.Def.Lifecycle.Kind() != metamodel.LifecycleImmutable
}

// includeHistory: ?includeHistory=true in der Anfrage.
func includeHistory(r *http.Request) bool {
	v := r.URL.Query().Get("includeHistory")
	return v == "true" || v == "1" || v == "on"
}

type historyCtx struct {
	URL, Target, Swap string
	On                bool
}

// locked: Das Modul hat den Datensatz als nicht mehr änderbar gekennzeichnet
// ("_locked": true, z. B. eine gebuchte Vorerfassung). Die Oberfläche blendet
// dann Bearbeiten, Beenden und Aktionen je Datensatz aus; prüfen muss weiterhin
// das Modul.
func locked(rec record) bool { b, _ := rec["_locked"].(bool); return b }

// recordActions: Aktionen je Datensatz ohne die, die das Modul für diesen
// Datensatz ausblendet ("_hidden_actions": ["unlock"], z. B. Entsperren bei
// einem aktiven Konto).
func recordActions(oc objectCtx, rec record) []metamodel.ActionConfig {
	hidden, _ := rec["_hidden_actions"].([]any)
	var out []metamodel.ActionConfig
	for _, a := range oc.RecordActions() {
		if !slices.ContainsFunc(hidden, func(h any) bool { return h == a.Name }) {
			out = append(out, a)
		}
	}
	return out
}
