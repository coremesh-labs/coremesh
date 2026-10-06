package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"slices"

	"github.com/camel/coremesh/pkg/sdk"
	"github.com/camel/coremesh/pkg/sdk/metamodel"
)

// JSON-API je Modul (Präfix /api/v1/{module}), angemeldet über die Session
// wie die Oberfläche:
//
//	GET  /api/v1/{module}                   Modul mit Objects und aufrufbaren Actions
//	POST /api/v1/{module}/{object}/{action} Payload = JSON-Body → {"payload": …}
//
// Fehler: {"error": "…"} mit dem HTTP-Status wie in der Oberfläche.
// POST verlangt Content-Type application/json – fremde Seiten können das
// ohne CORS-Freigabe nicht senden (CSRF-Schutz zusätzlich zu checkOrigin).

const maxAPIBody = 1 << 20

func (s *server) apiRoutes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.apiModule)
	mux.HandleFunc("GET /{object}", s.apiObject)
	mux.HandleFunc("POST /{object}/{action}", s.apiCall)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		s.apiError(w, r, fmt.Errorf("%w: %s %s", sdk.ErrNotFound, r.Method, r.URL.Path))
	})
	return mux
}

type apiObject struct {
	Object  string   `json:"object"`
	Title   string   `json:"title"`
	Section string   `json:"section,omitempty"`
	Actions []string `json:"actions"`
}

// GET /api/v1/{module}
func (s *server) apiModule(w http.ResponseWriter, r *http.Request) {
	m := moduleFrom(r)
	out := struct {
		Name        string      `json:"name"`
		Title       string      `json:"title"`
		Description string      `json:"description,omitempty"`
		Objects     []apiObject `json:"objects"`
	}{Name: m.Name, Title: m.Title, Description: m.Description, Objects: []apiObject{}}
	for _, o := range m.Objects {
		actions, err := s.allowedActions(r, o.Object)
		if err != nil {
			s.apiError(w, r, err)
			return
		}
		out.Objects = append(out.Objects, apiObject{Object: o.Object, Title: o.Title, Section: o.Section, Actions: actions})
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /api/v1/{module}/{object}/{action}
func (s *server) apiCall(w http.ResponseWriter, r *http.Request) {
	object, action := r.PathValue("object"), r.PathValue("action")
	if _, ok := moduleFrom(r).object(object); !ok {
		s.apiError(w, r, fmt.Errorf("%w: Object %s gehört nicht zu Modul %s", sdk.ErrNotFound, object, moduleFrom(r).Name))
		return
	}
	var payload any
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxAPIBody))
	if err != nil {
		s.apiError(w, r, fmt.Errorf("%w: Body: %v", sdk.ErrInvalidArgument, err))
		return
	}
	if len(body) > 0 {
		if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" {
			writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "Content-Type application/json erwartet"})
			return
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			s.apiError(w, r, fmt.Errorf("%w: JSON: %v", sdk.ErrInvalidArgument, err))
			return
		}
	}
	resp, err := s.call(r, object, action, payload)
	if err != nil {
		s.apiError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"payload": resp.Payload, "metadata": resp.Metadata})
}

func (s *server) apiError(w http.ResponseWriter, r *http.Request, err error) {
	status := errStatus(err)
	msg := err.Error()
	if status >= 500 && !errors.Is(err, sdk.ErrUnavailable) {
		s.logError(r, "API", err)
		msg = "Interner Fehler"
	}
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// GET /api/v1/{module}/{object} – Metadaten für Frontends: Metamodell mit
// Feldtypen, Lookups (Fremdschlüssel) und Relationen (Master-Detail) sowie
// die Actions, die der Benutzer aufrufen darf.
//
//	{"module": "businesspartner", "object": "BusinessPartner",
//	 "definition": {…metamodel.ObjectDefinition…},
//	 "lookups":   [{"field": "…", "object": "…", "value_field": "…", "label_fields": […]}],
//	 "relations": [{"section": "adressen", "object": "PartnerAddress", "foreign_key": "bp_id"}],
//	 "actions":   ["list", "get", …, "getAggregate", "saveAggregate"],
//	 "aggregate": true}
func (s *server) apiObject(w http.ResponseWriter, r *http.Request) {
	object := r.PathValue("object")
	mod := moduleFrom(r)
	if _, ok := mod.object(object); !ok {
		s.apiError(w, r, fmt.Errorf("%w: Object %s gehört nicht zu Modul %s", sdk.ErrNotFound, object, mod.Name))
		return
	}
	oc, err := s.definition(r, object)
	if err != nil {
		s.apiError(w, r, err)
		return
	}
	actions, err := s.allowedActions(r, object)
	if err != nil {
		s.apiError(w, r, err)
		return
	}
	type lookupMeta struct {
		Field string `json:"field"`
		metamodel.Lookup
	}
	type relationMeta struct {
		Section string `json:"section"`
		metamodel.Relation
	}
	lookups, relations := []lookupMeta{}, []relationMeta{}
	for _, f := range oc.Def.Fields {
		if f.Lookup != nil {
			lookups = append(lookups, lookupMeta{Field: f.Key, Lookup: *f.Lookup})
		}
	}
	for _, sd := range oc.Def.Sections {
		if sd.Relation != nil {
			relations = append(relations, relationMeta{Section: sd.Key, Relation: *sd.Relation})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"module": mod.Name, "object": object, "definition": oc.Def,
		"lookups": lookups, "relations": relations, "actions": actions,
		"aggregate": slices.Contains(actions, "getAggregate"),
	})
}

// allowedActions: Routen des Objects (Catalog.ListActions), die der Benutzer aufrufen darf.
func (s *server) allowedActions(r *http.Request, object string) ([]string, error) {
	resp, err := s.call(r, sdk.ObjectCatalog, "ListActions", map[string]any{"object": object})
	if err != nil {
		return nil, err
	}
	var list struct {
		Actions []struct {
			Action string `json:"action"`
		} `json:"actions"`
	}
	_ = sdk.Decode(resp.Payload, &list)
	u := userFrom(r)
	out := []string{}
	for _, a := range list.Actions {
		if u == nil || u.Can(object, a.Action) {
			out = append(out, a.Action)
		}
	}
	return out, nil
}
