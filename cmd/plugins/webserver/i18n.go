package main

import (
	"context"
	"embed"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
	"github.com/coremesh-labs/coremesh/pkg/sdk/module"
)

// Mehrsprachigkeit (de, en, zh-CN).
//
// Zwei Ebenen:
//   - Framework-Texte (Speichern, Abbrechen, Dialoge, Validierung, Meldungen)
//     gehören dem WebServer: i18n/<locale>.json, Schlüssel "core.…".
//   - Fachtexte (Titel, Feld-Labels, Abschnitte, Navigation) gehören den
//     Modulen. Sie melden sie mit Catalog.Describe beim Catalog an; der
//     WebServer holt sie mit Catalog.Translations (zwischengespeichert).
//
// Sprachaushandlung (localeMiddleware), in dieser Reihenfolge:
//  1. Profil des Benutzers (iam, PATCH /api/v1/user/profile)
//  2. Sprachwähler der Oberfläche (Cookie coremesh_lang, POST /locale)
//  3. Accept-Language des Browsers
//  4. Standard aus der Konfiguration (settings.default_locale, sonst de)

//go:embed i18n/*.json
var coreFiles embed.FS

const localeCookie = "coremesh_lang"

// translationService liefert Texte je Sprache.
type translationService struct {
	core    metamodel.Translations
	host    sdk.Host
	def     string        // Standardsprache
	ttl     time.Duration // Zwischenspeicher der Modul-Texte
	mu      sync.Mutex
	modules map[string]cachedDict
}

type cachedDict struct {
	dict  map[string]string
	until time.Time
}

func newTranslationService(host sdk.Host, defaultLocale string) (*translationService, error) {
	core, err := module.LoadTranslations(coreFiles, "i18n")
	if err != nil {
		return nil, err
	}
	def := metamodel.NormalizeLocale(defaultLocale)
	if def == "" {
		def = metamodel.LocaleDE
	}
	return &translationService{core: core, host: host, def: def, ttl: 30 * time.Second, modules: map[string]cachedDict{}}, nil
}

// T übersetzt einen Framework-Schlüssel; args wie bei fmt.Sprintf.
func (t *translationService) T(locale, key string, args ...any) string {
	s := t.core.Lookup(locale, key, key)
	if len(args) > 0 {
		return fmt.Sprintf(s, args...)
	}
	return s
}

// moduleDict holt die Fachtexte der Module (Catalog.Translations), mit
// Rückfall auf de; Fehler ergeben ein leeres Wörterbuch (Originaltexte).
func (t *translationService) moduleDict(ctx context.Context, locale string) map[string]string {
	t.mu.Lock()
	c, ok := t.modules[locale]
	t.mu.Unlock()
	if ok && time.Now().Before(c.until) {
		return c.dict
	}
	dict := map[string]string{}
	resp, err := t.host.Handle(ctx, sdk.Request{Object: sdk.ObjectCatalog, Action: "Translations", Payload: map[string]any{"locale": locale}})
	if err == nil {
		var out struct {
			Translations map[string]string `json:"translations"`
		}
		if sdk.Decode(resp.Payload, &out) == nil && out.Translations != nil {
			dict = out.Translations
		}
	}
	t.mu.Lock()
	t.modules[locale] = cachedDict{dict: dict, until: time.Now().Add(t.ttl)}
	t.mu.Unlock()
	return dict
}

// dictionary ist das zusammengeführte Wörterbuch einer Sprache: Core und
// Module, fehlende Schlüssel aus de.
func (t *translationService) dictionary(ctx context.Context, locale string) map[string]string {
	out := map[string]string{}
	maps.Copy(out, t.core[metamodel.LocaleDE])
	maps.Copy(out, t.core[locale])
	maps.Copy(out, t.moduleDict(ctx, locale))
	return out
}

// --- Sprachaushandlung -------------------------------------------------------

type localeKey struct{}

// localeFrom liefert die Sprache der Anfrage (gesetzt von localeMiddleware).
func localeFrom(r *http.Request) string {
	if l, ok := r.Context().Value(localeKey{}).(string); ok {
		return l
	}
	return metamodel.LocaleDE
}

