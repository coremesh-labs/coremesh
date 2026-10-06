package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func sessionFrom(w *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookie {
			return c
		}
	}
	return nil
}

func TestUnauthenticatedIsRedirected(t *testing.T) {
	s, h := newTestServer(t, "")
	n := len(h.calls)

	w := doAnon(s, "GET", "/m/crm/Partner?page=2", nil, false)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/login?next="+url.QueryEscape("/m/crm/Partner?page=2") {
		t.Fatalf("Seitenaufruf: %d %s", w.Code, w.Header().Get("Location"))
	}
	w = doAnon(s, "GET", "/m/crm/Partner", nil, true)
	if w.Code != http.StatusUnauthorized || !strings.HasPrefix(w.Header().Get("HX-Redirect"), "/login") {
		t.Fatalf("HTMX: %d %q", w.Code, w.Header().Get("HX-Redirect"))
	}
	if w := doAnon(s, "POST", "/m/crm/Partner", url.Values{"company_name": {"x"}}, true); w.Code != http.StatusUnauthorized {
		t.Fatalf("POST ohne Anmeldung: %d", w.Code)
	}
	if len(h.calls) != n {
		t.Fatal("ohne Anmeldung darf kein Modul aufgerufen werden")
	}
	// Öffentlich: Login und statische Dateien.
	if w := doAnon(s, "GET", "/login", nil, false); w.Code != 200 || !strings.Contains(w.Body.String(), `name="password"`) {
		t.Fatalf("Login-Seite: %d", w.Code)
	}
	if w := doAnon(s, "GET", "/static/app.css", nil, false); w.Code != 200 {
		t.Fatalf("static: %d", w.Code)
	}
}

func TestLoginLogout(t *testing.T) {
	s, h := newTestServer(t, "")

	w := doAnon(s, "POST", "/login", url.Values{"username": {"Tester"}, "password": {"falsch"}}, false)
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "Benutzername oder Passwort falsch") || sessionFrom(w) != nil {
		t.Fatalf("falsches Passwort: %d", w.Code)
	}
	w = doAnon(s, "POST", "/login", url.Values{"username": {"unbekannt"}, "password": {"x"}}, false)
	if !strings.Contains(w.Body.String(), "Benutzername oder Passwort falsch") {
		t.Fatal("unbekannter Benutzer muss dieselbe Meldung bekommen")
	}

	// Benutzername ist unabhängig von Groß-/Kleinschreibung; next wird befolgt.
	w = doAnon(s, "POST", "/login", url.Values{"username": {" Tester "}, "password": {testPassword}, "next": {"/m/crm/Partner"}}, false)
	c := sessionFrom(w)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/m/crm/Partner" || c == nil {
		t.Fatalf("Login: %d %s", w.Code, w.Header().Get("Location"))
	}
	if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Secure {
		t.Fatalf("Cookie-Flags: %+v", c)
	}

	// Mit dem neuen Cookie: Zugriff, Benutzer im Aufrufkontext.
	r := httptest.NewRequest("GET", "/m/crm/Partner", nil)
	r.AddCookie(c)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, r)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Abmelden") {
		t.Fatalf("mit Session: %d", rec.Code)
	}
	call := h.ctxs[len(h.ctxs)-1]
	if call.UserID == "" || call.Metadata["username"] != "tester" || call.TenantID != "t-42" {
		t.Fatalf("CallContext: %+v", call)
	}

	// Logout beendet die Session serverseitig.
	r = httptest.NewRequest("POST", "/logout", nil)
	r.AddCookie(c)
	rec = httptest.NewRecorder()
	s.ServeHTTP(rec, r)
	r = httptest.NewRequest("GET", "/m/crm/Partner", nil)
	r.AddCookie(c)
	rec = httptest.NewRecorder()
	s.ServeHTTP(rec, r)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("nach Logout: %d", rec.Code)
	}
}

func TestOpenRedirectBlocked(t *testing.T) {
	for _, next := range []string{"https://evil.example", "//evil.example", `/\evil.example`, "javascript:alert(1)", ""} {
		if got := safeNext(next); got != "/" {
			t.Errorf("safeNext(%q) = %q", next, got)
		}
	}
	if safeNext("/m/crm/Partner?x=1") != "/m/crm/Partner?x=1" {
		t.Error("lokaler Pfad muss erlaubt sein")
	}
}

