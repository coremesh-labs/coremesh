package businesspartner

import (
	"strings"
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

func TestInternational(t *testing.T) {
	de, ch, it := dialing{"49", "0"}, dialing{"41", "0"}, dialing{"39", ""}
	for _, c := range []struct {
		in       string
		d        dialing
		ok       bool
		dial, ds string
	}{
		{"0341 216100", de, true, "+49341216100", "+49 341 216100"},
		{"03581/739074", de, true, "+493581739074", "+49 3581 739074"},
		{"0151 525 79 333", de, true, "+4915152579333", "+49 151 525 79 333"},
		{"+49 (0)341 216-100", de, true, "+49341216100", "+49 341 216 100"},
		{"0041 44 123 45 67", de, true, "+41441234567", "+41 44 123 45 67"},
		{"044 123 45 67", ch, true, "+41441234567", "+41 44 123 45 67"},
		{"06 1234 5678", it, true, "+390612345678", "+39 06 1234 5678"},
		{"0341 216100", dialing{}, false, "0341216100", "0341 216100"},
	} {
		dial, ds := international(c.in, c.d, c.ok)
		if dial != c.dial || ds != c.ds {
			t.Errorf("%q: %q %q", c.in, dial, ds)
		}
	}
}

// TestContactQR: Links in der Kommunikation, „Anrufen (QR)“ und „Kontakt aufs Handy (QR)“.
func TestContactQR(t *testing.T) {
	e := setup(t)
	if r := e.must("PartnerDialCode", "get", map[string]any{"id": "DE"}); r["dial_code"] != "49" || r["trunk_prefix"] != "0" || r["is_main"] != true {
		t.Fatalf("Ländervorwahl DE: %v", r)
	}
	_, err := e.do("PartnerDialCode", "create", data("country", "LI", "description", "Liechtenstein", "dial_code", "423", "is_main", true))
	expect(t, err, sdk.ErrInvalidArgument, "zweites Heimatland")
	_, err = e.do("PartnerDialCode", "create", data("country", "LIE", "description", "x", "dial_code", "423"))
	expect(t, err, sdk.ErrInvalidArgument, "Land mit drei Buchstaben")
	e.must("PartnerDialCode", "create", data("country", "li", "description", "Liechtenstein", "dial_code", "+423"))

	id := e.newBP("Auerbachs Keller; Mephisto & Co.")
	addr := e.must("PartnerAddressData", "create", data("street", "Grimmaische Str.", "house_no", "2-4", "zip_code", "04109", "city", "Leipzig", "country", "DE"))
	e.must("PartnerAddress", "create", data("bp_id", id, "address_id", addr["id"], "address_role_code", "MAIN", "is_default", true))
	for _, c := range [][2]string{{"PHONE_WORK", "0341 216100"}, {"MOBILE", "0172 1234567"}, {"EMAIL_WORK", "keller@auerbach.example"}} {
		e.must("PartnerContact", "create", data("bp_id", id, "comm_type_code", c[0], "value", c[1], "is_default", true))
	}
	links := map[string]string{}
	for _, c := range e.items("PartnerContact", map[string]any{"bp_id": id}) {
		l, _ := c["_links"].(map[string]any)
		links[c["comm_type_code"].(string)], _ = l["value"].(string)
	}
	if links["PHONE_WORK"] != "tel:+49341216100" || links["MOBILE"] != "tel:+491721234567" || links["EMAIL_WORK"] != "mailto:keller@auerbach.example" {
		t.Fatalf("Links: %v", links)
	}

	call := e.must("BusinessPartner", "qrCall", map[string]any{"id": id + "|2000-01-01"})
	qr, _ := call["qr"].([]any)
	if len(qr) != 2 || qr[0].(map[string]any)["text"] != "tel:+491721234567" && qr[1].(map[string]any)["text"] != "tel:+491721234567" {
		t.Fatalf("Anrufen: %v", call)
	}
	for _, q := range qr {
		if c := q.(map[string]any)["caption"].(string); !strings.Contains(c, ": +49 ") {
			t.Fatalf("Beschriftung: %q", c)
		}
	}

	// Vorlage der Kategorie: nur die Nummer (Scanner, die tel: nur als Text zeigen)
	e.must("PartnerCommCategory", "update", map[string]any{"id": "PHONE", "data": map[string]any{"qr_template": "{number}"}})
	call = e.must("BusinessPartner", "qrCall", map[string]any{"id": id})
	if qr := call["qr"].([]any); len(qr) != 2 || qr[0].(map[string]any)["text"] != "+491721234567" && qr[1].(map[string]any)["text"] != "+491721234567" {
		t.Fatalf("Vorlage {number}: %v", call)
	}
	e.must("PartnerCommCategory", "update", map[string]any{"id": "PHONE", "data": map[string]any{"qr_template": ""}})
	if r := e.must("BusinessPartner", "qrCall", map[string]any{"id": id}); r["qr"] != nil {
		t.Fatalf("ohne Vorlage: %v", r)
	}

	card := e.must("BusinessPartner", "qrContact", map[string]any{"id": id})
	text := card["qr"].([]any)[0].(map[string]any)["text"].(string)
	for _, want := range []string{"BEGIN:VCARD\r\nVERSION:3.0\r\n", `FN:Auerbachs Keller\; Mephisto & Co.` + "\r\n", `ORG:Auerbachs Keller\; Mephisto & Co.` + "\r\n",
		"TEL;TYPE=WORK,VOICE:+49 341 216100\r\n", "TEL;TYPE=CELL:+49 172 1234567\r\n", "EMAIL;TYPE=WORK:keller@auerbach.example\r\n",
		"ADR;TYPE=WORK:;;Grimmaische Str. 2-4;Leipzig;;04109;DE\r\n", "END:VCARD\r\n"} {
		if !strings.Contains(text, want) {
			t.Fatalf("vCard ohne %q:\n%s", want, text)
		}
	}
	if dl, ok := card["download"].(metamodel.Content); !ok || dl.FileName != "Auerbachs Keller Mephisto & Co.vcf" || !strings.HasPrefix(dl.ContentType, "text/vcard") {
		t.Fatalf("Datei: %v", card["download"])
	}

	// Person ohne Telefon und ohne Anschrift: Hinweis statt QR-Code
	p := e.must("BusinessPartner", "create", data("type", "PERSON", "gender", "MALE", "name1", "Faust", "name2", "Heinrich", "valid_from", "2000-01-01"))["id"].(string)
	if r := e.must("BusinessPartner", "qrCall", map[string]any{"id": p}); r["qr"] != nil || !strings.Contains(r["message"].(string), "Heinrich Faust: keine Telefonnummer") {
		t.Fatalf("ohne Nummer: %v", r)
	}
	if r := e.must("BusinessPartner", "qrContact", map[string]any{"id": p}); !strings.Contains(r["qr"].([]any)[0].(map[string]any)["text"].(string), "N:Faust;Heinrich;;;\r\nFN:Heinrich Faust\r\n") {
		t.Fatalf("Person: %v", r)
	}
}

// TestRoleFilter (0.16.2): Rolle und Finanzrolle als Auswahl der Filterleiste.
func TestRoleFilter(t *testing.T) {
	e := setup(t)
	gretchen := e.newBP("Gretchen")
	e.newBP("Valentin")
	e.must("PartnerRoleType", "create", data("code", "NEIGHBOR", "description", "Nachbarin", "valid_from", "2000-01-01"))
	e.must("PartnerRole", "create", data("bp_id", gretchen, "role_code", "NEIGHBOR", "valid_from", "2000-01-01"))
	if r := e.items("BusinessPartner", map[string]any{"role": "NEIGHBOR"}); len(r) != 1 || r[0]["id"] != gretchen {
		t.Fatalf("Rolle: %v", r)
	}
	resp, _ := e.p.Handle(e.ctx, sdk.Request{Object: sdk.ObjectCatalog, Action: sdk.ActionDescribe})
	for _, d := range resp.Payload.(metamodel.DescribeResponse).Objects {
		if d.Name != "BusinessPartner" {
			continue
		}
		n := 0
		for _, f := range d.Fields {
			if (f.Key == "role" && f.Lookup != nil || f.Key == "role_kind" && len(f.Options) == 2) && f.FilterOnly {
				n++
			}
		}
		if n != 2 {
			t.Fatalf("Filterfelder: %d", n)
		}
	}
}