// negotiate bestimmt die Sprache: Profil → Sprachwähler → Accept-Language → Standard.
func (t *translationService) negotiate(r *http.Request) string {
	if u := userFrom(r); u != nil {
		if l := metamodel.NormalizeLocale(u.Locale); l != "" {
			return l
		}
	}
	if c, err := r.Cookie(localeCookie); err == nil {
		if l := metamodel.NormalizeLocale(c.Value); l != "" {
			return l
		}
	}
	if l := acceptLanguage(r.Header.Get("Accept-Language")); l != "" {
		return l
	}
	return t.def
}

func (s *server) localeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loc := s.i18n.negotiate(r)
		w.Header().Set("Content-Language", loc)
		w.Header().Add("Vary", "Accept-Language, Cookie")
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), localeKey{}, loc)))
	})
}

// acceptLanguage wählt die unterstützte Sprache mit dem höchsten q-Wert.
func acceptLanguage(header string) string {
	type entry struct {
		loc string
		q   float64
		pos int
	}
	var list []entry
	for i, part := range strings.Split(header, ",") {
		tag, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		q := 1.0
		if v, ok := strings.CutPrefix(strings.TrimSpace(params), "q="); ok {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				q = f
			}
		}
		if loc := metamodel.NormalizeLocale(tag); loc != "" && q > 0 {
			list = append(list, entry{loc, q, i})
		}
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].q > list[j].q })
	if len(list) == 0 {
		return ""
	}
	return list[0].loc
}

// --- Lokalisierung von Metadaten ----------------------------------------------

// localizeDef übersetzt ein Metamodell in die Sprache der Anfrage. Actions
// ohne eigene Übersetzung erhalten den Standardtext ihres Kinds.
func (s *server) localizeDef(r *http.Request, d metamodel.ObjectDefinition) metamodel.ObjectDefinition {
	loc := localeFrom(r)
	dict := s.i18n.dictionary(r.Context(), loc)
	d.Actions = slices.Clone(d.Actions)
	for i, a := range d.Actions {
		if _, ok := dict[a.LabelKey]; !ok && a.Kind != metamodel.KindCustom {
			d.Actions[i].LabelKey = "core.action." + string(a.Kind)
		}
	}
	return d.Localize(metamodel.Translations{loc: dict}, loc)
}

// localizeModule übersetzt Titel, Beschreibung, Objects und Gruppen eines Moduls.
func (s *server) localizeModule(r *http.Request, m moduleInfo) moduleInfo {
	dict := s.i18n.dictionary(r.Context(), localeFrom(r))
	tr := func(key, fallback string) string {
		if v, ok := dict[key]; ok && key != "" {
			return v
		}
		return fallback
	}
	m.Title, m.Description = tr(m.TitleKey, m.Title), tr(m.DescriptionKey, m.Description)
	m.Objects = slices.Clone(m.Objects)
	for i, o := range m.Objects {
		o.Title, o.Section = tr(o.TitleKey, o.Title), tr(o.SectionKey, o.Section)
		m.Objects[i] = o
	}
	return m
}

// T übersetzt einen Framework-Text in die Sprache der Anfrage.
func (s *server) T(r *http.Request, key string, args ...any) string {
	return s.i18n.T(localeFrom(r), key, args...)
}

// --- Endpunkte ---------------------------------------------------------------

// GET /api/v1/i18n/{lang}: zusammengeführtes Wörterbuch (Core + Module).
func (s *server) apiTranslations(w http.ResponseWriter, r *http.Request) {
	loc := metamodel.NormalizeLocale(r.PathValue("lang"))
	if loc == "" {
		s.apiError(w, r, fmt.Errorf("%w: Sprache %q nicht unterstützt (%v)", sdk.ErrInvalidArgument, r.PathValue("lang"), metamodel.Locales))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"locale": loc, "locales": metamodel.Locales, "translations": s.i18n.dictionary(r.Context(), loc)})
}

