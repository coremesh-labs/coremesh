package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// server übersetzt HTTP-Anfragen in (object, action)-Aufrufe. Er registriert
// nur Module (siehe modules.go); jedes Modul hat eigene Sub-Router:
//
//	GET    /                                 –            → Startseite: alle Module
//	GET    /m/{module}                       Kind list    → Übersicht des ersten Objects
//	GET    /m/{module}/{object}              Kind list    → Tabelle
//	GET    /m/{module}/{object}/new          –            → leeres Formular (create)
//	POST   /m/{module}/{object}              Kind create  → neue Zeile + Toast
//	GET    /m/{module}/{object}/{id}         Kind item    → Detailansicht
//	GET    /m/{module}/{object}/{id}/edit    Kind item    → Formular mit Werten (update)
//	PUT    /m/{module}/{object}/{id}         Kind update  → Zeile oder Detail + Toast
//	DELETE /m/{module}/{object}/{id}         Kind delete  → Zeile entfernen + Toast
//	POST   /m/{module}/{object}/{id}         _method=PUT|DELETE (Formulare ohne JavaScript)
//	GET    /action/{module}/{object}/{name}  Kind custom  → Formular der Action
//	POST   /action/{module}/{object}/{name}  Kind custom  → Ergebnis + Toast
//	GET    /api/v1/{module}                  –            → JSON: Objects und Actions
//	POST   /api/v1/{module}/{object}/{action} beliebig    → JSON: Payload der Action
type server struct {
	host    sdk.Host
	views   *renderer
	cfg     settings
	auth    *authService
	i18n    *translationService
	mux     *http.ServeMux
	handler http.Handler
}

func newServer(host sdk.Host, views *renderer, cfg settings, auth *authService) *server {
	tr, err := newTranslationService(host, cfg.DefaultLocale)
	if err != nil {
		panic(err) // eingebettete Dateien – Fehler nur beim Entwickeln
	}
	s := &server{host: host, views: views, cfg: cfg, auth: auth, i18n: tr, mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /login", s.loginForm)
	s.mux.HandleFunc("POST /login", s.login)
	s.mux.HandleFunc("POST /logout", s.logout)
	s.mux.HandleFunc("GET /account/password", s.passwordForm)
	s.mux.HandleFunc("POST /account/password", s.changePassword)
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", staticHandler(cfg.StaticDir)))
	s.mux.HandleFunc("GET /{$}", s.home)
	// Nur Module: je Modul ein gekapselter Sub-Router für Oberfläche, Actions und API.
	s.mux.HandleFunc("GET /lookup", s.lookup)
	s.mux.HandleFunc("GET /peek/{object}/{id}", s.peek)
	// Mehrsprachigkeit: Sprachwähler, Wörterbuch, eigenes Profil (siehe i18n.go).
	s.mux.HandleFunc("POST /locale", s.setLocale)
	// TagEditor (Abschnitt „Tags“, Plugin tag): siehe tags.go.
	// Dokumente (Abschnitt „Dokumente“, docservice): siehe docs.go.
	s.mux.HandleFunc("GET /docs/{entity}/{id}", s.docSectionGet)
	s.mux.HandleFunc("POST /docs/{entity}/{id}", s.docSectionAttach)
	s.mux.HandleFunc("POST /docs/{entity}/{id}/{doc}/remove", s.docSectionRemove)
	s.mux.HandleFunc("GET /tags/{entity}/{id}", s.tagEditorGet)
	s.mux.HandleFunc("POST /tags/{entity}/{id}", s.tagEditorPost)
	s.mux.HandleFunc("POST /tags/{entity}/{id}/preview", s.tagEditorPost)
	s.mux.HandleFunc("GET /api/v1/i18n/{lang}", s.apiTranslations)
	s.mux.HandleFunc("GET /api/v1/user/profile", s.apiProfile)
	s.mux.HandleFunc("PATCH /api/v1/user/profile", s.apiProfile)
	s.mountModule("/m", s.uiRoutes(), s.fail)
	s.mountModule("/action", s.actionRoutes(), s.fail)
	s.mountModule("/api/v1", s.apiRoutes(), s.apiError)
	// Reihenfolge: Schutz-Header → CSRF-Prüfung → Anmeldung → Routen.
	s.handler = securityHeaders(checkOrigin(s.requireAuth(s.localeMiddleware(s.mux))))
	return s
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.handler.ServeHTTP(w, r) }

// --- Aufruf der Fachmodule -------------------------------------------------

// call startet für jede HTTP-Anfrage eine eigene Wurzelanfrage im Host
// (Ingress). Abbruch durch den Browser bricht über r.Context() die ganze
// Aufrufkette ab.
func (s *server) call(r *http.Request, object, action string, payload any) (sdk.Response, error) {
	ctx := sdk.WithCall(r.Context(), s.callContext(r))
	return s.host.Handle(ctx, sdk.Request{Object: object, Action: action, Payload: payload})
}

func (s *server) callContext(r *http.Request) sdk.CallContext {
	md := map[string]string{"ingress": name}
	if lang := r.Header.Get("Accept-Language"); lang != "" {
		md["locale"] = strings.TrimSpace(strings.SplitN(strings.SplitN(lang, ",", 2)[0], ";", 2)[0])
	}
	call := sdk.CallContext{RequestID: newID(), TenantID: s.cfg.Tenant, Metadata: md}
	// Angemeldeter Benutzer: Identität und Mandant aus der Benutzertabelle.
	if u := userFrom(r); u != nil {
		call.UserID = u.ID
		md["username"] = u.Username
		if u.TenantID != "" {
			call.TenantID = u.TenantID
		}
	}
	return call
}

var objectRe = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)

