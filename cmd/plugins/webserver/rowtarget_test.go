package main

import (
	"net/url"
	"strings"
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// TestRowTargets (0.54.0): Aktionen in der Listenzeile, danach nur die Zeile bzw. der
// Abschnitt neu (HX-Trigger mit id), Zeile einzeln abrufbar.
func TestRowTargets(t *testing.T) {
	s, _ := newTestServer(t, "")
	list := do(s, "GET", "/m/crm/Partner", nil, true).Body.String()
	mustContain(t, list,
		`hx-get="/action/crm/Partner/Remind?id=p/1&row=702f31"`,
		`hx-get="/m/crm/Partner/p%2F1?view=row" hx-trigger="coremesh-row-changed[detail.id=='702f31'] from:body"`)
	row := do(s, "GET", "/m/crm/Partner/p%2F1?view=row", nil, true).Body.String()
	mustContain(t, row, `<tr id="row-702f31"`)
	mustNotContain(t, row, `<section id="detail"`)

	form := do(s, "GET", "/action/crm/Partner/Remind?id=p%2F1&row=702f31", nil, true).Body.String()
	mustContain(t, form, `name="_row" value="702f31"`)
	w := do(s, "POST", "/action/crm/Partner/Notify", url.Values{"_id": {"p/1"}, "_row": {"702f31"}, "company_name": {"x"}}, true)
	if got := w.Header().Get("HX-Trigger"); got != `{"coremesh-row-changed":{"id":"702f31"}}` {
		t.Fatalf("Zeile: %q", got)
	}
	w = do(s, "POST", "/action/crm/Partner/Notify", url.Values{"_id": {"p/1"}, "_section": {"rel-kontakte"}, "company_name": {"x"}}, true)
	if got := w.Header().Get("HX-Trigger"); !strings.Contains(got, `"coremesh-section-changed":{"id":"rel-kontakte"}`) || !strings.Contains(got, "coremesh-head-changed") {
		t.Fatalf("Abschnitt: %q", got)
	}
	w = do(s, "POST", "/action/crm/Partner/Notify", url.Values{"_id": {"p/1"}, "_row": {"x\"><script>"}, "company_name": {"x"}}, true)
	if got := w.Header().Get("HX-Trigger"); got != "coremesh-changed, coremesh-record-changed" {
		t.Fatalf("ungültige Zeile: %q", got)
	}
}

func TestTone(t *testing.T) {
	f := metamodel.FieldDefinition{Key: "status", Type: metamodel.TypeSelect, Options: []metamodel.Option{
		{Value: "OPEN", Label: "offen", Tone: metamodel.ToneDanger}, {Value: "PROPOSED", Label: "Vorschlag (bestätigen)", Short: "Vorschlag", Tone: metamodel.ToneAttention},
		{Value: "DONE", Label: "erledigt"}}}
	d := metamodel.ObjectDefinition{Fields: []metamodel.FieldDefinition{{Key: "name", Type: metamodel.TypeText}, f}}
	if got := string(selectCell(record{"status": "PROPOSED"}, f)); got != `<span class="badge tone-attention" title="Vorschlag (bestätigen)">Vorschlag</span>` {
		t.Fatalf("Kennzeichen: %s", got)
	}
	if rowTone(record{"status": "OPEN"}, d) != "danger" || rowTone(record{"status": "DONE"}, d) != "" {
		t.Fatal("Zeilenfarbe")
	}
}
