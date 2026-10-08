package main

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// TestActionFileUpload: Dateifeld im Aktionsformular – multipart, Inhalt als
// Text und Dateiname an die Aktion; Latin-1 wird nach UTF-8 gelesen.
func TestActionFileUpload(t *testing.T) {
	withDef(t, "CustomerContact", func(d *metamodel.ObjectDefinition) {
		d.Fields = append(d.Fields, metamodel.FieldDefinition{Key: "file", Label: "Datei", Type: metamodel.TypeFile, Editable: true,
			Required: true, ActionOnly: true})
		d.Actions = append(slices.Clone(d.Actions),
			metamodel.ActionConfig{Name: "upload", Kind: metamodel.KindCustom, Label: "Einlesen …", Fields: []string{"value", "file"}})
	})
	s, h := newMDServer(t)
	b := do(s, "GET", "/action/crm/CustomerContact/upload", nil, true).Body.String()
	mustContain(t, b, `enctype="multipart/form-data"`, `hx-encoding="multipart/form-data"`, `type="file" name="file"`)

	send := func(content []byte) *httptest.ResponseRecorder {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		_ = mw.WriteField("value", "x")
		if content != nil {
			fw, _ := mw.CreateFormFile("file", "/tmp/umsaetze.csv")
			_, _ = fw.Write(content)
		}
		mw.Close()
		r := httptest.NewRequest("POST", "/action/crm/CustomerContact/upload", &body)
		r.Header.Set("Content-Type", mw.FormDataContentType())
		r.Header.Set("HX-Request", "true")
		if tok, ok := testTokens.Load(s); ok {
			r.AddCookie(&http.Cookie{Name: sessionCookie, Value: tok.(string)})
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	send([]byte("Buchungstag;Betrag\n01.01.2026;12,50\n"))
	data := h.find("upload").Payload.(map[string]any)["data"].(map[string]any)
	if data["file"] != "Buchungstag;Betrag\n01.01.2026;12,50\n" || data["file_name"] != "umsaetze.csv" || data["value"] != "x" {
		t.Fatalf("Daten: %v", data)
	}
	send([]byte{'M', 0xfc, 'l', 'l', 'e', 'r'}) // Latin-1 „Müller“
	if d := h.find("upload").Payload.(map[string]any)["data"].(map[string]any); d["file"] != "Müller" {
		t.Fatalf("Latin-1: %q", d["file"])
	}
	uploads := func() (n int) {
		for _, c := range h.calls {
			if c.Action == "upload" {
				n++
			}
		}
		return n
	}
	n := uploads()
	if w := send(nil); uploads() != n || w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ohne Datei: Code %d, Aufrufe %d → %d", w.Code, n, uploads())
	}
}
