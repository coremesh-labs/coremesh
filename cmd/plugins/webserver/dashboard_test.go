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

	// Dashboard nicht abrufbar: die erste Liste wie bisher
	h.fail = map[string]error{"CrmDashboard.status": sdk.ErrPermissionDenied}
	b = do(s, "GET", "/m/crm/", nil, false).Body.String()
	mustNotContain(t, b, "module-dashboard")
	mustContain(t, b, "ACME")
	b = do(s, "GET", "/", nil, false).Body.String()
	mustNotContain(t, b, `class="figure`)
}