func TestCSRFOriginCheck(t *testing.T) {
	s, h := newTestServer(t, "")
	n := len(h.calls)
	send := func(headers map[string]string) int {
		r := httptest.NewRequest("POST", "/m/crm/Partner", strings.NewReader("company_name=x"))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		tok, _ := testTokens.Load(s)
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: tok.(string)})
		for k, v := range headers {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w.Code
	}
	if code := send(map[string]string{"Sec-Fetch-Site": "cross-site"}); code != http.StatusForbidden {
		t.Fatalf("cross-site: %d", code)
	}
	if code := send(map[string]string{"Origin": "https://evil.example"}); code != http.StatusForbidden {
		t.Fatalf("fremder Origin: %d", code)
	}
	if len(h.calls) != n {
		t.Fatal("abgelehnte Anfrage hat das Modul erreicht")
	}
	if code := send(map[string]string{"Sec-Fetch-Site": "same-origin", "Origin": "http://example.com"}); code == http.StatusForbidden {
		t.Fatalf("eigene Seite abgelehnt: %d", code) // httptest-Host ist example.com
	}
}

func TestBruteForceLimiter(t *testing.T) {
	ids := newMemIdentity()
	ids.add("admin", "richtiges-passwort", "", "*.*")
	a := newAuthService(ids, newMemStore(), time.Hour)
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	a.now = func() time.Time { return now }

	for i := 0; i < freeAttempts; i++ {
		if _, _, _, err := a.Login(context.Background(), "admin", "falsch", "1.2.3.4"); err != errInvalidCredentials {
			t.Fatalf("Versuch %d: %v", i, err)
		}
	}
	if _, _, _, err := a.Login(context.Background(), "admin", "richtiges-passwort", "1.2.3.4"); err != errTooManyAttempts {
		t.Fatalf("gesperrt erwartet: %v", err)
	}
	// Andere IP ist nicht betroffen; nach Ablauf der Sperre geht es wieder.
	if _, _, _, err := a.Login(context.Background(), "admin", "richtiges-passwort", "5.6.7.8"); err != nil {
		t.Fatalf("andere IP: %v", err)
	}
	now = now.Add(2 * time.Minute)
	if _, _, _, err := a.Login(context.Background(), "admin", "richtiges-passwort", "1.2.3.4"); err != nil {
		t.Fatalf("nach Sperre: %v", err)
	}
}

func TestSessionExpiryAndDeactivation(t *testing.T) {
	st, ids := newMemStore(), newMemIdentity()
	ids.add("admin", "admin-passwort-1", "demo", "*.*")
	a := newAuthService(ids, st, time.Hour)

	now := time.Now()
	a.now = func() time.Time { return now }
	_, token, _, err := a.Login(context.Background(), "Admin", "admin-passwort-1", "x")
	if err != nil {
		t.Fatal(err)
	}
	if u, _ := a.Session(context.Background(), token); u == nil {
		t.Fatal("Session gültig erwartet")
	}
	now = now.Add(61 * time.Minute)
	if u, _ := a.Session(context.Background(), token); u != nil {
		t.Fatal("abgelaufene Session akzeptiert")
	}
	// In der Datenbank steht nur der Hash des Tokens.
	for id := range st.sessions {
		if id == token {
			t.Fatal("Token im Klartext gespeichert")
		}
	}

	// Deaktiviert jemand den Benutzer in iam, endet seine Session sofort.
	now = now.Add(-61 * time.Minute)
	_, token, _, _ = a.Login(context.Background(), "admin", "admin-passwort-1", "x")
	ids.deactivate("admin")
	if u, _ := a.Session(context.Background(), token); u != nil {
		t.Fatal("deaktivierter Benutzer hat noch eine Session")
	}
}

