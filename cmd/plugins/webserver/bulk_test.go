package main

import (
	"net/url"
	"strings"
	"testing"
)

// TestBulkAction (0.55.0): Kästchen je Zeile, Leiste mit der Sammelaktion; die Aktion
// läuft je angekreuzter Zeile, Fehler einer Zeile stehen im Ergebnis, die Liste lädt neu.
func TestBulkAction(t *testing.T) {
	s, h := newTestServer(t, "")
	list := do(s, "GET", "/m/crm/Partner", nil, true).Body.String()
	mustContain(t, list, `data-bulk-bar hidden`, `hx-post="/action/crm/Partner/Approve/_bulk"`,
		`hx-include="#rows input[name=_ids]:checked"`, `data-bulk-all`, `name="_ids" value="p/1"`)

	w := do(s, "POST", "/action/crm/Partner/Approve/_bulk", url.Values{"_ids": {"p/1", "p/9", "p/1"}}, true)
	body := w.Body.String()
	if w.Code != 200 || w.Header().Get("HX-Trigger") != "coremesh-changed" {
		t.Fatalf("Status %d, Trigger %q: %s", w.Code, w.Header().Get("HX-Trigger"), body)
	}
	mustContain(t, body, "1 von 2 ausgeführt", "schon freigegeben", "ACME")
	n := 0
	h.mu.Lock()
	for _, c := range h.calls {
		if c.Object == "Partner" && c.Action == "Approve" {
			n++
		}
	}
	h.mu.Unlock()
	if n != 2 {
		t.Fatalf("Aufrufe: %d (doppelte id nur einmal)", n)
	}

	// ohne Auswahl bzw. Aktion mit Feldern: abgewiesen
	if w := do(s, "POST", "/action/crm/Partner/Approve/_bulk", url.Values{}, true); w.Code == 200 {
		t.Fatal("leere Auswahl angenommen")
	}
	if w := do(s, "POST", "/action/crm/Partner/Remind/_bulk", url.Values{"_ids": {"p/1"}}, true); w.Code == 200 || strings.Contains(w.Body.String(), "ausgeführt") {
		t.Fatal("Aktion mit Feldern als Sammelaktion")
	}
}

func TestBulkActionWithFields(t *testing.T) {
	s, h := newTestServer(t, "")
	list := do(s, "GET", "/m/crm/Partner", nil, true).Body.String()
	mustContain(t, list, `hx-get="/action/crm/Partner/Remind"`)
	form := do(s, "GET", "/action/crm/Partner/Remind?_ids=p/1&_ids=p/9", nil, true).Body.String()
	mustContain(t, form, `name="_ids" value="p/1"`, `name="_ids" value="p/9"`, `name="remind_on"`, "(2 Zeilen)")
	if strings.Contains(form, `name="_id"`) {
		t.Error("_id im Sammelformular")
	}
	w := do(s, "POST", "/action/crm/Partner/Remind", url.Values{"_ids": {"p/1", "p/9"}, "remind_on": {"2026-11-30"}}, true)
	mustContain(t, w.Body.String(), "2 von 2 ausgeführt")
	n := 0
	h.mu.Lock()
	for _, c := range h.calls {
		if c.Object == "Partner" && c.Action == "Remind" && c.Payload.(map[string]any)["data"].(map[string]any)["remind_on"] == "2026-11-30" {
			n++
		}
	}
	h.mu.Unlock()
	if n != 2 {
		t.Fatalf("Aufrufe mit Eingabe: %d", n)
	}
}
