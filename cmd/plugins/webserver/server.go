package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/camel/coremesh/pkg/sdk"
	"github.com/camel/coremesh/pkg/sdk/metamodel"
)

// server übersetzt HTTP-Anfragen in (object, action)-Aufrufe.
//
//	GET    /ui/{object}             Kind list    → Tabelle
//	GET    /ui/{object}/new         –            → leeres Formular (create)
//	POST   /ui/{object}             Kind create  → neue Zeile + Toast
//	GET    /ui/{object}/{id}        Kind item    → Detailansicht
//	GET    /ui/{object}/{id}/edit   Kind item    → Formular mit Werten (update)
//	PUT    /ui/{object}/{id}        Kind update  → Zeile oder Detail + Toast
//	DELETE /ui/{object}/{id}        Kind delete  → Zeile entfernen + Toast
//	POST   /ui/{object}/{id}        _method=PUT|DELETE (Formulare ohne JavaScript)
//	GET    /action/{object}/{name}  Kind custom  → Formular der Action
//	POST   /action/{object}/{name}  Kind custom  → Ergebnis + Toast
type server struct {
	host    sdk.Host
	views   *renderer
	cfg     settings
	auth    *authService
	mux     *http.ServeMux
	handler http.Handler
}

func newServer(host sdk.Host, views *renderer, cfg settings, auth *authService) *server {
	s := &server{host: host, views: views, cfg: cfg, auth: auth, mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /login", s.loginForm)
	s.mux.HandleFunc("POST /login", s.login)
	s.mux.HandleFunc("POST /logout", s.logout)
	s.mux.HandleFunc("GET /account/password", s.passwordForm)
	s.mux.HandleFunc("POST /account/password", s.changePassword)
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", staticHandler(cfg.StaticDir)))
	s.mux.HandleFunc("GET /{$}", s.home)
	s.mux.HandleFunc("GET /ui/{object}", s.list)
	s.mux.HandleFunc("GET /ui/{object}/new", s.newForm)
	s.mux.HandleFunc("POST /ui/{object}", s.create)
	s.mux.HandleFunc("GET /ui/{object}/{id}", s.item)
	s.mux.HandleFunc("GET /ui/{object}/{id}/edit", s.editForm)
	s.mux.HandleFunc("PUT /ui/{object}/{id}", s.update)
	s.mux.HandleFunc("DELETE /ui/{object}/{id}", s.delete)
	s.mux.HandleFunc("POST /ui/{object}/{id}", s.methodOverride)
	s.mux.HandleFunc("GET /action/{object}/{name}", s.actionForm)
	s.mux.HandleFunc("POST /action/{object}/{name}", s.runAction)
	// Reihenfolge: Schutz-Header → CSRF-Prüfung → Anmeldung → Routen.
	s.handler = securityHeaders(checkOrigin(s.requireAuth(s.mux)))
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

// loadObject holt das Metamodell des Objects aus dem Catalog.
func (s *server) loadObject(r *http.Request) (objectCtx, error) {
	object := r.PathValue("object")
	if !objectRe.MatchString(object) {
		return objectCtx{}, fmt.Errorf("%w: Object %q", sdk.ErrNotFound, object)
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
		return objectCtx{}, fmt.Errorf("%w: %s ist derzeit nicht verfügbar", sdk.ErrUnavailable, def.Definition.Title)
	}
	return newObjectCtx(object, def.Definition).visibleFor(userFrom(r)), nil
}

// need liefert die Action eines Kinds oder einen Fehler, wenn das Object sie nicht anbietet.
func need(oc objectCtx, kind metamodel.ActionKind) (*metamodel.ActionConfig, error) {
	if a := oc.Has[string(kind)]; a != nil {
		return a, nil
	}
	if oc.Denied[string(kind)] {
		return nil, fmt.Errorf("%w: keine Berechtigung für %s (%s)", sdk.ErrPermissionDenied, oc.Def.Title, kind)
	}
	return nil, fmt.Errorf("%w: %s bietet keine Aktion vom Typ %s an", sdk.ErrUnimplemented, oc.Def.Title, kind)
}

// nav liefert die Navigation aus Catalog.ListObjects: alle verfügbaren
// Objects mit Metamodell.
func (s *server) nav(r *http.Request) []navItem {
	resp, err := s.call(r, sdk.ObjectCatalog, "ListObjects", nil)
	if err != nil {
		s.logError(r, "Navigation", err)
		return nil
	}
	var list struct {
		Objects []struct {
			Object, Title, Icon string
			Defined, Available  bool
		} `json:"objects"`
	}
	_ = sdk.Decode(resp.Payload, &list)
	u := userFrom(r)
	var out []navItem
	for _, o := range list.Objects {
		// Nur Objects mit Metamodell, die gerade laufen und für die der
		// Benutzer mindestens eine Berechtigung hat.
		if o.Defined && o.Available && (u == nil || u.CanAny(o.Object)) {
			out = append(out, navItem{Object: o.Object, Title: o.Title, Icon: o.Icon})
		}
	}
	return out
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
		err = s.views.fragment(&buf, fragment, data)
	} else {
		err = s.views.page(&buf, fragment, data, pageData{AppTitle: s.cfg.Title, Title: title, Active: active, Nav: s.nav(r), User: userFrom(r)})
	}
	if err != nil {
		s.logError(r, "Template "+fragment, err)
		http.Error(w, "Darstellungsfehler", http.StatusInternalServerError)
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
		msg = "Interner Fehler"
	}
	if isHTMX(r) {
		w.Header().Set("HX-Retarget", "#toast-container")
		w.Header().Set("HX-Reswap", "beforeend")
		s.render(w, r, status, "toast", &toast{Level: "error", Message: msg}, "", "")
		return
	}
	s.render(w, r, status, "error", map[string]any{"Status": status, "Message": msg}, "Fehler", "")
}

func errStatus(err error) int {
	switch {
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
	s.render(w, r, http.StatusOK, "home", map[string]any{"Nav": s.nav(r), "AppTitle": s.cfg.Title}, "", "")
}

// GET /ui/{object}
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
	s.render(w, r, http.StatusOK, "list", view{objectCtx: oc, Rows: rows}, oc.Def.Title, oc.Object)
}

// GET /ui/{object}/new
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

func (s *server) createView(r *http.Request, oc objectCtx, values, errs map[string]string) view {
	return view{
		objectCtx: oc, Mode: "create", Modal: isHTMX(r),
		FormTitle: oc.Def.Title + " – " + oc.Has["create"].Label, FormAction: "/ui/" + oc.Object,
		CancelURL: "/ui/" + oc.Object, FormFields: buildFields(oc.Def, "create", values, errs),
	}
}

// POST /ui/{object}
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
	data, raw, errs := parseFields(oc.Def, r.PostForm)
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
		http.Redirect(w, r, "/ui/"+oc.Object, http.StatusSeeOther)
		return
	}
	v := view{objectCtx: oc, Record: asRecord(resp.Payload), Toast: &toast{Level: "success", Message: oc.Def.Title + " angelegt"}}
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