func TestChangePassword(t *testing.T) {
	s, _ := newTestServer(t, "")
	tok, _ := testTokens.Load(s)
	post := func(cur, next, confirm string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/account/password", strings.NewReader(url.Values{
			"current_password": {cur}, "new_password": {next}, "confirm_password": {confirm}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: tok.(string)})
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	if w := post("falsch", "neues-passwort-1", "neues-passwort-1"); w.Code != 422 || !strings.Contains(w.Body.String(), "aktuelle Passwort ist falsch") {
		t.Fatalf("falsches aktuelles Passwort: %d", w.Code)
	}
	if w := post(testPassword, "kurz", "kurz"); w.Code != 422 || !strings.Contains(w.Body.String(), "mindestens 10") {
		t.Fatalf("zu kurz: %d", w.Code)
	}
	if w := post(testPassword, "neues-passwort-1", "anders-passwort-1"); w.Code != 422 {
		t.Fatalf("Bestätigung abweichend: %d", w.Code)
	}
	w := post(testPassword, "neues-passwort-1", "neues-passwort-1")
	if w.Code != 200 || sessionFrom(w) == nil {
		t.Fatalf("Ändern: %d", w.Code)
	}
	// Alte Session ist beendet, Login mit neuem Passwort klappt.
	if w := do(s, "GET", "/m/crm/Partner", nil, false); w.Code != http.StatusSeeOther {
		t.Fatalf("alte Session noch gültig: %d", w.Code)
	}
	if w := doAnon(s, "POST", "/login", url.Values{"username": {"tester"}, "password": {"neues-passwort-1"}}, false); w.Code != http.StatusSeeOther {
		t.Fatalf("Login mit neuem Passwort: %d", w.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	s, _ := newTestServer(t, "")
	w := doAnon(s, "GET", "/login", nil, false)
	for k, v := range map[string]string{"X-Frame-Options": "DENY", "X-Content-Type-Options": "nosniff", "Cache-Control": "no-store"} {
		if w.Header().Get(k) != v {
			t.Errorf("%s = %q", k, w.Header().Get(k))
		}
	}
}

// TestPermissionsShapeUI: Navigation und Buttons zeigen nur, was die Rollen
// erlauben. (Verbindlich prüft der Dispatcher; hier geht es um die Darstellung.)
func TestPermissionsShapeUI(t *testing.T) {
	s, _ := newTestServer(t, "")
	ids, _ := testIdentities.Load(s)
	ids.(*memIdentity).add("leser", "leser-passwort-1", "t-7", "Partner.List", "Partner.Item", "Greeting.*")

	login := doAnon(s, "POST", "/login", url.Values{"username": {"leser"}, "password": {"leser-passwort-1"}}, false)
	c := sessionFrom(login)
	get := func(path string) string {
		r := httptest.NewRequest("GET", path, nil)
		r.AddCookie(c)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w.Body.String()
	}

	list := get("/m/crm/Partner")
	mustContain(t, list, "ACME", ">Anzeigen<", `href="/m/crm/Partner"`)
	mustNotContain(t, list, ">Neu<", ">Bearbeiten<", ">Löschen<", ">Benachrichtigen<")

	u := &user{Permissions: []string{"Partner.*", "*.list"}}
	if !u.Can("Partner", "Delete") || !u.Can("Contract", "list") || u.Can("Contract", "get") || !u.CanAny("Partner") || !u.CanAny("Contract") || (&user{Permissions: []string{"Partner.*"}}).CanAny("Contract") {
		t.Fatal("Can/CanAny")
	}
}

func TestForbiddenIs403(t *testing.T) {
	s, _ := newTestServer(t, "")
	ids, _ := testIdentities.Load(s)
	ids.(*memIdentity).add("leser", "leser-passwort-1", "", "Partner.List")
	c := sessionFrom(doAnon(s, "POST", "/login", url.Values{"username": {"leser"}, "password": {"leser-passwort-1"}}, false))
	for _, tc := range []struct{ method, path string }{
		{"GET", "/m/crm/Partner/new"},          // create nicht erlaubt
		{"POST", "/action/crm/Partner/Notify"}, // custom nicht erlaubt
		{"GET", "/m/crm/Partner/p1"},           // item nicht erlaubt
	} {
		r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(""))
		r.Header.Set("HX-Request", "true")
		r.AddCookie(c)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "keine Berechtigung") {
			t.Errorf("%s %s: %d", tc.method, tc.path, w.Code)
		}
	}
}