// GET/PATCH /api/v1/user/profile: eigenes Profil; PATCH {"locale": "en"|""}.
func (s *server) apiProfile(w http.ResponseWriter, r *http.Request) {
	action, payload := "Me", any(nil)
	if r.Method == http.MethodPatch {
		var in map[string]any
		if err := decodeJSONBody(w, r, &in); err != nil {
			s.apiError(w, r, err)
			return
		}
		action, payload = "UpdateProfile", in
	}
	resp, err := s.call(r, "Account", action, payload)
	if err != nil {
		s.apiError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, resp.Payload)
}

// POST /locale (Sprachwähler der Oberfläche): setzt das Cookie; bei
// angemeldeten Benutzern wird die Wahl auch im Profil gespeichert, damit sie
// über Geräte hinweg gilt. "auto" bzw. leer = automatisch.
func (s *server) setLocale(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, s.T(r, "core.app.bad_request"), http.StatusBadRequest)
		return
	}
	loc := metamodel.NormalizeLocale(r.PostForm.Get("locale"))
	c := &http.Cookie{Name: localeCookie, Value: loc, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode,
		Secure: s.secureCookie(r), MaxAge: 365 * 24 * 3600}
	if loc == "" {
		c.MaxAge = -1
	}
	http.SetCookie(w, c)
	if userFrom(r) != nil {
		if _, err := s.call(r, "Account", "UpdateProfile", map[string]any{"locale": loc}); err != nil {
			s.logError(r, "Sprache im Profil", err)
		}
	}
	back := safeNext(r.PostForm.Get("next"))
	if isHTMX(r) {
		w.Header().Set("HX-Redirect", back)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// POST /keydate: Stichtag (Arbeitsdatum) im Profil setzen; leer oder
// clear=1 = heute. Neuanlagen und Prüfungen „zum heutigen Tag“ nehmen ihn.
func (s *server) setKeyDate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, s.T(r, "core.app.bad_request"), http.StatusBadRequest)
		return
	}
	date := strings.TrimSpace(r.PostForm.Get("key_date"))
	if r.PostForm.Get("clear") != "" {
		date = ""
	}
	if _, err := s.call(r, "Account", "UpdateProfile", map[string]any{"key_date": date}); err != nil {
		s.fail(w, r, err)
		return
	}
	back := safeNext(r.PostForm.Get("next"))
	if isHTMX(r) {
		w.Header().Set("HX-Redirect", back)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// coreTexts: Framework-Texte aller Sprachen für Stellen ohne Übersetzungsdienst.
var coreTexts = sync.OnceValue(func() metamodel.Translations {
	t, _ := module.LoadTranslations(coreFiles, "i18n")
	return t
})

// T übersetzt einen Framework-Text in der Sprache des objectCtx (gesetzt
// beim Laden des Metamodells), sonst auf Deutsch.
func (oc objectCtx) T(key string, args ...any) string {
	if oc.tr != nil {
		return oc.tr(key, args...)
	}
	s := coreTexts().Lookup(metamodel.LocaleDE, key, key)
	if len(args) > 0 {
		return fmt.Sprintf(s, args...)
	}
	return s
}

// withLocale bindet Übersetzung und Lokalisierung an den objectCtx.
func (s *server) withLocale(r *http.Request, oc objectCtx) objectCtx {
	loc := localeFrom(r)
	oc.tr = func(key string, args ...any) string { return s.i18n.T(loc, key, args...) }
	return oc
}

// earlyText übersetzt für Middleware vor der Sprachaushandlung (ohne
// Benutzer): Sprachwähler-Cookie, sonst Accept-Language, sonst Deutsch.
func earlyText(r *http.Request, key string) string {
	loc := ""
	if c, err := r.Cookie(localeCookie); err == nil {
		loc = metamodel.NormalizeLocale(c.Value)
	}
	if loc == "" {
		loc = acceptLanguage(r.Header.Get("Accept-Language"))
	}
	return coreTexts().Lookup(loc, key, key)
}

// originalURI ist der Pfad, wie der Browser ihn angefragt hat – in den
// Modul-Sub-Routern ist r.URL bereits um /m/{module} gekürzt (StripPrefix).
func originalURI(r *http.Request) string {
	if r.RequestURI != "" {
		return r.RequestURI
	}
	return r.URL.RequestURI()
}
