package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"

	"github.com/camel/coremesh/pkg/sdk"
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
	u := userFrom(r)
	out := struct {
		Name        string      `json:"name"`
		Title       string      `json:"title"`
		Description string      `json:"description,omitempty"`
		Objects     []apiObject `json:"objects"`
	}{Name: m.Name, Title: m.Title, Description: m.Description, Objects: []apiObject{}}
	for _, o := range m.Objects {
		resp, err := s.call(r, sdk.ObjectCatalog, "ListActions", map[string]any{"object": o.Object})
		if err != nil {
			s.apiError(w, r, err)
			return
		}
		var list struct {
			Actions []struct {
				Action string `json:"action"`
			} `json:"actions"`
		}
		_ = sdk.Decode(resp.Payload, &list)
		ao := apiObject{Object: o.Object, Title: o.Title, Section: o.Section, Actions: []string{}}
		for _, a := range list.Actions {
			if u == nil || u.Can(o.Object, a.Action) {
				ao.Actions = append(ao.Actions, a.Action)
			}
		}
		out.Objects = append(out.Objects, ao)
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
