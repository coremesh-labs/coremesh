package main

import (
	"net/http"
	"net/url"
	"slices"
	"sync"

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
}

// dashboard ruft den Status des Moduls ab; ok = false: kein Dashboard bzw. nicht abrufbar.
func (s *server) dashboard(r *http.Request, m moduleInfo) (dashboardData, bool) {
	if m.Dashboard == nil {
		return dashboardData{}, false
	}
	resp, err := s.call(r, m.Dashboard.Object, m.Dashboard.Action, map[string]any{})
	if err != nil {
		s.logError(r, "Dashboard "+m.Name, err)
		return dashboardData{}, false
	}
	var in metamodel.DashboardResponse
	if err := sdk.Decode(resp.Payload, &in); err != nil {
		s.logError(r, "Dashboard "+m.Name, err)
		return dashboardData{}, false
	}
	dict := s.i18n.dictionary(r.Context(), localeFrom(r))
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
	out := dashboardData{Summary: figures(in.Summary)}
	for _, t := range in.Tiles {
		out.Tiles = append(out.Tiles, dashTile{Title: tr(t.TitleKey, t.Title), Icon: t.Icon, URL: link(t.Object, t.Query), Figures: figures(t.Figures)})
	}
	return out, true
}

// homeModule ist ein Modul auf der Startseite mit den Zahlen seines Status.
type homeModule struct {
	moduleInfo
	Summary []dashFigure
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
				out[i].Summary = d.Summary
			}
		}()
	}
	wg.Wait()
	return out
}