// GET /ui/{object}/{id}
func (s *server) item(w http.ResponseWriter, r *http.Request) {
	oc, rec, err := s.loadItem(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
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

// GET /ui/{object}/{id}/edit
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

func (s *server) editView(r *http.Request, oc objectCtx, rec record, values, errs map[string]string, viewParam string) view {
	if viewParam != "row" {
		viewParam = "detail"
	}
	id := recordID(rec)
	target := "#detail"
	if viewParam == "row" {
		target = "#row-" + hex.EncodeToString([]byte(id))
	}
	return view{
		objectCtx: oc, Record: rec, Mode: "edit", Modal: isHTMX(r), ViewParam: viewParam, Target: target,
		FormTitle:  oc.Def.Title + " – " + oc.Has["update"].Label,
		FormAction: "/ui/" + oc.Object + "/" + pathEscape(id), CancelURL: "/ui/" + oc.Object + "/" + pathEscape(id),
		FormFields: buildFields(oc.Def, "edit", values, errs),
	}
}

// PUT /ui/{object}/{id}
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
	data, raw, errs := parseFields(oc.Def, r.PostForm)
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
		http.Redirect(w, r, "/ui/"+oc.Object+"/"+pathEscape(id), http.StatusSeeOther)
		return
	}
	rec := asRecord(resp.Payload)
	if recordID(rec) == "" {
		rec["id"] = id
	}
	viewParam := r.PostForm.Get("_view")
	if viewParam != "row" {
		viewParam = "detail"
	}
	v := view{objectCtx: oc, Record: rec, ViewParam: viewParam, Toast: &toast{Level: "success", Message: oc.Def.Title + " gespeichert"}}
	s.render(w, r, http.StatusOK, "updated", v, "", "")
}