// loadObject holt das Metamodell des Objects aus dem Catalog. Das Object
// muss zum Modul der Anfrage gehören (Kapselung: kein Zugriff auf Objects
// anderer Module über diesen Namensraum).
func (s *server) loadObject(r *http.Request) (objectCtx, error) {
	object := r.PathValue("object")
	mod := moduleFrom(r)
	if !objectRe.MatchString(object) {
		return objectCtx{}, fmt.Errorf("%w: Object %q", sdk.ErrNotFound, object)
	}
	if _, ok := mod.object(object); !ok {
		return objectCtx{}, fmt.Errorf("%w: %s", sdk.ErrNotFound, s.T(r, "core.error.not_in_module", object, mod.Title))
	}
	resp, err := s.call(r, sdk.ObjectCatalog, "GetDefinition", map[string]any{"object": object})
	if err != nil {
		return objectCtx{}, err
	}
	var def struct {
		Definition metamodel.ObjectDefinition `json:"definition"`
		Available  bool                       `json:"available"`
	}
	if err := sdk.Decode(resp.Payload, &def); err != nil {
		return objectCtx{}, err
	}
	if !def.Available {
		return objectCtx{}, fmt.Errorf("%w: %s", sdk.ErrUnavailable, s.T(r, "core.error.unavailable", def.Definition.Title))
	}
	return s.withDisplay(r, s.withLocale(r, newObjectCtx(mod.Name, object, s.localizeDef(r, def.Definition)).visibleFor(userFrom(r)))), nil
}

// need liefert die Action eines Kinds oder einen Fehler, wenn das Object sie nicht anbietet.
func need(oc objectCtx, kind metamodel.ActionKind) (*metamodel.ActionConfig, error) {
	if a := oc.Has[string(kind)]; a != nil {
		return a, nil
	}
	if oc.Denied[string(kind)] {
		return nil, fmt.Errorf("%w: %s", sdk.ErrPermissionDenied, oc.T("core.error.no_permission", oc.Def.Title, oc.T("core.action."+string(kind))))
	}
	return nil, fmt.Errorf("%w: %s bietet keine Aktion vom Typ %s an", sdk.ErrUnimplemented, oc.Def.Title, kind)
}

// --- Rendering ---------------------------------------------------------------

// isHTMX: HTMX-Anfrage (Fragment) oder Seitenaufruf (Layout + Fragment).
// History-Restores von HTMX brauchen die ganze Seite.
func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-History-Restore-Request") != "true"
}

