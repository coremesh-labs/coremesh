package main

import (
	"context"
	"fmt"
	"net/http"
	"slices"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
)

// Modul-Schicht des WebServers.
//
// Der WebServer registriert ausschließlich Module (Catalog.ListModules).
// Jedes Modul erhält drei gekapselte Sub-Router mit eigenem Präfix:
//
//	/m/{module}/…        Oberfläche (HTMX), siehe uiRoutes
//	/action/{module}/…   custom-Actions, siehe actionRoutes
//	/api/v1/{module}/…   JSON-API, siehe apiRoutes
//
// mountModule löst das Modul einmal je Anfrage auf, prüft die Sichtbarkeit
// für den Benutzer und reicht die Anfrage mit abgeschnittenem Präfix an den
// Sub-Router weiter. Innerhalb eines Moduls sind nur dessen Objects
// erreichbar; ein Object eines anderen Moduls ergibt 404.

// moduleInfo ist ein Modul aus Catalog.GetModule/ListModules.
type moduleInfo struct {
	Name        string         `json:"name"`
	Title       string         `json:"title"`
	Icon        string         `json:"icon"`
	Description string         `json:"description"`
	Available   bool           `json:"available"`
	Objects     []moduleObject `json:"objects"`
	Services    []string       `json:"services"` // Objects ohne Metamodell (nur JSON-API)

	TitleKey       string `json:"title_key"`
	DescriptionKey string `json:"description_key"`
}

type moduleObject struct {
	Object     string `json:"object"`
	Title      string `json:"title"`
	Icon       string `json:"icon"`
	Section    string `json:"section"`
	Available  bool   `json:"available"`
	TitleKey   string `json:"title_key"`
	SectionKey string `json:"section_key"`
}

// visibleFor behält nur Objects, die laufen und für die der Benutzer
// mindestens eine Berechtigung hat. Verbindlich prüft der Dispatcher.
func (m moduleInfo) visibleFor(u *user) moduleInfo {
	var objs []moduleObject
	for _, o := range m.Objects {
		if o.Available && (u == nil || u.CanAny(o.Object)) {
			objs = append(objs, o)
		}
	}
	m.Objects = objs
	return m
}

func (m moduleInfo) object(name string) (moduleObject, bool) {
	i := slices.IndexFunc(m.Objects, func(o moduleObject) bool { return o.Object == name })
	if i < 0 {
		return moduleObject{}, false
	}
	return m.Objects[i], true
}

type moduleKey struct{}

// moduleFrom liefert das Modul der laufenden Anfrage (gesetzt von mountModule).
func moduleFrom(r *http.Request) moduleInfo {
	m, _ := r.Context().Value(moduleKey{}).(moduleInfo)
	return m
}

// mountModule bindet einen Sub-Router unter prefix/{module}/ ein.
func (s *server) mountModule(prefix string, sub http.Handler, onError func(http.ResponseWriter, *http.Request, error)) {
	s.mux.HandleFunc(prefix+"/{module}/", func(w http.ResponseWriter, r *http.Request) {
		m, err := s.loadModule(r, r.PathValue("module"))
		if err != nil {
			onError(w, r, err)
			return
		}
		ctx := context.WithValue(r.Context(), moduleKey{}, m)
		http.StripPrefix(prefix+"/"+m.Name, sub).ServeHTTP(w, r.WithContext(ctx))
	})
	// Ohne abschließenden Schrägstrich: wie die Startseite des Moduls.
	s.mux.HandleFunc(prefix+"/{module}", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path += "/"
		if r.URL.RawPath != "" {
			r.URL.RawPath += "/"
		}
		s.mux.ServeHTTP(w, r)
	})
}

// loadModule holt das Modul aus dem Catalog und beschränkt es auf die
// Objects, die der Benutzer sehen darf.
func (s *server) loadModule(r *http.Request, name string) (moduleInfo, error) {
	resp, err := s.call(r, sdk.ObjectCatalog, "GetModule", map[string]any{"module": name})
	if err != nil {
		return moduleInfo{}, err
	}
	var m moduleInfo
	if err := sdk.Decode(resp.Payload, &m); err != nil {
		return moduleInfo{}, err
	}
	if !m.Available {
		return moduleInfo{}, fmt.Errorf("%w: %s", sdk.ErrUnavailable, s.T(r, "core.error.module_unavailable", m.Title))
	}
	v := m.visibleFor(userFrom(r))
	if len(v.Objects) == 0 {
		return moduleInfo{}, fmt.Errorf("%w: %s", sdk.ErrPermissionDenied, s.T(r, "core.error.module_no_permission", m.Title))
	}
	return s.localizeModule(r, v), nil
}

