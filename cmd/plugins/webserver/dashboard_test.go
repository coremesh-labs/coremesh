package main

import (
	"maps"
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
)

// TestDashboard: Zahlen an der Kachel der Startseite und Übersicht des Moduls;
// ohne Dashboard bzw. bei einem Fehler bleibt es beim bisherigen Verhalten.
func TestDashboard(t *testing.T) {
	plain := maps.Clone(crmModule)
	crmModule["dashboard"] = map[string]any{"object": "CrmDashboard", "action": "status"}
	t.Cleanup(func() { crmModule = plain })

	s, h := newTestServer(t, "")
	b := do(s, "GET", "/", nil, false).Body.String()
	mustContain(t, b, `<span class="figure tone-attention"><b>4</b> zu begleichen</span>`, `<span class="figure zero"><b>0</b> fällig</span>`)

	b = do(s, "GET", "/m/crm/", nil, false).Body.String()
	mustContain(t, b, `class="home module-dashboard"`, `<a class="tile-title" href="/m/crm/Partner">`,
		`<a class="figure tone-danger" href="/m/crm/Partner?kind=customer"><b>7</b> CRM <em>12,50 EUR</em></a>`, // Text übersetzt (LabelKey)
		`<span class="figure"><b>1</b> fremd</span>`)                                                            // Object nicht im Modul: ohne Link
	// Die Listen bleiben erreichbar.
	if w := do(s, "GET", "/m/crm/Partner", nil, false); w.Code != 200 {
		t.Fatalf("Liste: %d", w.Code)
	}

	// Zwischenspeicher: Startseite und Übersicht haben zusammen einmal gerechnet; der Stand steht auf der Seite.
	calls := func() int {
		h.mu.Lock()
		defer h.mu.Unlock()
		n := 0
		for _, c := range h.calls {
			if c.Object == "CrmDashboard" {
				n++
			}
		}
		return n
	}
	do(s, "GET", "/", nil, false)
	if n := calls(); n != 1 {
		t.Fatalf("Abrufe beim Modul: %d statt 1", n)
	}
	mustContain(t, b, `class="stand"`, `href="/m/crm/?refresh=1"`)
	// „Aktualisieren“ rechnet neu
	do(s, "GET", "/m/crm/?refresh=1", nil, false)
	if n := calls(); n != 2 {
		t.Fatalf("nach Aktualisieren: %d Abrufe", n)
	}

	// Dashboard nicht abrufbar: aus dem Speicher weiter die alten Zahlen, nach der Neuberechnung die erste Liste wie bisher
	h.mu.Lock()
	h.fail = map[string]error{"CrmDashboard.status": sdk.ErrPermissionDenied}
	h.mu.Unlock()
	mustContain(t, do(s, "GET", "/m/crm/", nil, false).Body.String(), "module-dashboard")
	b = do(s, "GET", "/m/crm/?refresh=1", nil, false).Body.String()
	mustNotContain(t, b, "module-dashboard")
	mustContain(t, b, "ACME")
	b = do(s, "GET", "/", nil, false).Body.String()
	mustNotContain(t, b, `class="figure`)

	// dashboard_refresh "0": ohne Speicher rechnet jeder Abruf
	h.mu.Lock()
	h.fail = nil
	h.mu.Unlock()
	s.cfg.DashboardRefresh = "0"
	n := calls()
	do(s, "GET", "/m/crm/", nil, false)
	do(s, "GET", "/m/crm/", nil, false)
	if got := calls() - n; got != 2 {
		t.Fatalf("ohne Speicher: %d Abrufe statt 2", got)
	}
}

// TestHomePersonas: Kachel je Person – ohne gewählte Person mit den Zahlen ihrer Module,
// die Aufmerksamkeit verlangen; mit gewählter Person ohne Zahlen (nur deren Rechte gelten).
func TestHomePersonas(t *testing.T) {
	mods := []homeModule{
		{moduleInfo: moduleInfo{Name: "bank", Title: "Bank"}, Summary: []dashFigure{{Label: "nicht zugeordnet", Count: 3, Tone: "danger", URL: "/m/bank/X"}, {Label: "gebucht", Count: 9}}},
		{moduleInfo: moduleInfo{Name: "mail", Title: "Posteingang"}, Summary: []dashFigure{{Label: "offen", Count: 0, Tone: "attention"}}},
		{moduleInfo: moduleInfo{Name: "crm", Title: "CRM"}, Summary: []dashFigure{{Label: "neu", Count: 2, Tone: "attention"}}},
	}
	u := &user{Personas: []persona{{ID: "p1", Name: "Kontoauszüge", Modules: []string{"bank", "mail"}}, {ID: "p2", Name: "Administration"}}}
	got := homePersonas(u, mods)
	if len(got) != 2 || len(got[0].Figures) != 1 || got[0].Figures[0].Label != "Bank: nicht zugeordnet" || got[0].Figures[0].Count != 3 || got[0].Figures[0].URL != "" {
		t.Fatalf("Kachel der Person: %+v", got)
	}
	if len(got[1].Figures) != 0 || got[1].Active {
		t.Fatalf("Person ohne Module im Menü: %+v", got[1])
	}
	u.PersonaID = "p1"
	if got = homePersonas(u, mods); !got[0].Active || len(got[0].Figures) != 0 {
		t.Fatalf("gewählte Person: %+v", got)
	}
	if homePersonas(&user{}, mods) != nil {
		t.Fatal("ohne Personen keine Kacheln")
	}
}
