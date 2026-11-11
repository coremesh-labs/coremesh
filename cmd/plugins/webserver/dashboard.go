package main

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// Dashboard (seit 0.66.0): Ein Modul kann seinen Status liefern
// (ModuleDefinition.Dashboard → metamodel.DashboardResponse).
//
//   - Startseite: die Kachel des Moduls zeigt die Zahlen aus Summary (z. B.
//     „Rechnungen zu begleichen 4, davon fällig 2“).
//   - /m/{module}: statt der ersten Liste die Übersicht des Moduls mit einer Kachel
//     je Tile (z. B. Rechnungen, Aufträge bzw. je Vorgangsart). Kacheln und Zahlen
//     führen in die gefilterte Liste; die Listen bleiben in der Navigation.
//
// Module ohne Dashboard bleiben wie bisher. Schlägt der Abruf fehl (z. B. fehlendes
// Recht), fällt die Seite auf das bisherige Verhalten zurück.

// dashFigure ist eine Zahl einer Kachel, fertig für das Template.
type dashFigure struct {
	Label, Text, Tone, URL string
	Count                  int64
}

// dashTile ist eine Kachel der Übersicht eines Moduls.
type dashTile struct {
	Title, Icon, URL string
	Figures          []dashFigure
}

type dashboardData struct {
	Summary []dashFigure
	Tiles   []dashTile
	At      time.Time // Stand: wann berechnet
}

// Zwischenspeicher (seit 0.68.0): Die Zahlen kommen aus dem Speicher, damit Startseite und
// Übersicht sofort stehen – manche Module fragen für ihren Status andere Module ab (z. B.
// offene Posten im Hauptbuch) und brauchen Sekunden.
//
//   - Je Benutzer, Person, Sprache und Modul ein Eintrag (die Zahlen hängen von den Rechten ab).
//   - Der erste Abruf rechnet; danach liefert der Speicher. Ein Zeitgeber rechnet alle
//     dashboard_refresh (Einstellung, Standard 5m) neu – für alle, die das Dashboard innerhalb von
//     dashboard_idle (Standard 12h) gesehen haben. Ist ein Eintrag beim Abruf älter, wird er im
//     Hintergrund erneuert.
//   - „Aktualisieren“ (?refresh=1) rechnet sofort neu. dashboard_refresh: "0" schaltet den
//     Speicher ab (jeder Abruf rechnet).
//
// Die Seite zeigt den Stand (Uhrzeit der Berechnung).

// dashboardIdle: Einträge, die so lange niemand gesehen hat, rechnet der Zeitgeber nicht mehr
// (Einstellung dashboard_idle, Standard 12h – so steht das Dashboard über den Arbeitstag sofort).
func (s *server) dashboardIdle() time.Duration {
	if d, err := time.ParseDuration(strings.TrimSpace(s.cfg.DashboardIdle)); err == nil && d > 0 {
		return d
	}
	return 12 * time.Hour
}

// dashboardTimeout: längste Dauer einer Berechnung im Hintergrund.
const dashboardTimeout = 2 * time.Minute

// dashJob: was zum Berechnen nötig ist – Aufrufkontext des Benutzers, Sprache, Modul.
type dashJob struct {
	call   sdk.CallContext
	locale string
	module moduleInfo
}

type dashEntry struct {
	job        dashJob
	data       dashboardData
	ok         bool
	seen       time.Time     // zuletzt angezeigt
	refreshing bool          // Neuberechnung im Hintergrund läuft
	first      chan struct{} // erste Berechnung läuft: geschlossen, wenn data steht
}

type dashCache struct {
	mu      sync.Mutex
	entries map[string]*dashEntry
}

// dashboardRefresh: Abstand der Neuberechnung (0 = kein Zwischenspeicher).
func (s *server) dashboardRefresh() time.Duration {
	v := strings.TrimSpace(s.cfg.DashboardRefresh)
	if v == "" {
		return 5 * time.Minute
	}
	if d, err := time.ParseDuration(v); err == nil && d >= 0 {
		return d
	}
	return 5 * time.Minute
}

