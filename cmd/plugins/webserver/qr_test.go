package main

import (
	"strings"
	"testing"
)

func TestQRCodesOf(t *testing.T) {
	got := qrCodesOf(map[string]any{"qr": []any{
		map[string]any{"text": "tel:+493414801099", "caption": "Telefon geschäftlich"},
		map[string]any{"text": ""}, // ohne Text: übergangen
		map[string]any{"text": "BEGIN:VCARD\r\nEND:VCARD\r\n", "level": "l"},
	}})
	if len(got) != 2 || got[0].Caption != "Telefon geschäftlich" || !strings.HasPrefix(string(got[0].SVG), "<svg ") || got[1].Text == "" {
		t.Fatalf("QR-Codes: %+v", got)
	}
	if qrCodesOf("text") != nil || qrCodesOf(map[string]any{}) != nil {
		t.Fatal("ohne QR-Codes")
	}
}

// Links aus "_links": nur erlaubte Schemata, Text immer escaped.
func TestLinked(t *testing.T) {
	rec := record{"_links": map[string]any{"tel": "tel:+493414801099", "mail": "mailto:a@b.de", "web": "https://x.de",
		"js": "javascript:alert(1)", "data": "data:text/html,x", "quote": `tel:1" onclick="x`}}
	for key, want := range map[string]string{
		"tel":   `<a href="tel:+493414801099">0341 &lt;1&gt;</a>`,
		"mail":  `<a href="mailto:a@b.de">0341 &lt;1&gt;</a>`,
		"web":   `<a href="https://x.de" target="_blank" rel="noopener noreferrer">0341 &lt;1&gt;</a>`,
		"js":    `0341 &lt;1&gt;`,
		"data":  `0341 &lt;1&gt;`,
		"quote": `0341 &lt;1&gt;`,
		"none":  `0341 &lt;1&gt;`,
	} {
		if got := string(linked(rec, key, "0341 <1>")); got != want {
			t.Errorf("%s: %s", key, got)
		}
	}
	if got := linked(rec, "tel", ""); got != "" {
		t.Errorf("leer: %s", got)
	}
}

// TestTheme: Farbschema der Installation an Seite und Anmeldung; ohne theme kein Attribut.
func TestTheme(t *testing.T) {
	s, _ := newTestServer(t, "")
	if b := do(s, "GET", "/", nil, false).Body.String(); strings.Contains(b, "data-theme") {
		t.Fatal("ohne theme: kein data-theme")
	}
	s.cfg.Theme = "green"
	if b := do(s, "GET", "/", nil, false).Body.String(); !strings.Contains(b, `<html lang="de" data-theme="green">`) {
		t.Fatalf("Seite ohne Farbschema: %.300s", b)
	}
	if b := doAnon(s, "GET", "/login", nil, false).Body.String(); !strings.Contains(b, `data-theme="green"`) {
		t.Fatalf("Anmeldung ohne Farbschema: %.300s", b)
	}
	for _, v := range []string{"green", "dark-blue", "x1"} {
		if !themeRe.MatchString(v) {
			t.Errorf("%q abgelehnt", v)
		}
	}
	for _, v := range []string{"Green\"", "a b", "<x>", ""} {
		if themeRe.MatchString(v) {
			t.Errorf("%q angenommen", v)
		}
	}
}

// TestCopyFieldsOf (0.53.0): Werte mit Knopf „Kopieren“; ohne Wert entfallen, copy leer = Wert.
func TestCopyFieldsOf(t *testing.T) {
	got := copyFieldsOf(map[string]any{"copy": []any{
		map[string]any{"label": "IBAN", "value": "DE89 3704 0044 0532 0130 00", "copy": "DE89370400440532013000"},
		map[string]any{"label": "leer", "value": " "},
		map[string]any{"label": "Verwendungszweck", "value": "RE 4711"},
	}})
	if len(got) != 2 || got[0].Copy != "DE89370400440532013000" || got[1].Copy != "RE 4711" {
		t.Fatalf("Felder: %+v", got)
	}
	if copyFieldsOf(nil) != nil {
		t.Fatal("ohne Felder")
	}
}
