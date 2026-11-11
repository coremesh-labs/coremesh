package businesspartner

import (
	"context"
	"encoding/base64"
	"regexp"
	"strings"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/crud"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// Kontakt aufs Handy (seit 0.16.0): Telefonnummern und Kontaktdaten eines
// Geschäftspartners als QR-Code (der WebServer zeichnet {"qr": [{text, caption}]}).
//
//   - „Anrufen (QR)“: je Telefonnummer ein QR-Code nach der Vorlage der
//     Kommunikationskategorie (Standard tel:{number}); die Kamera des Handys
//     öffnet damit die Telefon-App. Manche Scanner (z. B. MIUI) zeigen tel: nur
//     als Text – dann hilft die Vorlage {number} (nur die Nummer).
//   - „Kontakt aufs Handy (QR)“: eine vCard 3.0 mit Name, Anschrift, Nummern,
//     E-Mail und Webseite als QR-Code und als .vcf zum Speichern.
//
// Nummern werden international geschrieben: Die Ländervorwahl kommt aus dem
// Katalog „Ländervorwahlen“ zum Land der Hauptanschrift (ohne Anschrift: das
// Heimatland des Katalogs). Welche Kategorie wählbar ist (PHONE), steht im
// Katalog der Kommunikationskategorien; der vCard-Typ je Kommunikationstyp im
// Katalog der Kommunikationstypen. In der Kommunikation sind Nummern, E-Mail
// und Webseite Links (tel:, mailto:, https:) – auf dem Handy ein Tipp.

const (
	catPhone = "PHONE"
	catFax   = "FAX"
	catEmail = "EMAIL"
	catWeb   = "WEB"
)

// Ländervorwahlen (Katalog).
func (m *Module) dialCodeCatalog() *entity {
	return &entity{
		Object: "PartnerDialCode", Customizing: true, Section: "Kataloge", Title: "Ländervorwahlen", Icon: "icon-phone", Table: "partner__dial_codes",
		Keys: []string{"country"}, Order: "country", Search: []string{"country", "description"},
		Fields: []field{
			{Key: "country", Label: "Land", Help: "ISO-2, wie in der Anschrift", Short: "Land", Type: tText, Required: true, Listable: true, Immutable: true},
			{Key: "description", Label: "Bezeichnung", Type: tText, Required: true, Listable: true},
			{Key: "dial_code", Label: "Ländervorwahl", Help: "Ohne +, z. B. 49", Short: "Vorwahl", Type: tText, Required: true, Listable: true},
			{Key: "trunk_prefix", Label: "Verkehrsausscheidungsziffer", Help: "Entfällt international, z. B. 0; leer = keine", Short: "Präfix", Type: tText, Listable: true},
			{Key: "is_main", Label: "Heimatland", Help: "Für Nummern ohne Anschrift", Short: "Heimat", Type: tBool, Listable: true},
			// seit 0.17.0: Postleitzahl erkennen beim Übernehmen aus Text
			{Key: "zip_pattern", Label: "Aufbau der Postleitzahl", Help: "Regulärer Ausdruck, z. B. \\d{5}; leer = nicht erkennen", Short: "PLZ", Type: tText, Listable: true},
		},
		Validate: func(ctx context.Context, rec, _ record) error {
			rec["country"] = strings.ToUpper(strings.TrimSpace(str(rec["country"])))
			if !countryRe.MatchString(str(rec["country"])) {
				return invalid("Land %q: zwei Buchstaben nach ISO 3166 (z. B. DE)", str(rec["country"]))
			}
			rec["dial_code"] = strings.TrimPrefix(strings.TrimSpace(str(rec["dial_code"])), "+")
			if !onlyDigitsRe.MatchString(str(rec["dial_code"])) {
				return invalid("Ländervorwahl %q: nur Ziffern", str(rec["dial_code"]))
			}
			if z := strings.TrimSpace(str(rec["zip_pattern"])); z != "" {
				if _, err := regexp.Compile(z); err != nil {
					return invalid("Aufbau der Postleitzahl %q: kein gültiger regulärer Ausdruck", z)
				}
				rec["zip_pattern"] = z
			} else {
				rec["zip_pattern"] = nil
			}
			if t := strings.TrimSpace(str(rec["trunk_prefix"])); t != "" && !onlyDigitsRe.MatchString(t) {
				return invalid("Verkehrsausscheidungsziffer %q: nur Ziffern", t)
			}
			if !asBool(rec["is_main"]) {
				return nil
			}
			return m.uniqueMain(ctx, "partner__dial_codes", "country", str(rec["country"]), "", nil, "Heimatland")
		},
	}
}

var (
	onlyDigitsRe = regexp.MustCompile(`^[0-9]{1,4}$`)
	dialSepRe    = regexp.MustCompile(`[ ()/.\-]+`)
)

// dialing: Ländervorwahl und Präfix eines Landes.
type dialing struct{ code, trunk string }

// dialFor: Vorwahl zum Land; leeres Land = Heimatland; unbekannt = keine.
func (m *Module) dialFor(ctx context.Context, country string) (dialing, bool) {
	sql, args := "SELECT dial_code, trunk_prefix FROM partner__dial_codes WHERE country = ?", []any{strings.ToUpper(country)}
	if country == "" {
		sql, args = "SELECT dial_code, trunk_prefix FROM partner__dial_codes WHERE is_main = ?", []any{true}
	}
	res, err := m.db.Query(ctx, sql, args...)
	if err != nil || len(res.Rows) == 0 {
		return dialing{}, false
	}
	return dialing{code: str(res.Rows[0][0]), trunk: strings.TrimSpace(str(res.Rows[0][1]))}, true
}

// international: Nummer als +<Vorwahl><Nummer> (für tel:) und lesbar mit
// Leerzeichen („+49 341 4801099“). Ohne Vorwahl bleibt die Nummer, wie sie ist.
func international(value string, d dialing, ok bool) (dial, display string) {
	v := strings.ReplaceAll(strings.TrimSpace(value), "(0)", "") // +49 (0)341 …
	switch {
	case strings.HasPrefix(v, "+"):
	case strings.HasPrefix(v, "00"):
		v = "+" + v[2:]
	case ok && d.trunk != "" && strings.HasPrefix(v, d.trunk):
		v = "+" + d.code + " " + strings.TrimPrefix(v, d.trunk)
	case ok && d.trunk == "":
		v = "+" + d.code + " " + v
	}
	display = strings.Join(strings.Fields(dialSepRe.ReplaceAllString(v, " ")), " ")
	return strings.ReplaceAll(display, " ", ""), display
}

// partnerContact: eine Kommunikation des Partners mit Kategorie und vCard-Typ.
type partnerContact struct {
	category, typeName, vcardType, value, qrTemplate string
	isDefault                                        bool
}

// contactsOf: heute gültige Kommunikation, Standard zuerst.
func (m *Module) contactsOf(ctx context.Context, bp string) ([]partnerContact, error) {
	d := today()
	res, err := m.db.Query(ctx, `SELECT t.category_code, t.description, t.vcard_type, c.value, c.is_default, k.qr_template
		FROM partner__contacts c JOIN partner__comm_types t ON t.code = c.comm_type_code AND t.valid_from <= ? AND t.valid_to >= ?
		LEFT JOIN partner__comm_categories k ON k.code = t.category_code
		WHERE c.bp_id = ? AND c.valid_from <= ? AND c.valid_to >= ?
		ORDER BY t.category_code, c.is_default DESC, t.is_main DESC, t.code, c.value`, d, d, bp, d, d)
	if err != nil {
		return nil, err
	}
	var out []partnerContact
	for _, r := range res.Rows {
		out = append(out, partnerContact{category: str(r[0]), typeName: str(r[1]), vcardType: str(r[2]), value: str(r[3]), isDefault: asBool(r[4]), qrTemplate: strings.TrimSpace(str(r[5]))})
	}
	return out, nil
}

// partnerAddress: Hauptanschrift (Adressrolle mit Hauptanschrift), heute gültig.
type partnerAddress struct{ street, houseNo, zip, city, country string }

func (m *Module) mainAddress(ctx context.Context, bp string) (partnerAddress, bool) {
	d := today()
	res, err := m.db.Query(ctx, `SELECT a.street, a.house_no, a.zip_code, a.city, a.country
		FROM partner__bp_addresses b JOIN partner__addresses a ON a.id = b.address_id
		LEFT JOIN partner__address_roles r ON r.code = b.address_role_code AND r.valid_from <= ? AND r.valid_to >= ?
		WHERE b.bp_id = ? AND b.valid_from <= ? AND b.valid_to >= ?
		ORDER BY r.is_main DESC, b.is_default DESC, b.valid_from DESC`, d, d, bp, d, d)
	if err != nil || len(res.Rows) == 0 {
		return partnerAddress{}, false
	}
	r := res.Rows[0]
	return partnerAddress{street: str(r[0]), houseNo: str(r[1]), zip: str(r[2]), city: str(r[3]), country: str(r[4])}, true
}

// countryOf: Land der Hauptanschrift, sonst leer (= Heimatland).
func (m *Module) countryOf(ctx context.Context, bp string) string {
	a, _ := m.mainAddress(ctx, bp)
	return a.country
}

// decorateContact: Link zum Wert (tel:, mailto:, https:) für die Oberfläche.
func (m *Module) decorateContact(ctx context.Context, rec record) error {
	res, err := m.db.Query(ctx, "SELECT category_code FROM partner__comm_types WHERE code = ? ORDER BY valid_to DESC", rec["comm_type_code"])
	if err != nil || len(res.Rows) == 0 {
		return err
	}
	value := strings.TrimSpace(str(rec["value"]))
	var href string
	switch str(res.Rows[0][0]) {
	case catPhone:
		d, ok := m.dialFor(ctx, m.countryOf(ctx, str(rec["bp_id"])))
		dial, _ := international(value, d, ok)
		href = "tel:" + dial
	case catEmail:
		href = "mailto:" + value
	case catWeb:
		href = value
	}
	if href != "" && value != "" {
		rec["_links"] = map[string]any{"value": href}
	}
	return nil
}

// --- Aktionen ------------------------------------------------------------------------

func (m *Module) contactActions() []crud.Action {
	return []crud.Action{
		{ActionConfig: metamodel.ActionConfig{Name: "qrCall", Label: "Anrufen (QR) …", Icon: "icon-phone", Record: true}, Handle: m.qrCall},
		{ActionConfig: metamodel.ActionConfig{Name: "qrContact", Label: "Kontakt aufs Handy (QR) …", Icon: "icon-phone", Record: true}, Handle: m.qrContact},
	}
}

// partnerOf: BP-Nummer aus der ID der Zeitscheibe und der heutige Stand.
func (m *Module) partnerOf(ctx context.Context, payload any) (id, kind, name1, name2 string, err error) {
	var in struct {
		ID string `json:"id"`
	}
	if err = sdk.Decode(payload, &in); err != nil {
		return
	}
	id, _, _ = strings.Cut(in.ID, "|")
	if strings.TrimSpace(id) == "" {
		err = invalid("Geschäftspartner fehlt")
		return
	}
	res, qerr := m.db.Query(ctx, `SELECT type, name1, name2 FROM partner__bp WHERE id = ? ORDER BY
		CASE WHEN valid_from <= ? AND valid_to >= ? THEN 0 ELSE 1 END, valid_from DESC`, id, today(), today())
	if qerr != nil {
		err = qerr
		return
	}
	if len(res.Rows) == 0 {
		err = invalid("Geschäftspartner %s gibt es nicht", id)
		return
	}
	kind, name1, name2 = str(res.Rows[0][0]), str(res.Rows[0][1]), str(res.Rows[0][2])
	return
}

func (m *Module) qrCall(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	id, kind, name1, name2, err := m.partnerOf(ctx, req.Payload)
	if err != nil {
		return sdk.Response{}, err
	}
	contacts, err := m.contactsOf(ctx, id)
	if err != nil {
		return sdk.Response{}, err
	}
	d, ok := m.dialFor(ctx, m.countryOf(ctx, id))
	var qr []any
	for _, c := range contacts {
		if c.qrTemplate == "" || c.value == "" {
			continue // Kategorie ohne QR-Vorlage (z. B. Fax, E-Mail)
		}
		dial, display := international(c.value, d, ok)
		text := strings.NewReplacer("{number}", dial, "{display}", display).Replace(c.qrTemplate)
		qr = append(qr, map[string]any{"text": text, "caption": c.typeName + ": " + display})
	}
	name := displayName(kind, name1, name2)
	if len(qr) == 0 {
		return sdk.Response{Payload: map[string]any{"message": name + ": keine Telefonnummer hinterlegt (Kommunikation)."}}, nil
	}
	return sdk.Response{Payload: map[string]any{"message": name + " – mit der Kamera des Handys scannen, dann auf die Nummer tippen.", "qr": qr}}, nil
}

func (m *Module) qrContact(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	id, kind, name1, name2, err := m.partnerOf(ctx, req.Payload)
	if err != nil {
		return sdk.Response{}, err
	}
	contacts, err := m.contactsOf(ctx, id)
	if err != nil {
		return sdk.Response{}, err
	}
	addr, hasAddr := m.mainAddress(ctx, id)
	d, ok := m.dialFor(ctx, addr.country)
	card := vcard(kind, name1, name2, addr, hasAddr, contacts, d, ok)
	name := displayName(kind, name1, name2)
	file := safeFile(name) + ".vcf"
	return sdk.Response{Payload: map[string]any{
		"message": name + " – mit der Kamera des Handys scannen und „Kontakt hinzufügen“ wählen; die Datei .vcf übernimmt den Kontakt auch am Rechner.",
		"qr":      []any{map[string]any{"text": card, "caption": name, "level": "L"}},
		"download": metamodel.Content{Content: base64.StdEncoding.EncodeToString([]byte(card)),
			ContentType: "text/vcard; charset=utf-8", FileName: file},
	}}, nil
}

func displayName(kind, name1, name2 string) string {
	if kind == "PERSON" && name2 != "" {
		return name2 + " " + name1
	}
	return strings.TrimSpace(name1 + " " + name2)
}

var unsafeFileRe = regexp.MustCompile(`[^\p{L}\p{N} .,&_-]+`)

func safeFile(s string) string {
	s = strings.TrimRight(strings.Join(strings.Fields(unsafeFileRe.ReplaceAllString(s, " ")), " "), ". ")
	if s == "" {
		return "Kontakt"
	}
	return s
}

// vcard: Visitenkarte nach RFC 2426 (vCard 3.0), Zeilenende CRLF.
func vcard(kind, name1, name2 string, a partnerAddress, hasAddr bool, contacts []partnerContact, d dialing, ok bool) string {
	var b strings.Builder
	line := func(s string) { b.WriteString(s + "\r\n") }
	line("BEGIN:VCARD")
	line("VERSION:3.0")
	if kind == "PERSON" {
		line("N:" + vesc(name1) + ";" + vesc(name2) + ";;;")
		line("FN:" + vesc(displayName(kind, name1, name2)))
	} else {
		line("N:;;;;")
		line("FN:" + vesc(displayName(kind, name1, name2)))
		line("ORG:" + vesc(name1))
		line("X-ABShowAs:COMPANY") // iPhone: als Firma anzeigen
	}
	for _, c := range contacts {
		if c.value == "" {
			continue
		}
		typ := ""
		if t := strings.TrimSpace(c.vcardType); t != "" {
			typ = ";TYPE=" + t
		}
		switch c.category {
		case catPhone, catFax:
			_, display := international(c.value, d, ok)
			line("TEL" + typ + ":" + vesc(display))
		case catEmail:
			line("EMAIL" + typ + ":" + vesc(c.value))
		case catWeb:
			line("URL:" + vesc(c.value))
		}
	}
	if hasAddr {
		street := strings.TrimSpace(a.street + " " + a.houseNo)
		line("ADR;TYPE=WORK:;;" + vesc(street) + ";" + vesc(a.city) + ";;" + vesc(a.zip) + ";" + vesc(a.country))
	}
	line("END:VCARD")
	return b.String()
}

// vesc: Sonderzeichen in vCard-Werten (\ ; , Zeilenumbruch).
func vesc(s string) string {
	return strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\r\n", `\n`, "\n", `\n`).Replace(s)
}
