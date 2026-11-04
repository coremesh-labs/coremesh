package businesspartner

import (
	"fmt"
	"strings"
	"testing"
)

// TestPasteContact (0.17.0): Kontaktdaten aus kopiertem Text übernehmen.
func TestPasteContact(t *testing.T) {
	e := setup(t)
	id := e.newBP("Auerbachs Keller GmbH")
	text := "Auerbachs Keller GmbH\nGrimmaische Str. 2-4\nD-04109 Leipzig\n\nTelefon: 0341 - 21 61 00\nTelefax: 0341 - 21 61 011\nMobil 0172 1234567\nE-Mail: Keller@Auerbach.example\nwww.auerbach.example\nGeöffnet täglich"
	tableOf := func(r map[string]any) string {
		var b strings.Builder
		for _, row := range r["table"].(map[string]any)["rows"].([][]any) {
			fmt.Fprintln(&b, row...)
		}
		return b.String()
	}
	// Nur prüfen: nichts gespeichert
	r := e.must("BusinessPartner", "pasteContact", map[string]any{"id": id, "data": map[string]any{"paste_text": text, "paste_check": true}})
	if got := tableOf(r); !strings.Contains(got, "Grimmaische Str. 2-4, 04109 Leipzig (DE) wird angelegt") || len(e.items("PartnerContact", map[string]any{"bp_id": id})) != 0 {
		t.Fatalf("Prüfung: %v\n%s", r["message"], got)
	}
	r = e.must("BusinessPartner", "pasteContact", map[string]any{"id": id, "data": map[string]any{"paste_text": text}})
	got := tableOf(r)
	for _, want := range []string{"Name Auerbachs Keller GmbH passt", "Telefon geschäftlich 0341 21 61 00 angelegt (Standard)", "Fax geschäftlich 0341 21 61 011 angelegt (Standard)",
		"Mobiltelefon 0172 1234567 angelegt\n", "E-Mail geschäftlich keller@auerbach.example angelegt (Standard)", "Webseite https://www.auerbach.example angelegt (Standard)",
		"nicht erkannt Geöffnet täglich"} {
		if !strings.Contains(got, want) {
			t.Fatalf("fehlt %q:\n%s", want, got)
		}
	}
	addr := e.items("PartnerAddressData", map[string]any{})
	if len(addr) != 1 || addr[0]["street"] != "Grimmaische Str." || addr[0]["house_no"] != "2-4" || addr[0]["zip_code"] != "04109" || addr[0]["country"] != "DE" {
		t.Fatalf("Adresse: %v", addr)
	}
	if n := len(e.items("PartnerContact", map[string]any{"bp_id": id})); n != 5 {
		t.Fatalf("Kommunikation: %d", n)
	}
	// Zweites Mal: alles vorhanden; andere Anschrift und anderer Name nur gemeldet
	r = e.must("BusinessPartner", "pasteContact", map[string]any{"id": id, "data": map[string]any{"paste_text": "Mephisto AG\nHexenküche 1\n1010 Wien\nÖsterreich\nTel.: 0341/216100"}})
	got = tableOf(r)
	for _, want := range []string{"weicht ab von „Auerbachs Keller GmbH“", "Hexenküche 1, 1010 Wien (AT) weicht ab", "0341/216100 schon vorhanden"} {
		if !strings.Contains(got, want) {
			t.Fatalf("fehlt %q:\n%s", want, got)
		}
	}
	// Erkennungswörter aus dem Katalog
	e.must("PartnerCommType", "update", map[string]any{"id": "MOBILE|1900-01-01", "data": map[string]any{"keywords": "Mobil ,Handy,  Diensthandy "}})
	if k := e.must("PartnerCommType", "get", map[string]any{"id": "MOBILE|1900-01-01"})["keywords"]; k != "Mobil, Handy, Diensthandy" {
		t.Fatalf("Erkennungswörter: %v", k)
	}
	r = e.must("BusinessPartner", "pasteContact", map[string]any{"id": id, "data": map[string]any{"paste_text": "Diensthandy: 0160 7654321", "paste_check": true}})
	if got := tableOf(r); !strings.Contains(got, "Mobiltelefon 0160 7654321 wird angelegt") {
		t.Fatalf("Diensthandy:\n%s", got)
	}
	_, err := e.do("PartnerDialCode", "update", map[string]any{"id": "DE", "data": map[string]any{"zip_pattern": "(\\d{5"}})
	if err == nil {
		t.Fatal("ungültiger PLZ-Aufbau angenommen")
	}
}