// DELETE /ui/{object}/{id}
func (s *server) delete(w http.ResponseWriter, r *http.Request) {
	oc, err := s.loadObject(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	act, err := need(oc, metamodel.KindDelete)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if _, err := s.call(r, oc.Object, act.Name, map[string]any{"id": r.PathValue("id")}); err != nil {
		s.fail(w, r, err)
		return
	}
	switch {
	case !isHTMX(r):
		http.Redirect(w, r, "/ui/"+oc.Object, http.StatusSeeOther)
	case r.FormValue("_view") == "detail":
		// Aus der Detailansicht: zurück zur Übersicht.
		loc, _ := json.Marshal(map[string]string{"path": "/ui/" + oc.Object, "target": "#main-content"})
		w.Header().Set("HX-Location", string(loc))
		w.WriteHeader(http.StatusOK)
	default:
		// Aus der Tabelle: Zeile durch "nichts" ersetzen + Toast.
		s.render(w, r, http.StatusOK, "toast", &toast{Level: "success", Message: oc.Def.Title + " gelöscht"}, "", "")
	}
}

// POST /ui/{object}/{id} mit _method=PUT|DELETE – für Formulare ohne JavaScript.
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

// GET /action/{object}/{name}
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
			return oc, nil, fmt.Errorf("%w: keine Berechtigung für %s.%s", sdk.ErrPermissionDenied, oc.Object, r.PathValue("name"))
		}
		return oc, nil, fmt.Errorf("%w: %s bietet die Aktion %q nicht an", sdk.ErrUnimplemented, oc.Def.Title, r.PathValue("name"))
	}
	return oc, act, nil
}

func (s *server) actionView(r *http.Request, oc objectCtx, act *metamodel.ActionConfig, id string, values, errs map[string]string) view {
	return view{
		objectCtx: oc, Mode: "action", Modal: isHTMX(r), Action: *act, ActionID: id,
		FormTitle: oc.Def.Title + " – " + act.Label, FormAction: "/action/" + oc.Object + "/" + act.Name,
		CancelURL: "/ui/" + oc.Object, FormFields: buildFields(oc.Def, "action", values, errs),
	}
}

// POST /action/{object}/{name}
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
	data, raw, errs := parseFields(oc.Def, r.PostForm)
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
		if errors.Is(err, sdk.ErrInvalidArgument) {
			s.formAgain(w, r, s.actionView(r, oc, act, id, raw, nil), err.Error())
			return
		}
		s.fail(w, r, err)
		return
	}
	v := view{objectCtx: oc, Action: *act, Result: resp.Payload, Modal: isHTMX(r),
		Toast: &toast{Level: "success", Message: act.Label + " ausgeführt"}}
	if m, ok := resp.Payload.(map[string]any); ok {
		v.Message, _ = m["message"].(string)
	}
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