// render schreibt bei HTMX-Anfragen nur das Fragment, sonst die ganze Seite.
func (s *server) render(w http.ResponseWriter, r *http.Request, status int, fragment string, data any, title, active string) {
	w.Header().Add("Vary", "HX-Request")
	var buf bytes.Buffer
	var err error
	if isHTMX(r) {
		err = s.views.fragment(localeFrom(r), &buf, fragment, data)
	} else {
		err = s.views.page(localeFrom(r), &buf, fragment, data, pageData{AppTitle: s.cfg.Title, Title: title, Active: active, Nav: s.nav(r, active), User: userFrom(r), Path: originalURI(r)})
	}
	if err != nil {
		s.logError(r, "Template "+fragment, err)
		http.Error(w, s.T(r, "core.app.render_error"), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	w.Write(buf.Bytes())
}

// fail zeigt einen Fehler: bei HTMX als Toast (HX-Retarget), sonst als Seite.
func (s *server) fail(w http.ResponseWriter, r *http.Request, err error) {
	status := errStatus(err)
	if status >= 500 {
		s.logError(r, "Anfrage", err)
	}
	msg := err.Error()
	if status == http.StatusInternalServerError {
		msg = s.T(r, "core.app.internal_error")
	}
	if isHTMX(r) {
		w.Header().Set("HX-Retarget", "#toast-container")
		w.Header().Set("HX-Reswap", "beforeend")
		s.render(w, r, status, "toast", &toast{Level: "error", Message: msg}, "", "")
		return
	}
	s.render(w, r, status, "error", map[string]any{"Status": status, "Message": msg}, s.T(r, "core.app.error"), "")
}

func errStatus(err error) int {
	switch {
	case errors.Is(err, errUnsupportedMedia):
		return http.StatusUnsupportedMediaType
	case errors.Is(err, errMethodNotAllowed):
		return http.StatusMethodNotAllowed
	case errors.Is(err, sdk.ErrNotFound), errors.Is(err, sdk.ErrUnimplemented):
		return http.StatusNotFound
	case errors.Is(err, sdk.ErrInvalidArgument):
		return http.StatusUnprocessableEntity
	case errors.Is(err, sdk.ErrPermissionDenied):
		return http.StatusForbidden
	case errors.Is(err, sdk.ErrAlreadyExists), errors.Is(err, sdk.ErrFailedPrecondition):
		return http.StatusConflict
	case errors.Is(err, sdk.ErrUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, context.Canceled):
		return 499 // Client hat abgebrochen
	}
	return http.StatusInternalServerError
}

func (s *server) logError(r *http.Request, what string, err error) {
	ctx := sdk.WithCall(context.WithoutCancel(r.Context()), s.callContext(r))
	_ = s.host.Log(ctx, sdk.LogError, what+" fehlgeschlagen", map[string]string{"err": err.Error(), "path": r.URL.Path})
}

// --- Handler -----------------------------------------------------------------

func (s *server) home(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, "home", map[string]any{"Modules": s.modules(r), "AppTitle": s.cfg.Title}, "", "")
}

// GET /m/{module}/{object}
func (s *server) list(w http.ResponseWriter, r *http.Request) {
	oc, err := s.loadObject(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	act, err := need(oc, metamodel.KindList)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	resp, err := s.call(r, oc.Object, act.Name, map[string]any{"query": queryParams(r.URL.Query())})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	rows, err := records(resp.Payload)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	filterBar := s.filterFields(r, oc, r.URL.Query())
	if len(rows) > 0 {
		for _, row := range rows {
			oc.applyListDisplay(row)
		}
		oc.Def = withoutFields(oc.Def, hiddenEverywhere(rows)) // Spalten ohne Leserecht in allen Zeilen
	}
	v := view{objectCtx: oc, Rows: rows, History: includeHistory(r), Filter: map[string]string{}, FilterBar: filterBar}
	q := url.Values{}
	for _, k := range append([]string{"q"}, oc.Def.Filters...) {
		if val := strings.TrimSpace(r.URL.Query().Get(k)); val != "" {
			v.Filter[k] = val
			q.Set(k, val)
		}
	}
	if v.History {
		q.Set("includeHistory", "true")
	}
	v.ListURL = oc.URL
	if len(q) > 0 {
		v.ListURL += "?" + q.Encode()
	}
	s.render(w, r, http.StatusOK, "list", v, oc.Def.Title, oc.Object)
}

// GET /m/{module}/{object}/new
func (s *server) newForm(w http.ResponseWriter, r *http.Request) {
	oc, err := s.loadObject(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if _, err := need(oc, metamodel.KindCreate); err != nil {
		s.fail(w, r, err)
		return
	}
	v := s.createView(r, oc, nil, nil)
	s.render(w, r, http.StatusOK, "form", v, v.FormTitle, oc.Object)
}

// createView baut das Formular für create. Vorbelegung über die URL
// (?<feld>=<wert>), feste Felder über _lock, _view=refresh für die
// Master-Detail-Ansicht (Antwort: Dialog zu, Abschnitte laden neu).
func (s *server) createView(r *http.Request, oc objectCtx, values, errs map[string]string) view {
	if values == nil {
		values = map[string]string{}
		for _, f := range oc.Def.Fields {
			if v := r.URL.Query().Get(f.Key); v != "" {
				values[f.Key] = v
			}
		}
	}
	locked, lockList := lockedFields(oc.Def, r.FormValue("_lock"))
	m := s.mask(r, oc, "create", "", values, locked, s.createAccess(r, oc.Def))
	v := view{
		objectCtx: oc, Mode: "create", Modal: isHTMX(r), ViewParam: refreshParam(r), Locked: lockList,
		FormTitle: s.T(r, "core.form.title", oc.Def.Title, oc.Has["create"].Label), FormAction: oc.URL,
		CancelURL: oc.URL, FormMessage: m.message, FormStateURL: formStateURL(oc),
		FormFields: buildFields(m.def, "create", m.values, errs, formOpts{locked: locked,
			readonly: m.readonly,
			labels:   s.lookupLabels(r, m.def, m.values, nil), trigger: formStateURL(oc)}),
	}
	return v
}

// refreshParam: "refresh", wenn das Formular aus einem eingebetteten
// Abschnitt (Master-Detail) kommt, sonst "".
func refreshParam(r *http.Request) string {
	if r.FormValue("_view") == "refresh" || r.URL.Query().Get("view") == "refresh" {
		return "refresh"
	}
	return ""
}

// refreshed beantwortet ein Formular aus der Master-Detail-Ansicht: Der
// Dialog wird geleert (leerer Haupt-Swap in #modal), ein Toast erscheint, und
// das Ereignis coremesh-changed lädt die eingebetteten Abschnitte neu.
func (s *server) refreshed(w http.ResponseWriter, r *http.Request, msg string) {
	w.Header().Set("HX-Trigger", "coremesh-changed")
	s.render(w, r, http.StatusOK, "toast", &toast{Level: "success", Message: msg}, "", "")
}

// POST /m/{module}/{object}
func (s *server) create(w http.ResponseWriter, r *http.Request) {
	oc, err := s.loadObject(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	act, err := need(oc, metamodel.KindCreate)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.fail(w, r, fmt.Errorf("%w: %v", sdk.ErrInvalidArgument, err))
		return
	}
	data, raw, errs := s.parseMasked(r, oc, "create", "")
	if len(errs) > 0 {
		s.formAgain(w, r, s.createView(r, oc, raw, errs), "")
		return
	}
	resp, err := s.call(r, oc.Object, act.Name, map[string]any{"data": data})
	if err != nil {
		if errors.Is(err, sdk.ErrInvalidArgument) {
			s.formAgain(w, r, s.createView(r, oc, raw, nil), err.Error())
			return
		}
		s.fail(w, r, err)
		return
	}
	if !isHTMX(r) {
		http.Redirect(w, r, oc.URL, http.StatusSeeOther)
		return
	}
	if refreshParam(r) == "refresh" {
		s.refreshed(w, r, s.T(r, "core.toast.created", oc.Def.Title))
		return
	}
	v := view{objectCtx: oc, Record: asRecord(resp.Payload), Toast: &toast{Level: "success", Message: s.T(r, "core.toast.created", oc.Def.Title)}}
	s.render(w, r, http.StatusOK, "created", v, "", "")
}

// formAgain zeigt ein Formular mit Fehlern erneut (422). Bei HTMX wird das
// Ziel auf den Dialog umgelenkt, statt z. B. an die Tabelle anzuhängen.
func (s *server) formAgain(w http.ResponseWriter, r *http.Request, v view, formError string) {
	v.FormError = formError
	if isHTMX(r) {
		w.Header().Set("HX-Retarget", "#modal")
		w.Header().Set("HX-Reswap", "innerHTML")
	}
	s.render(w, r, http.StatusUnprocessableEntity, "form", v, v.FormTitle, v.Object)
}

// GET /m/{module}/{object}/{id}
func (s *server) item(w http.ResponseWriter, r *http.Request) {
	oc, rec, err := s.loadItem(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	oc.applyDisplay(rec)
	oc.Def = withoutFields(oc.Def, accessOf(rec).hidden)
	s.render(w, r, http.StatusOK, "detail", view{objectCtx: oc, Record: rec}, oc.Def.Title, oc.Object)
}

func (s *server) loadItem(r *http.Request) (objectCtx, record, error) {
	oc, err := s.loadObject(r)
	if err != nil {
		return oc, nil, err
	}
	act, err := need(oc, metamodel.KindItem)
	if err != nil {
		return oc, nil, err
	}
	resp, err := s.call(r, oc.Object, act.Name, map[string]any{"id": r.PathValue("id")})
	if err != nil {
		return oc, nil, err
	}
	rec := asRecord(resp.Payload)
	if recordID(rec) == "" {
		rec["id"] = r.PathValue("id")
	}
	return oc, rec, nil
}

// GET /m/{module}/{object}/{id}/edit
func (s *server) editForm(w http.ResponseWriter, r *http.Request) {
	oc, rec, err := s.loadItem(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if _, err := need(oc, metamodel.KindUpdate); err != nil {
		s.fail(w, r, err)
		return
	}
	values := map[string]string{}
	for _, f := range oc.Def.Fields {
		values[f.Key] = formValue(rec, f)
	}
	v := s.editView(r, oc, rec, values, nil, r.URL.Query().Get("view"))
	s.render(w, r, http.StatusOK, "form", v, v.FormTitle, oc.Object)
}

// editView: viewParam row (Tabelle), detail (Detailansicht) oder refresh
// (eingebetteter Abschnitt einer Master-Detail-Ansicht).
func (s *server) editView(r *http.Request, oc objectCtx, rec record, values, errs map[string]string, viewParam string) view {
	if viewParam != "row" && viewParam != "refresh" {
		viewParam = "detail"
	}
	id := recordID(rec)
	target := "#detail"
	switch viewParam {
	case "row":
		target = "#row-" + hex.EncodeToString([]byte(id))
	case "refresh":
		target = "#modal"
	}
	locked, lockList := lockedFields(oc.Def, r.FormValue("_lock"))
	acc := accessOf(rec)
	if acc.empty() {
		acc = decodeAccess(r.FormValue("_access")) // Neuauswertung, erneutes Anzeigen
	}
	m := s.mask(r, oc, "edit", id, values, locked, acc)
	return view{
		Access:    acc.encode(),
		objectCtx: oc, Record: rec, Mode: "edit", Modal: isHTMX(r), ViewParam: viewParam, Target: target, Locked: lockList,
		FormTitle:  s.T(r, "core.form.title", oc.Def.Title, oc.Has["update"].Label),
		FormAction: oc.URL + "/" + pathEscape(id), CancelURL: oc.URL + "/" + pathEscape(id),
		FormMessage: m.message, FormStateURL: formStateURL(oc),
		FormFields: buildFields(m.def, "edit", m.values, errs, formOpts{locked: locked,
			readonly: m.readonly,
			labels:   s.lookupLabels(r, m.def, m.values, labelsOf(rec)), trigger: formStateURL(oc)}),
	}
}

// PUT /m/{module}/{object}/{id}
func (s *server) update(w http.ResponseWriter, r *http.Request) {
	oc, err := s.loadObject(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	act, err := need(oc, metamodel.KindUpdate)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.fail(w, r, fmt.Errorf("%w: %v", sdk.ErrInvalidArgument, err))
		return
	}
	id := r.PathValue("id")
	data, raw, errs := s.parseMasked(r, oc, "edit", id)
	if len(errs) > 0 {
		s.formAgain(w, r, s.editView(r, oc, record{"id": id}, raw, errs, r.PostForm.Get("_view")), "")
		return
	}
	resp, err := s.call(r, oc.Object, act.Name, map[string]any{"id": id, "data": data})
	if err != nil {
		if errors.Is(err, sdk.ErrInvalidArgument) {
			s.formAgain(w, r, s.editView(r, oc, record{"id": id}, raw, nil, r.PostForm.Get("_view")), err.Error())
			return
		}
		s.fail(w, r, err)
		return
	}
	if !isHTMX(r) {
		http.Redirect(w, r, oc.URL+"/"+pathEscape(id), http.StatusSeeOther)
		return
	}
	rec := asRecord(resp.Payload)
	if recordID(rec) == "" {
		rec["id"] = id
	}
	viewParam := r.PostForm.Get("_view")
	if viewParam == "refresh" {
		s.refreshed(w, r, s.T(r, "core.toast.saved", oc.Def.Title))
		return
	}
	if viewParam != "row" {
		viewParam = "detail"
	}
	oc = oc.forRow(rec, viewParam)
	v := view{objectCtx: oc, Record: rec, ViewParam: viewParam, Toast: &toast{Level: "success", Message: s.T(r, "core.toast.saved", oc.Def.Title)}}
	s.render(w, r, http.StatusOK, "updated", v, "", "")
}

// DELETE /m/{module}/{object}/{id}: Physisches Löschen gibt es nicht – immer
// 405. Die Meldung nennt den Weg, den der Lebenszyklus des Objects vorsieht
// (siehe lifecycle.go).
func (s *server) delete(w http.ResponseWriter, r *http.Request) {
	oc, err := s.loadObject(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Allow", "GET, PUT, POST")
	s.fail(w, r, deleteNotAllowed(oc))
}

// POST /m/{module}/{object}/{id} mit _method=PUT|DELETE – für Formulare ohne JavaScript.
func (s *server) methodOverride(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.fail(w, r, fmt.Errorf("%w: %v", sdk.ErrInvalidArgument, err))
		return
	}
	switch strings.ToUpper(r.PostForm.Get("_method")) {
	case http.MethodPut:
		s.update(w, r)
	case http.MethodDelete:
		s.delete(w, r)
	default:
		http.Error(w, "_method muss PUT oder DELETE sein", http.StatusMethodNotAllowed)
	}
}

// GET /action/{module}/{object}/{name}
func (s *server) actionForm(w http.ResponseWriter, r *http.Request) {
	oc, act, err := s.loadAction(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	v := s.actionView(r, oc, act, r.URL.Query().Get("id"), nil, nil)
	s.render(w, r, http.StatusOK, "form", v, v.FormTitle, oc.Object)
}

func (s *server) loadAction(r *http.Request) (objectCtx, *metamodel.ActionConfig, error) {
	oc, err := s.loadObject(r)
	if err != nil {
		return oc, nil, err
	}
	act := oc.custom(r.PathValue("name"))
	if act == nil {
		if oc.Denied["custom:"+r.PathValue("name")] {
			return oc, nil, fmt.Errorf("%w: %s", sdk.ErrPermissionDenied, s.T(r, "core.error.no_permission_action", oc.Object, r.PathValue("name")))
		}
		return oc, nil, fmt.Errorf("%w: %s bietet die Aktion %q nicht an", sdk.ErrUnimplemented, oc.Def.Title, r.PathValue("name"))
	}
	return oc, act, nil
}

func (s *server) actionView(r *http.Request, oc objectCtx, act *metamodel.ActionConfig, id string, values, errs map[string]string) view {
	var message string
	if values == nil {
		values, message = s.actionDefaults(r, oc, act, id)
	}
	return view{
		objectCtx: oc, Mode: "action", Modal: isHTMX(r), Action: *act, ActionID: id,
		FormTitle: s.T(r, "core.form.title", oc.Def.Title, act.Label), FormAction: oc.ActionURL + "/" + act.Name,
		CancelURL: oc.URL, FormMessage: message,
		FormFields: actionFields(buildFields(oc.ActionDef, "action", values, errs, formOpts{}), act.Fields),
	}
}

// actionDefaults: Vorbelegung und Hinweis eines Aktionsformulars beim Öffnen
// (ActionConfig.FormState) – aus der FormState-Action des Objects.
func (s *server) actionDefaults(r *http.Request, oc objectCtx, act *metamodel.ActionConfig, id string) (map[string]string, string) {
	if !act.FormState || oc.Def.FormState == "" {
		return nil, ""
	}
	req := metamodel.FormStateRequest{Mode: "action", Action: act.Name, ID: id, Values: map[string]string{}}
	resp, err := s.call(r, oc.Object, oc.Def.FormState, req)
	if err != nil {
		s.logError(r, "FormState", err)
		return nil, ""
	}
	var st metamodel.FormState
	if err := sdk.Decode(resp.Payload, &st); err != nil {
		return nil, ""
	}
	values := map[string]string{}
	for k, fs := range st.Fields {
		if fs.Value != nil {
			values[k] = *fs.Value
		}
	}
	return values, st.Message
}

// POST /action/{module}/{object}/{name}
func (s *server) runAction(w http.ResponseWriter, r *http.Request) {
	oc, act, err := s.loadAction(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.fail(w, r, fmt.Errorf("%w: %v", sdk.ErrInvalidArgument, err))
		return
	}
	id := r.PostForm.Get("_id")
	// Aktion ohne Formular (Confirm ohne Fields): nur bestätigt, keine Eingaben.
	direct := act.Confirm != "" && len(act.Fields) == 0
	data, raw, errs := map[string]any{}, map[string]string{}, map[string]string{}
	if !direct {
		data, raw, errs = parseFields(actionDef(oc.ActionDef, act.Fields), r.PostForm)
	}
	if len(errs) > 0 {
		s.formAgain(w, r, s.actionView(r, oc, act, id, raw, errs), "")
		return
	}
	payload := map[string]any{"data": data}
	if id != "" {
		payload["id"] = id
	}
	resp, err := s.call(r, oc.Object, act.Name, payload)
	if err != nil {
		if errors.Is(err, sdk.ErrInvalidArgument) && !direct {
			s.formAgain(w, r, s.actionView(r, oc, act, id, raw, nil), err.Error())
			return
		}
		s.fail(w, r, err)
		return
	}
	v := view{objectCtx: oc, Action: *act, Result: resp.Payload, Modal: isHTMX(r),
		Toast: &toast{Level: "success", Message: s.T(r, "core.toast.executed", act.Label)}}
	if m, ok := resp.Payload.(map[string]any); ok {
		v.Message, _ = m["message"].(string)
	}
	v.ResultTable = tableOf(resp.Payload)
	// Übersicht neu laden: Die Liste hört auf dieses Ereignis.
	w.Header().Set("HX-Trigger", "coremesh-changed")
	s.render(w, r, http.StatusOK, "result", v, act.Label, oc.Object)
}

// --- Hilfsfunktionen ---------------------------------------------------------

func pathEscape(s string) string { return url.PathEscape(s) }

func newID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// staticHandler liefert /static/: zuerst aus staticDir (falls gesetzt), sonst
// die eingebetteten Dateien.
func staticHandler(staticDir string) http.Handler {
	sub, _ := fs.Sub(embeddedStatic, "static")
	embedded := http.FileServerFS(sub)
	if staticDir == "" {
		return embedded
	}
	local := http.FileServer(http.Dir(staticDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fi, err := os.Stat(filepath.Join(staticDir, filepath.FromSlash(r.URL.Path))); err == nil && !fi.IsDir() {
			local.ServeHTTP(w, r)
			return
		}
		embedded.ServeHTTP(w, r)
	})
}