// dashboard liefert den Status des Moduls aus dem Zwischenspeicher (bzw. rechnet beim
// ersten Abruf und bei ?refresh=1); ok = false: kein Dashboard bzw. nicht abrufbar.
func (s *server) dashboard(r *http.Request, m moduleInfo) (dashboardData, bool) {
	if m.Dashboard == nil {
		return dashboardData{}, false
	}
	job := dashJob{call: s.callContext(r), locale: localeFrom(r), module: m}
	maxAge := s.dashboardRefresh()
	if maxAge == 0 {
		return s.computeDashboard(job)
	}
	persona := ""
	if u := userFrom(r); u != nil {
		persona = u.PersonaID
	}
	key := strings.Join([]string{job.call.TenantID, job.call.UserID, persona, job.locale, m.Name}, "|")
	force := r.Method == http.MethodGet && r.URL.Query().Get("refresh") == "1"

	s.dash.mu.Lock()
	if s.dash.entries == nil {
		s.dash.entries = map[string]*dashEntry{}
	}
	e := s.dash.entries[key]
	if e != nil && !force {
		e.job, e.seen = job, time.Now()
		if first := e.first; first != nil {
			// Die erste Berechnung läuft schon (z. B. zwei Seiten zugleich): darauf warten statt doppelt rechnen.
			s.dash.mu.Unlock()
			<-first
			s.dash.mu.Lock()
		}
		data, ok := e.data, e.ok
		if time.Since(data.At) > maxAge && !e.refreshing {
			e.refreshing = true
			go s.refreshDashboard(key, job)
		}
		s.dash.mu.Unlock()
		return data, ok
	}
	if e == nil {
		e = &dashEntry{first: make(chan struct{})}
		s.dash.entries[key] = e
	}
	e.job, e.seen = job, time.Now()
	s.dash.mu.Unlock()

	data, ok := s.computeDashboard(job)
	s.dash.mu.Lock()
	e.data, e.ok = data, ok
	if e.first != nil {
		close(e.first)
		e.first = nil
	}
	s.dash.mu.Unlock()
	return data, ok
}

// refreshDashboard rechnet einen Eintrag im Hintergrund neu.
func (s *server) refreshDashboard(key string, job dashJob) {
	data, ok := s.computeDashboard(job)
	s.dash.mu.Lock()
	if e := s.dash.entries[key]; e != nil {
		e.data, e.ok, e.refreshing = data, ok, false
	}
	s.dash.mu.Unlock()
}

// dashboardTimer rechnet die Einträge regelmäßig neu (alle dashboard_refresh), solange sie
// innerhalb von dashboard_idle jemand gesehen hat; ältere entfallen.
func (s *server) dashboardTimer(ctx context.Context) {
	every := s.dashboardRefresh()
	if every == 0 {
		return
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		type todo struct {
			key string
			job dashJob
		}
		var list []todo
		s.dash.mu.Lock()
		for k, e := range s.dash.entries {
			switch {
			case time.Since(e.seen) > s.dashboardIdle():
				delete(s.dash.entries, k)
			case !e.refreshing && e.first == nil:
				e.refreshing = true
				list = append(list, todo{k, e.job})
			}
		}
		s.dash.mu.Unlock()
		for _, td := range list {
			if ctx.Err() != nil {
				return
			}
			s.refreshDashboard(td.key, td.job)
		}
	}
}

// computeDashboard ruft den Status beim Modul ab (mit den Rechten des Benutzers).
func (s *server) computeDashboard(job dashJob) (dashboardData, bool) {
	m := job.module
	ctx, cancel := context.WithTimeout(sdk.WithCall(context.Background(), job.call), dashboardTimeout)
	defer cancel()
	fail := func(err error) (dashboardData, bool) {
		_ = s.host.Log(ctx, sdk.LogError, "Dashboard "+m.Name+" fehlgeschlagen", map[string]string{"err": err.Error()})
		return dashboardData{At: time.Now()}, false
	}
	resp, err := s.host.Handle(ctx, sdk.Request{Object: m.Dashboard.Object, Action: m.Dashboard.Action, Payload: map[string]any{}})
	if err != nil {
		return fail(err)
	}
	var in metamodel.DashboardResponse
	if err := sdk.Decode(resp.Payload, &in); err != nil {
		return fail(err)
	}
	dict := s.i18n.dictionary(ctx, job.locale)
	tr := func(key, fallback string) string {
		if v, ok := dict[key]; ok && key != "" {
			return v
		}
		return fallback
	}
	link := func(object string, query map[string]string) string {
		if object == "" || !slices.ContainsFunc(m.Objects, func(o moduleObject) bool { return o.Object == object }) {
			return "" // nicht sichtbar für den Benutzer: Zahl ohne Link
		}
		u := moduleURL(m.Name, object)
		if len(query) > 0 {
			q := url.Values{}
			for k, v := range query {
				q.Set(k, v)
			}
			u += "?" + q.Encode()
		}
		return u
	}
	figures := func(fs []metamodel.DashboardFigure) []dashFigure {
		var out []dashFigure
		for _, f := range fs {
			df := dashFigure{Label: tr(f.LabelKey, f.Label), Text: f.Text, Count: f.Count, URL: link(f.Object, f.Query)}
			if f.Count != 0 {
				df.Tone = f.Tone
			}
			out = append(out, df)
		}
		return out
	}
	out := dashboardData{Summary: figures(in.Summary), At: time.Now()}
	for _, t := range in.Tiles {
		out.Tiles = append(out.Tiles, dashTile{Title: tr(t.TitleKey, t.Title), Icon: t.Icon, URL: link(t.Object, t.Query), Figures: figures(t.Figures)})
	}
	return out, true
}

// homeModule ist ein Modul auf der Startseite mit den Zahlen seines Status.
type homeModule struct {
	moduleInfo
	Summary []dashFigure
	At      time.Time // Stand der Zahlen
}

