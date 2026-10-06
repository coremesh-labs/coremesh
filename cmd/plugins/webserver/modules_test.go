package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestModuleRouting: Der WebServer kennt nur Module; jedes Object ist nur im
// Namensraum seines Moduls erreichbar.
func TestModuleRouting(t *testing.T) {
	s, h := newTestServer(t, "")

	// Startseite: Modul-Kacheln, keine Objects.
	home := do(s, "GET", "/", nil, false).Body.String()
	mustContain(t, home, `<a href="/m/crm">`, "<strong>CRM</strong>", "1 Objekt")

	// Modul-Einstieg = Übersicht des ersten Objects, Seitenleiste mit Gruppen.
	for _, path := range []string{"/m/crm", "/m/crm/"} {
		w := do(s, "GET", path, nil, false)
		if w.Code != 200 {
			t.Fatalf("%s: %d", path, w.Code)
		}
		mustContain(t, w.Body.String(), "ACME", `class="module active"`, `<span class="section">Stammdaten</span>`,
			`href="/m/crm/Partner" hx-get="/m/crm/Partner"`)
	}
	if h.find("List").Object != "Partner" {
		t.Fatal("Modul-Einstieg ruft nicht Partner.List")
	}

	// Kapselung: fremdes Object, unbekanntes Modul, ungültiger Pfad.
	for path, want := range map[string]int{
		"/m/crm/Fremd":           404,
		"/m/gibtsnicht/Partner":  404,
		"/action/crm/Fremd/Ping": 404,
		"/ui/Partner":            404, // alte Objekt-Routen gibt es nicht mehr
		"/m/crm/Partner/p1/x/y":  404,
	} {
		if w := do(s, "GET", path, nil, true); w.Code != want {
			t.Errorf("%s: %d, erwartet %d", path, w.Code, want)
		}
	}
	if h.find("GetDefinition").Payload.(map[string]any)["object"] == "Fremd" {
		t.Fatal("Metamodell eines fremden Objects geladen")
	}
}

// TestModuleHiddenWithoutPermission: Ein Modul ohne berechtigtes Object
// erscheint nicht und ist gesperrt.
func TestModuleHiddenWithoutPermission(t *testing.T) {
	s, _ := newTestServer(t, "")
	ids, _ := testIdentities.Load(s)
	ids.(*memIdentity).add("gast", "gast-passwort-1", "", "Greeting.*")
	c := sessionFrom(doAnon(s, "POST", "/login", url.Values{"username": {"gast"}, "password": {"gast-passwort-1"}}, false))
	get := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		r.AddCookie(c)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	mustNotContain(t, get("/").Body.String(), "/m/crm")
	if w := get("/m/crm/Partner"); w.Code != http.StatusForbidden {
		t.Fatalf("gesperrtes Modul: %d", w.Code)
	}
}

func TestModuleAPI(t *testing.T) {
	s, h := newTestServer(t, "")
	api := func(method, path, ctype, body string, anonymous bool) (int, map[string]any) {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if ctype != "" {
			r.Header.Set("Content-Type", ctype)
		}
		if tok, ok := testTokens.Load(s); ok && !anonymous {
			r.AddCookie(&http.Cookie{Name: sessionCookie, Value: tok.(string)})
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		var out map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s %s: keine JSON-Antwort: %q", method, path, w.Body.String())
		}
		return w.Code, out
	}

	code, out := api("POST", "/api/v1/crm/Partner/Item", "application/json", `{"id":"p/1"}`, false)
	if code != 200 || out["payload"].(map[string]any)["company_name"] != "ACME <AG>" {
		t.Fatalf("Aufruf: %d %v", code, out)
	}
	if p := h.last().Payload.(map[string]any); p["id"] != "p/1" {
		t.Fatalf("Payload: %v", p)
	}

	for _, tc := range []struct {
		method, path, ctype, body string
		anonymous                 bool
		want                      int
	}{
		{"POST", "/api/v1/crm/Partner/Item", "", `{"id":"p/1"}`, true, 401},              // ohne Session
		{"POST", "/api/v1/crm/Partner/Item", "text/plain", `{"id":"p/1"}`, false, 415},   // CSRF-Schutz
		{"POST", "/api/v1/crm/Partner/Item", "application/json", `{kaputt`, false, 422},  // ungültiges JSON
		{"POST", "/api/v1/crm/Fremd/List", "application/json", `{}`, false, 404},         // fremdes Object
		{"POST", "/api/v1/crm/Partner/Gibtsnicht", "application/json", `{}`, false, 405}, // keine Route
		{"GET", "/api/v1/gibtsnicht", "", "", false, 404},                                // unbekanntes Modul
		{"GET", "/api/v1/crm/Partner/List", "", "", false, 404},                          // nur POST
	} {
		if code, out := api(tc.method, tc.path, tc.ctype, tc.body, tc.anonymous); code != tc.want || out["error"] == "" {
			t.Errorf("%s %s: %d %v, erwartet %d", tc.method, tc.path, code, out, tc.want)
		}
	}
}

func TestModuleAPIDescribe(t *testing.T) {
	s, _ := newTestServer(t, "")
	ids, _ := testIdentities.Load(s)
	ids.(*memIdentity).add("leser", "leser-passwort-1", "", "Partner.List")
	c := sessionFrom(doAnon(s, "POST", "/login", url.Values{"username": {"leser"}, "password": {"leser-passwort-1"}}, false))
	r := httptest.NewRequest("GET", "/api/v1/crm", nil)
	r.AddCookie(c)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	var out struct {
		Name    string
		Objects []apiObject
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	// Nur Actions, die der Benutzer aufrufen darf.
	if out.Name != "crm" || len(out.Objects) != 2 || strings.Join(out.Objects[0].Actions, ",") != "List" || out.Objects[1].Object != "Search" {
		t.Fatalf("Modulbeschreibung: %+v", out)
	}
}

// TestModuleServiceAPI: Services eines Moduls (Objects ohne Metamodell, z. B. Tags)
// sind über die JSON-API erreichbar, aber nicht in der Navigation.
func TestModuleServiceAPI(t *testing.T) {
	s, _ := newTestServer(t, "")
	r := httptest.NewRequest("POST", "/api/v1/crm/Search/run", strings.NewReader(`{}`))
	r.Header.Set("Content-Type", "application/json")
	tok, _ := testTokens.Load(s)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: tok.(string)})
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"hits": 1`) {
		t.Fatalf("Service über API: %d %s", w.Code, w.Body.String())
	}
	mustNotContain(t, do(s, "GET", "/m/crm/Partner", nil, false).Body.String(), "/m/crm/Search")
}
