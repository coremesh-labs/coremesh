package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/camel/coremesh/pkg/sdk/metamodel"
)

func TestAcceptLanguage(t *testing.T) {
	for header, want := range map[string]string{
		"zh-CN,zh;q=0.9,en;q=0.8": "zh-CN",
		"fr-FR,en;q=0.5,de;q=0.7": "de",
		"zh-TW,en;q=0.3":          "en",
		"fr,it":                   "",
		"":                        "",
		"en-GB;q=0":               "",
	} {
		if got := acceptLanguage(header); got != want {
			t.Errorf("%q → %q, erwartet %q", header, got, want)
		}
	}
}

// TestCoreDictionariesComplete: Alle Framework-Schlüssel gibt es in jeder Sprache.
func TestCoreDictionariesComplete(t *testing.T) {
	core := coreTexts()
	for key := range core[metamodel.LocaleDE] {
		for _, loc := range metamodel.Locales {
			if core[loc][key] == "" {
				t.Errorf("%s: %s fehlt", loc, key)
			}
		}
	}
	for _, loc := range metamodel.Locales {
		if len(core[loc]) != len(core[metamodel.LocaleDE]) {
			t.Errorf("%s: %d Schlüssel, de: %d", loc, len(core[loc]), len(core[metamodel.LocaleDE]))
		}
	}
}

func get(s *server, path string, header map[string]string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", path, nil)
	for k, v := range header {
		r.Header.Set(k, v)
	}
	for _, c := range cookies {
		r.AddCookie(c)
	}
	if tok, ok := testTokens.Load(s); ok {
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: tok.(string)})
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

// TestLocaleNegotiation: Profil → Sprachwähler (Cookie) → Accept-Language → Standard.
func TestLocaleNegotiation(t *testing.T) {
	s, _ := newTestServer(t, "")
	ids, _ := testIdentities.Load(s)
	lang := func(w *httptest.ResponseRecorder) string { return w.Header().Get("Content-Language") }

	if got := lang(get(s, "/m/crm/Partner", nil)); got != "de" {
		t.Errorf("Standard: %s", got)
	}
	if got := lang(get(s, "/m/crm/Partner", map[string]string{"Accept-Language": "en-US,en;q=0.9"})); got != "en" {
		t.Errorf("Accept-Language: %s", got)
	}
	zhCookie := &http.Cookie{Name: localeCookie, Value: "zh-CN"}
	if got := lang(get(s, "/m/crm/Partner", map[string]string{"Accept-Language": "en"}, zhCookie)); got != "zh-CN" {
		t.Errorf("Sprachwähler vor Accept-Language: %s", got)
	}
	// Profil schlägt alles.
	ids.(*memIdentity).users["tester"].Locale = "en"
	if got := lang(get(s, "/m/crm/Partner", map[string]string{"Accept-Language": "zh"}, zhCookie)); got != "en" {
		t.Errorf("Profil: %s", got)
	}
}

// TestLocalizedUI: Framework-Texte, Modul-Texte und Standardtexte der Actions in zh-CN.
func TestLocalizedUI(t *testing.T) {
	s, _ := newTestServer(t, "")
	zh := map[string]string{"Accept-Language": "zh-CN"}
	b := get(s, "/m/crm/Partner", zh).Body.String()
	mustContain(t, b,
		`<html lang="zh-CN">`,
		"<h1><i class=\"icon-users\"></i>伙伴</h1>", // Object-Titel aus dem Modul (crm.Partner.title)
		">客户关系",                                   // Modul in der Navigation (crm.module.title)
		">新建</a>",                                 // Action create ohne eigene Übersetzung: core.action.create
		"显示已停用 / 历史条目",                            // Schalter (Partner hat ein Status-Flag)
		`<option value="zh-CN" selected>中文（简体）</option>`,
		"退出登录",
	)
	// Validierungsmeldung im Formular (englisch).
	r := httptest.NewRequest("POST", "/m/crm/Partner", strings.NewReader(url.Values{"employees": {"viele"}}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Accept-Language", "en")
	r.Header.Set("HX-Request", "true")
	tok, _ := testTokens.Load(s)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: tok.(string)})
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, r)
	mustContain(t, rec.Body.String(), "Number expected", "Required", ">Save<")
}

func TestI18nAPIAndLocaleSwitch(t *testing.T) {
	s, h := newTestServer(t, "")
	var out struct {
		Locale       string
		Locales      []string
		Translations map[string]string
	}
	w := get(s, "/api/v1/i18n/zh", nil)
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if out.Locale != "zh-CN" || out.Translations["core.app.save"] != "保存" || out.Translations["crm.module.title"] != "客户关系" || len(out.Locales) != 3 {
		t.Fatalf("Wörterbuch: %s", w.Body.String())
	}
	if w := get(s, "/api/v1/i18n/fr", nil); w.Code != 422 {
		t.Fatalf("unbekannte Sprache: %d", w.Code)
	}

	// PATCH /api/v1/user/profile → iam Account.UpdateProfile.
	r := httptest.NewRequest("PATCH", "/api/v1/user/profile", strings.NewReader(`{"locale":"en"}`))
	r.Header.Set("Content-Type", "application/json")
	tok, _ := testTokens.Load(s)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: tok.(string)})
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, r)
	if rec.Code != 200 || h.find("UpdateProfile").Payload.(map[string]any)["locale"] != "en" {
		t.Fatalf("Profil: %d %s", rec.Code, rec.Body.String())
	}

	// Sprachwähler: Cookie + Profil, zurück zur Seite.
	w = do(s, "POST", "/locale", url.Values{"locale": {"zh"}, "next": {"/m/crm/Partner"}}, false)
	c := sessionCookieNamed(w, localeCookie)
	if w.Code != 303 || w.Header().Get("Location") != "/m/crm/Partner" || c == nil || c.Value != "zh-CN" {
		t.Fatalf("Sprachwähler: %d %v %v", w.Code, w.Header(), c)
	}
	if h.find("UpdateProfile").Payload.(map[string]any)["locale"] != "zh-CN" {
		t.Fatal("Sprachwahl nicht im Profil gespeichert")
	}
	// Automatisch: Cookie wird gelöscht.
	w = do(s, "POST", "/locale", url.Values{"locale": {"auto"}}, false)
	if c := sessionCookieNamed(w, localeCookie); c == nil || c.MaxAge >= 0 {
		t.Fatalf("automatisch: %v", c)
	}
}

func TestHistoryToggle(t *testing.T) {
	s, h := newTestServer(t, "")
	b := do(s, "GET", "/m/crm/Partner", nil, true).Body.String()
	mustContain(t, b, `name="includeHistory"`, `hx-get="/m/crm/Partner" hx-target="#list"`)
	do(s, "GET", "/m/crm/Partner?includeHistory=true", nil, true)
	if q := h.find("List").Payload.(map[string]any)["query"].(map[string]any); q["includeHistory"] != "true" {
		t.Fatalf("Parameter an das Modul: %v", q)
	}
}

func sessionCookieNamed(w *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// TestLocaleSwitchReturnPath: Der Sprachwähler springt auf die volle URL
// zurück, auch innerhalb eines Moduls (/m/{module}/…).
func TestLocaleSwitchReturnPath(t *testing.T) {
	s, _ := newTestServer(t, "")
	b := do(s, "GET", "/m/crm/Partner/p%2F1", nil, false).Body.String()
	mustContain(t, b, `<input type="hidden" name="next" value="/m/crm/Partner/p%2F1">`)
}