// homeModules: Module der Startseite; die Dashboards werden nebeneinander abgerufen.
func (s *server) homeModules(r *http.Request) []homeModule {
	mods := s.modules(r)
	out := make([]homeModule, len(mods))
	var wg sync.WaitGroup
	for i, m := range mods {
		out[i].moduleInfo = m
		if m.Dashboard == nil {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if d, ok := s.dashboard(r, m); ok {
				out[i].Summary, out[i].At = d.Summary, d.At
			}
		}()
	}
	wg.Wait()
	return out
}

// Personen auf der Startseite (seit 0.67.0): eine Kachel je Person, die der Benutzer
// einnehmen kann. Ohne gewählte Person (alle Rollen) zeigt sie, was in den Modulen der
// Person zu tun ist – die Zahlen der Module, die Aufmerksamkeit verlangen (Tone attention
// bzw. danger, nicht null), mit dem Namen des Moduls. Ein Klick wählt die Person; die
// Startseite zeigt dann deren Module mit ihrem Status. Mit gewählter Person gelten nur
// deren Rechte – die Kacheln der anderen Personen stehen dann ohne Zahlen zum Wechseln da.

// personaFigureMax: so viele Zahlen zeigt die Kachel einer Person höchstens.
const personaFigureMax = 6

type homePersona struct {
	ID, Name string
	Active   bool
	Figures  []dashFigure
	More     int // weitere Zahlen, die nicht mehr auf die Kachel passen
}

func homePersonas(u *user, mods []homeModule) []homePersona {
	if u == nil || len(u.Personas) == 0 {
		return nil
	}
	out := make([]homePersona, 0, len(u.Personas))
	for _, p := range u.Personas {
		hp := homePersona{ID: p.ID, Name: p.Name, Active: p.ID == u.PersonaID}
		if u.PersonaID == "" {
			for _, m := range mods {
				if !slices.Contains(p.Modules, m.Name) {
					continue
				}
				for _, f := range m.Summary {
					if f.Count == 0 || (f.Tone != metamodel.ToneAttention && f.Tone != metamodel.ToneDanger) {
						continue
					}
					if len(hp.Figures) == personaFigureMax {
						hp.More++
						continue
					}
					f.Label, f.URL = m.Title+": "+f.Label, ""
					hp.Figures = append(hp.Figures, f)
				}
			}
		}
		out = append(out, hp)
	}
	return out
}

// oldestStand: ältester Stand der Zahlen der Startseite ("" = keine Zahlen).
func oldestStand(mods []homeModule) string {
	var at time.Time
	for _, m := range mods {
		if len(m.Summary) > 0 && (at.IsZero() || m.At.Before(at)) {
			at = m.At
		}
	}
	if at.IsZero() {
		return ""
	}
	return at.Format("15:04")
}

// Vorberechnen beim Start (seit 0.69.0): Für alle Benutzer mit gültiger Anmeldung rechnet der
// Server die Dashboards ihrer Module gleich nach dem Start – mit ihrer gewählten Person und der
// Sprache ihres Profils (sonst der Standardsprache). So steht die Startseite auch nach einem
// Neustart sofort. Die Module melden sich nach und nach an: der zweite Durchgang holt nach,
// was beim ersten noch nicht abrufbar war.

// dashboardWarmDelays: Wartezeiten vor den Durchgängen nach dem Start.
var dashboardWarmDelays = []time.Duration{5 * time.Second, 40 * time.Second}

func (s *server) warmDashboards(ctx context.Context) {
	if s.dashboardRefresh() == 0 {
		return
	}
	for _, wait := range dashboardWarmDelays {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		s.warmDashboardsOnce(ctx)
	}
}

// warmDashboardsOnce rechnet, was noch nicht (erfolgreich) im Speicher steht; liefert die Zahl der Berechnungen.
func (s *server) warmDashboardsOnce(ctx context.Context) int {
	users, err := s.auth.ActiveUsers(ctx)
	if err != nil {
		_ = s.host.Log(ctx, sdk.LogWarn, "Dashboards nicht vorberechnet", map[string]string{"err": err.Error()})
		return 0
	}
	n := 0
	for _, u := range users {
		loc := metamodel.NormalizeLocale(u.Locale)
		if loc == "" {
			loc = s.i18n.def
		}
		rctx := context.WithValue(context.WithValue(ctx, userKey{}, u), localeKey{}, loc)
		r, err1 := http.NewRequestWithContext(rctx, http.MethodGet, "/", nil)
		force, err2 := http.NewRequestWithContext(rctx, http.MethodGet, "/?refresh=1", nil)
		if err1 != nil || err2 != nil {
			continue
		}
		for _, m := range s.modules(r) {
			if m.Dashboard == nil {
				continue
			}
			if _, ok := s.dashboard(r, m); !ok {
				s.dashboard(force, m) // z. B. Modul war noch nicht bereit: neu rechnen
			}
			n++
		}
	}
	return n
}
