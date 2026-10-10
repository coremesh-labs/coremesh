package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// TestFileContent: GET /file/{object}/{id} liefert den Inhalt aus der Action
// content – PDF eingebettet, HTML nur als Download, unbekannt → 404.
func TestFileContent(t *testing.T) {
	s, h := newMDServer(t)
	w := do(s, "GET", "/file/Customer/c1", nil, false)
	if w.Code != http.StatusOK || w.Body.String() != "%PDF-1.4 x" {
		t.Fatalf("PDF: %d %q", w.Code, w.Body.String())
	}
	if ct, cd := w.Header().Get("Content-Type"), w.Header().Get("Content-Disposition"); ct != "application/pdf" ||
		cd != `inline; filename="11.1:VV:Vertrag.pdf"` || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("Header: %q %q", ct, cd)
	}
	if w.Header().Get("X-Frame-Options") != "SAMEORIGIN" || w.Header().Get("Content-Security-Policy") != "frame-ancestors 'self'" {
		t.Fatalf("Einbetten nicht erlaubt: %v", w.Header())
	}
	if p := h.find("content").Payload.(map[string]any); p["id"] != "c1" {
		t.Fatalf("Payload: %v", p)
	}
	if cd := do(s, "GET", "/file/Customer/c1?download=1", nil, false).Header().Get("Content-Disposition"); cd != `attachment; filename="11.1:VV:Vertrag.pdf"` {
		t.Fatalf("Download: %q", cd)
	}
	w = do(s, "GET", "/file/Customer/html", nil, false)
	if ct, cd := w.Header().Get("Content-Type"), w.Header().Get("Content-Disposition"); ct != "application/octet-stream" || cd != `attachment; filename=x.html` {
		t.Fatalf("HTML nur als Download: %q %q", ct, cd)
	}
	if w := do(s, "GET", "/file/Customer/fehlt", nil, false); w.Code != http.StatusNotFound {
		t.Fatalf("unbekannt: %d", w.Code)
	}
}

// TestViewerSection: Abschnitt Viewer bettet /file/{object}/{id} ein.
func TestViewerSection(t *testing.T) {
	withDef(t, "Customer", func(d *metamodel.ObjectDefinition) {
		d.Sections = append(slices.Clone(d.Sections), metamodel.SectionDefinition{Key: "dokument", Title: "Dokument", Viewer: true})
	})
	s, _ := newMDServer(t)
	b := do(s, "GET", "/m/crm/Customer/c1", nil, true).Body.String()
	mustContain(t, b, `<iframe src="/file/Customer/c1" title="Dokument"`, `href="/file/Customer/c1?download=1"`)
}

// TestBinaryUpload: Dateifeld mit Binary – Inhalt Base64, Name und MIME-Typ.
func TestBinaryUpload(t *testing.T) {
	withDef(t, "CustomerContact", func(d *metamodel.ObjectDefinition) {
		d.Fields = append(d.Fields, metamodel.FieldDefinition{Key: "file", Label: "Datei", Type: metamodel.TypeFile, Editable: true,
			Required: true, ActionOnly: true, Binary: true})
		d.Actions = append(slices.Clone(d.Actions),
			metamodel.ActionConfig{Name: "upload", Kind: metamodel.KindCustom, Label: "Hochladen …", Fields: []string{"file"}})
	})
	s, h := newMDServer(t)
	content := []byte{'%', 'P', 'D', 'F', '-', 0xfc, 0x00, 0xff}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "scan.pdf")
	_, _ = fw.Write(content)
	mw.Close()
	r := httptest.NewRequest("POST", "/action/crm/CustomerContact/upload", &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r.Header.Set("HX-Request", "true")
	if tok, ok := testTokens.Load(s); ok {
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: tok.(string)})
	}
	s.ServeHTTP(httptest.NewRecorder(), r)
	data := h.find("upload").Payload.(map[string]any)["data"].(map[string]any)
	got, err := base64.StdEncoding.DecodeString(data["file"].(string))
	if err != nil || !bytes.Equal(got, content) || data["file_name"] != "scan.pdf" || data["file_type"] != "application/pdf" {
		t.Fatalf("Daten: %v (%v)", data, err)
	}
}

// TestDynamicSelect: Auswahlfeld ohne feste Werte (Werte aus FormState) – der
// WebServer nimmt den Wert an, das Modul prüft.
func TestDynamicSelect(t *testing.T) {
	withDef(t, "CustomerContact", func(d *metamodel.ObjectDefinition) {
		d.FormState = "formState"
		d.Fields = append(d.Fields, metamodel.FieldDefinition{Key: "choice", Label: "Auswahl", Type: metamodel.TypeSelect, Editable: true, ActionOnly: true})
		d.Actions = append(slices.Clone(d.Actions),
			metamodel.ActionConfig{Name: "pick", Kind: metamodel.KindCustom, Label: "Wählen …", Fields: []string{"choice"}})
	})
	s, h := newMDServer(t)
	do(s, "POST", "/action/crm/CustomerContact/pick", url.Values{"choice": {"3.0.1"}}, true)
	if d := h.find("pick").Payload.(map[string]any)["data"].(map[string]any); d["choice"] != "3.0.1" {
		t.Fatalf("Daten: %v", d)
	}
}