// modules liefert alle Module, die der Benutzer sehen darf (Navigation, Startseite).
func (s *server) modules(r *http.Request) []moduleInfo {
	resp, err := s.call(r, sdk.ObjectCatalog, "ListModules", nil)
	if err != nil {
		s.logError(r, "Navigation", err)
		return nil
	}
	var list struct {
		Modules []moduleInfo `json:"modules"`
	}
	_ = sdk.Decode(resp.Payload, &list)
	var out []moduleInfo
	for _, m := range list.Modules {
		if v := m.visibleFor(userFrom(r)); len(v.Objects) > 0 {
			out = append(out, s.localizeModule(r, v))
		}
	}
	return out
}

// moduleURL und actionURL sind die Basis-Pfade eines Objects im Modul.
func moduleURL(module, object string) string { return "/m/" + module + "/" + object }
func actionURL(module, object string) string { return "/action/" + module + "/" + object }

// uiRoutes ist der Sub-Router der Oberfläche eines Moduls (Präfix /m/{module}).
func (s *server) uiRoutes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.moduleHome)
	mux.HandleFunc("GET /{object}", s.list)
	mux.HandleFunc("GET /{object}/new", s.newForm)
	mux.HandleFunc("POST /{object}", s.create)
	mux.HandleFunc("POST /{object}/_form", s.formRefresh)
	mux.HandleFunc("GET /{object}/{id}", s.item)
	mux.HandleFunc("GET /{object}/{id}/edit", s.editForm)
	mux.HandleFunc("GET /{object}/{id}/rel/{section}", s.relation)
	mux.HandleFunc("GET /{object}/{id}/end", s.endForm)
	mux.HandleFunc("POST /{object}/{id}/end", s.end)
	mux.HandleFunc("PUT /{object}/{id}", s.update)
	mux.HandleFunc("DELETE /{object}/{id}", s.delete)
	mux.HandleFunc("POST /{object}/{id}", s.methodOverride)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		s.fail(w, r, fmt.Errorf("%w: %s", sdk.ErrNotFound, r.URL.Path))
	})
	return mux
}

// actionRoutes ist der Sub-Router der custom-Actions (Präfix /action/{module}).
func (s *server) actionRoutes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{object}/{name}", s.actionForm)
	mux.HandleFunc("POST /{object}/{name}", s.runAction)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		s.fail(w, r, fmt.Errorf("%w: %s", sdk.ErrNotFound, r.URL.Path))
	})
	return mux
}

// GET /m/{module}: Einstieg = Übersicht des ersten sichtbaren Objects.
func (s *server) moduleHome(w http.ResponseWriter, r *http.Request) {
	r.SetPathValue("object", moduleFrom(r).Objects[0].Object)
	s.list(w, r)
}

// --- Navigation --------------------------------------------------------------

// navModule ist ein Modul in der Seitenleiste; nur das aktive zeigt seine Objects.
type navModule struct {
	Name, Title, Icon string
	Active            bool
	Sections          []navSection
}

type navSection struct {
	Title string
	Items []navItem
}

func (s *server) nav(r *http.Request, activeObject string) []navModule {
	active := moduleFrom(r).Name
	var out []navModule
	for _, m := range s.modules(r) {
		nm := navModule{Name: m.Name, Title: m.Title, Icon: m.Icon, Active: m.Name == active}
		if nm.Active {
			for _, o := range m.Objects {
				if n := len(nm.Sections); n == 0 || nm.Sections[n-1].Title != o.Section {
					nm.Sections = append(nm.Sections, navSection{Title: o.Section})
				}
				sec := &nm.Sections[len(nm.Sections)-1]
				sec.Items = append(sec.Items, navItem{Object: o.Object, Title: o.Title, Icon: o.Icon,
					URL: moduleURL(m.Name, o.Object), Active: o.Object == activeObject})
			}
		}
		out = append(out, nm)
	}
	return out
}
