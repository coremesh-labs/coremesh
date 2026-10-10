package main

import (
	"bytes"
	"encoding/base64"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"

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