// TestActionFormState: Aktionsformular mit FormState – Auswahlwerte,
// ausgeblendete Felder, Hinweis, Links (nur Pfade der Anwendung), Neuaufbau
// über ein Trigger-Feld; /new/{object} leitet zum Formular „Neu“.
func TestActionFormState(t *testing.T) {
	withDef(t, "CustomerContact", func(d *metamodel.ObjectDefinition) {
		d.FormState = "formState"
		d.Fields = append(d.Fields,
			metamodel.FieldDefinition{Key: "search", Label: "Suche", Type: metamodel.TypeText, Editable: true, ActionOnly: true, Trigger: true},
			metamodel.FieldDefinition{Key: "choice", Label: "Auswahl", Type: metamodel.TypeSelect, Editable: true, ActionOnly: true})
		d.Actions = append(slices.Clone(d.Actions),
			metamodel.ActionConfig{Name: "pick", Kind: metamodel.KindCustom, Label: "Wählen …", FormState: true, Fields: []string{"search", "choice", "value"}})
	})
	s, _ := newMDServer(t)
	b := do(s, "GET", "/action/crm/CustomerContact/pick?id=k1", nil, true).Body.String()
	mustContain(t, b, `<option value="A"`, `>Beta</option>`, "Welches Objekt?", `<a href="/new/Contract" target="_blank" rel="noopener">Vertrag anlegen</a>`,
		`hx-post="/action/crm/CustomerContact/pick/_form"`)
	if strings.Contains(b, "javascript:") || strings.Contains(b, `name="value"`) {
		t.Fatalf("Link oder ausgeblendetes Feld:\n%s", b)
	}
	b = do(s, "POST", "/action/crm/CustomerContact/pick/_form", url.Values{"_id": {"k1"}, "search": {"be"}}, true).Body.String()
	mustContain(t, b, `>Beta</option>`, `value="be"`)
	if strings.Contains(b, ">Alpha<") {
		t.Fatalf("Neuaufbau filtert nicht:\n%s", b)
	}
	w := do(s, "GET", "/new/Customer", nil, false)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/m/crm/Customer/new" {
		t.Fatalf("/new: %d %q", w.Code, w.Header().Get("Location"))
	}
}

// TestActionSection: Abschnitt mit eingebettetem Aktionsformular – lädt das
// Formular inline, Absenden ersetzt den Abschnitt (Fehler) bzw. lädt die
// Detailansicht neu (HX-Location); ausgeblendet per _hidden_sections.
func TestActionSection(t *testing.T) {
	withDef(t, "Customer", func(d *metamodel.ObjectDefinition) {
		d.Fields = append(slices.Clone(d.Fields), metamodel.FieldDefinition{Key: "answer", Label: "Antwort", Type: metamodel.TypeText, Editable: true, ActionOnly: true})
		d.Actions = append(slices.Clone(d.Actions), metamodel.ActionConfig{Name: "step", Kind: metamodel.KindCustom, Label: "Weiter …", Record: true, Fields: []string{"answer"}})
		d.Sections = append([]metamodel.SectionDefinition{{Key: "entscheidung", Title: "Entscheidung", Action: "step"}}, d.Sections...)
	})
	s, h := newMDServer(t)
	b := do(s, "GET", "/m/crm/Customer/c1", nil, true).Body.String()
	mustContain(t, b, `class="action-section" hx-get="/action/crm/Customer/step?inline=1&amp;id=c1"`)
	b = do(s, "GET", "/action/crm/Customer/step?inline=1&id=c1", nil, true).Body.String()
	mustContain(t, b, `hx-post="/action/crm/Customer/step" hx-target="closest .action-section"`, `name="_inline" value="1"`, `name="answer"`,
		`<button type="submit" class="btn primary">Weiter</button>`)
	if strings.Contains(b, "<dialog") {
		t.Fatal("eingebettet, kein Dialog")
	}
	w := do(s, "POST", "/action/crm/Customer/step", url.Values{"_inline": {"1"}, "_id": {"c1"}, "answer": {"ja"}}, true)
	if loc := w.Header().Get("HX-Location"); loc != `{"path":"/m/crm/Customer/c1","target":"#main-content"}` {
		t.Fatalf("HX-Location: %q", loc)
	}
	if d := h.find("step").Payload.(map[string]any); d["id"] != "c1" || d["data"].(map[string]any)["answer"] != "ja" {
		t.Fatalf("Payload: %v", d)
	}
	h.mu.Lock()
	h.fail["Customer.step"] = fmt.Errorf("%w: Wert passt nicht", sdk.ErrInvalidArgument)
	h.mu.Unlock()
	b = do(s, "POST", "/action/crm/Customer/step", url.Values{"_inline": {"1"}, "_id": {"c1"}, "answer": {"x"}}, true).Body.String()
	mustContain(t, b, "Wert passt nicht", `value="x"`, `class="action-inline"`)
	customerMarkers.Store(map[string]any{"_hidden_sections": []any{"entscheidung"}})
	t.Cleanup(func() { customerMarkers.Store(map[string]any{}) })
	if b = do(s, "GET", "/m/crm/Customer/c1", nil, true).Body.String(); strings.Contains(b, "action-section") {
		t.Fatal("ausgeblendeter Abschnitt")
	}
}
