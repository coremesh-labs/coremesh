package businesspartner

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/crud"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// --- Kontaktdaten aus Text übernehmen (seit 0.17.0) ---------------------------------
//
// Ein aus dem Web oder einem Briefkopf kopierter Block wie
//
//	Sanitär Gerlach GmbH
//	An der Landeskrone 14c
//	02827 Görlitz
//	Telefon: 03581 - 40 09 50
//	E-Mail: bestellung@sanitaer-gerlach.de
//
// ergänzt den Partner um Anschrift und Kommunikation. Was woran erkannt wird,
// steht in den Katalogen: Erkennungswörter je Kommunikationstyp (Telefon, Fax …)
// und der Aufbau der Postleitzahl je Land (Ländervorwahlen). Vorhandenes bleibt
// unverändert; eine abweichende Anschrift oder ein anderer Name wird nur gemeldet.

var pasteFields = []field{
	{Key: "paste_text", Label: "Text (z. B. Impressum, Briefkopf oder Signatur – Anschrift, Telefon, Fax, E-Mail, Web)", Type: metamodel.TypeTextarea, ActionOnly: true, Wide: true},
	{Key: "paste_check", Label: "Nur prüfen (nichts anlegen)", Type: tBool, ActionOnly: true},
}

func (m *Module) pasteAction() crud.Action {
	return crud.Action{ActionConfig: metamodel.ActionConfig{Name: "pasteContact", Label: "Kontaktdaten aus Text übernehmen …", Icon: "icon-edit",
		Record: true, Fields: []string{"paste_text", "paste_check"}}, Handle: m.pasteContact}
}

// normalizeKeywords: „Telefon ,Tel.,  Fon“ → „Telefon, Tel., Fon“; leer = nil.
func normalizeKeywords(v any) any {
	var out []string
	for _, k := range strings.Split(str(v), ",") {
		if k = strings.Join(strings.Fields(k), " "); k != "" {
			out = append(out, k)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return strings.Join(out, ", ")
}

// keyOf: Vergleichsform eines Erkennungsworts bzw. einer Beschriftung im Text
// („Tel.-Nr.:“ → „tel.-nr“).
func keyOf(s string) string {
	return strings.ToLower(strings.Trim(strings.Join(strings.Fields(s), " "), " .:"))
}

type commTypeInfo struct{ code, category, name string }

type pasteCatalog struct {
	byKeyword map[string]commTypeInfo // Erkennungswort → Typ
	mainOf    map[string]commTypeInfo // Kategorie → Haupttyp
	countries []pasteCountry          // Heimatland zuerst
}

type pasteCountry struct {
	code, name string
	zip        *regexp.Regexp // ganze Zeile: [Länderkürzel-]PLZ Ort
}

func (m *Module) loadPasteCatalog(ctx context.Context) (*pasteCatalog, error) {
	d := today()
	res, err := m.db.Query(ctx, `SELECT code, category_code, description, is_main, keywords FROM partner__comm_types
		WHERE valid_from <= ? AND valid_to >= ? ORDER BY is_main DESC, code`, d, d)
	if err != nil {
		return nil, err
	}
	c := &pasteCatalog{byKeyword: map[string]commTypeInfo{}, mainOf: map[string]commTypeInfo{}}
	for _, r := range res.Rows {
		t := commTypeInfo{code: str(r[0]), category: str(r[1]), name: str(r[2])}
		if _, ok := c.mainOf[t.category]; !ok { // Haupttyp zuerst (ORDER BY)
			c.mainOf[t.category] = t
		}
		for _, k := range strings.Split(str(r[4]), ",") {
			if k = keyOf(k); k != "" {
				if _, dup := c.byKeyword[k]; !dup {
					c.byKeyword[k] = t
				}
			}
		}
	}
	res, err = m.db.Query(ctx, "SELECT country, description, zip_pattern FROM partner__dial_codes ORDER BY is_main DESC, country")
	if err != nil {
		return nil, err
	}
	for _, r := range res.Rows {
		z := strings.TrimSpace(str(r[2]))
		if z == "" {
			continue
		}
		re, err := regexp.Compile(`^(?:[A-Z]{1,3}[-\s])?(` + z + `)\s+(\S.*)$`)
		if err != nil {
			continue // im Katalog geprüft; alter Bestand ohne Prüfung
		}
		c.countries = append(c.countries, pasteCountry{code: str(r[0]), name: str(r[1]), zip: re})
	}
	return c, nil
}

var (
	pasteEmailRe  = regexp.MustCompile(`(?i)^(?:mailto:)?([a-z0-9._%+\-]+@[a-z0-9\-]+(?:\.[a-z0-9\-]+)+)$`)
	pasteWebRe    = regexp.MustCompile(`(?i)^(?:https?://|www\.)\S+\.\S+$`)
	pastePhoneRe  = regexp.MustCompile(`^\+?[\d\s()/.\-]+$`)
	pasteLabelRe  = regexp.MustCompile(`^([\p{L}][\p{L}\s.\-]*?)\s*[:：]\s*(.+)$`)
	pasteWordRe   = regexp.MustCompile(`^([\p{L}][\p{L}.\-]*)\s+(.+)$`)
	pasteStreetRe = regexp.MustCompile(`^(.*\S)\s+(\d+\s*\p{L}?(?:\s*[-/]\s*\d+\s*\p{L}?)?)$`)
)

type pastedContact struct {
	t     commTypeInfo
	value string
}

type pasted struct {
	names    []string
	addr     partnerAddress
	hasAddr  bool
	contacts []pastedContact
	unknown  []string
}

// cleanValue bringt den Wert in die Form der Kategorie; "" = passt nicht.
func cleanValue(category, v string) string {
	v = strings.Join(strings.Fields(v), " ")
	switch category {
	case catEmail:
		if mm := pasteEmailRe.FindStringSubmatch(v); mm != nil {
			v = strings.ToLower(mm[1])
		}
	case catWeb:
		if strings.HasPrefix(strings.ToLower(v), "www.") {
			v = "https://" + v
		}
	case catPhone, catFax:
		v = strings.ReplaceAll(v, " - ", " ")
	}
	if validateContact(category, v) != nil {
		return ""
	}
	return v
}

// guess: Wert ohne (bekannte) Beschriftung – E-Mail, Web oder Telefon (Haupttyp der Kategorie).
func (c *pasteCatalog) guess(v string) (pastedContact, bool) {
	cat := ""
	switch {
	case pasteEmailRe.MatchString(v):
		cat = catEmail
	case pasteWebRe.MatchString(v):
		cat = catWeb
	case pastePhoneRe.MatchString(v) && len(digitsRe.FindAllString(v, -1)) >= 6:
		cat = catPhone
	}
	t, ok := c.mainOf[cat]
	if cat == "" || !ok {
		return pastedContact{}, false
	}
	if val := cleanValue(cat, v); val != "" {
		return pastedContact{t: t, value: val}, true
	}
	return pastedContact{}, false
}

// contact: Zeile als Kommunikation – „Telefon: …“, „Tel. …“ oder ein Wert allein.
func (c *pasteCatalog) contact(line string) (pastedContact, bool) {
	for _, re := range []*regexp.Regexp{pasteLabelRe, pasteWordRe} {
		mm := re.FindStringSubmatch(line)
		if mm == nil {
			continue
		}
		if t, ok := c.byKeyword[keyOf(mm[1])]; ok {
			if v := cleanValue(t.category, mm[2]); v != "" {
				return pastedContact{t: t, value: v}, true
			}
		}
		if re == pasteLabelRe { // unbekannte Beschriftung, z. B. „Mail an: …“
			if pc, ok := c.guess(strings.TrimSpace(mm[2])); ok {
				return pc, true
			}
		}
	}
	return c.guess(line)
}

func (c *pasteCatalog) parse(text string) pasted {
	var lines []string
	for _, l := range strings.Split(strings.ReplaceAll(text, " ", " "), "\n") {
		if l = strings.Join(strings.Fields(l), " "); l != "" {
			lines = append(lines, l)
		}
	}
	var p pasted
	used := make([]bool, len(lines))
	for i, l := range lines {
		if pc, ok := c.contact(l); ok {
			p.contacts, used[i] = append(p.contacts, pc), true
		}
	}
	// Land als eigene Zeile („Österreich“) – dann gilt dessen PLZ-Aufbau.
	country := -1
	for i, l := range lines {
		for j, k := range c.countries {
			if !used[i] && (strings.EqualFold(l, k.name) || l == k.code) {
				country, used[i] = j, true
			}
		}
	}
	zipLine := -1
	for i, l := range lines {
		if used[i] {
			continue
		}
		order := c.countries
		if country >= 0 {
			order = []pasteCountry{c.countries[country]}
		}
		for _, k := range order {
			if mm := k.zip.FindStringSubmatch(l); mm != nil {
				p.addr.zip, p.addr.city, p.addr.country = mm[1], mm[2], k.code
				zipLine, used[i] = i, true
				break
			}
		}
		if zipLine >= 0 {
			break
		}
	}
	if zipLine > 0 && !used[zipLine-1] {
		s := lines[zipLine-1]
		if mm := pasteStreetRe.FindStringSubmatch(s); mm != nil {
			p.addr.street, p.addr.houseNo = mm[1], strings.ReplaceAll(mm[2], " ", "")
		} else {
			p.addr.street = s
		}
		p.hasAddr, used[zipLine-1] = true, true
	}
	for i, l := range lines {
		if used[i] {
			continue
		}
		if zipLine < 0 || i < zipLine {
			p.names = append(p.names, l)
		} else {
			p.unknown = append(p.unknown, l)
		}
	}
	return p
}

func addrText(a partnerAddress) string {
	return strings.TrimSpace(strings.TrimSpace(a.street+" "+a.houseNo) + ", " + a.zip + " " + a.city + " (" + a.country + ")")
}

func sameAddr(a, b partnerAddress) bool {
	n := func(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), "")) }
	return n(a.street) == n(b.street) && n(a.houseNo) == n(b.houseNo) && n(a.zip) == n(b.zip) && n(a.city) == n(b.city) && strings.EqualFold(a.country, b.country)
}

// sameValue: gleiche Nummer (nur Ziffern) bzw. gleicher Text ohne Groß/klein.
func sameValue(category, a, b string) bool {
	if category == catPhone || category == catFax {
		return strings.Join(digitsRe.FindAllString(a, -1), "") == strings.Join(digitsRe.FindAllString(b, -1), "")
	}
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func (m *Module) pasteContact(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	var in struct {
		Data struct {
			Text  string `json:"paste_text"`
			Check any    `json:"paste_check"`
		} `json:"data"`
	}
	if err := sdk.Decode(req.Payload, &in); err != nil {
		return sdk.Response{}, err
	}
	id, kind, name1, name2, err := m.partnerOf(ctx, req.Payload)
	if err != nil {
		return sdk.Response{}, err
	}
	if strings.TrimSpace(in.Data.Text) == "" {
		return sdk.Response{}, invalid("Text fehlt – Anschrift, Telefon, E-Mail … hineinkopieren")
	}
	check := asBool(in.Data.Check)
	if !check {
		if ok, err := sdk.CheckAccess(ctx, "BusinessPartner", "update", ""); err != nil {
			return sdk.Response{}, err
		} else if !ok {
			return sdk.Response{}, fmt.Errorf("%w: keine Berechtigung, Geschäftspartner zu ändern", sdk.ErrPermissionDenied)
		}
	}
	cat, err := m.loadPasteCatalog(ctx)
	if err != nil {
		return sdk.Response{}, err
	}
	p := cat.parse(in.Data.Text)
	var rows [][]any
	add := func(what, value, result string) { rows = append(rows, []any{what, value, result}) }
	verb := "angelegt"
	if check {
		verb = "wird angelegt"
	}
	created := 0
	err = m.db.InTx(ctx, nil, func(ctx context.Context) error {
		if name := strings.Join(p.names, " "); name != "" {
			if have := displayName(kind, name1, name2); keyOf(name) != keyOf(have) && keyOf(name) != keyOf(name1) {
				add("Name", name, "weicht ab von „"+have+"“ – nicht geändert (Stammdaten)")
			} else {
				add("Name", name, "passt")
			}
		}
		if p.hasAddr {
			if err := m.pasteAddress(ctx, id, p.addr, check, verb, add, &created); err != nil {
				return err
			}
		}
		have, err := m.currentContacts(ctx, id)
		if err != nil {
			return err
		}
		for _, c := range p.contacts {
			dup, first := false, true
			for _, h := range have {
				if h.category == c.t.category {
					first = false
					dup = dup || h.code == c.t.code && sameValue(c.t.category, h.value, c.value)
				}
			}
			if dup {
				add(c.t.name, c.value, "schon vorhanden")
				continue
			}
			if !check {
				if _, err := m.set.Entity("PartnerContact").Create(ctx, map[string]any{"data": map[string]any{
					"bp_id": id, "comm_type_code": c.t.code, "value": c.value, "is_default": first}}); err != nil {
					return fmt.Errorf("%s %s: %w", c.t.name, c.value, err)
				}
			}
			created++
			have = append(have, commValue{code: c.t.code, category: c.t.category, value: c.value})
			if first {
				add(c.t.name, c.value, verb+" (Standard)")
			} else {
				add(c.t.name, c.value, verb)
			}
		}
		for _, u := range p.unknown {
			add("nicht erkannt", u, "–")
		}
		return nil
	})
	if err != nil {
		return sdk.Response{}, err
	}
	msg := fmt.Sprintf("%s: %d Einträge angelegt.", displayName(kind, name1, name2), created)
	if check {
		msg = fmt.Sprintf("%s: Prüfung – %d Einträge würden angelegt, nichts gespeichert.", displayName(kind, name1, name2), created)
	}
	if len(rows) == 0 {
		msg = "Im Text wurde nichts erkannt (Erkennungswörter: Katalog Kommunikationstypen; Postleitzahl: Katalog Ländervorwahlen)."
	}
	return sdk.Response{Payload: map[string]any{"message": msg, "table": map[string]any{"columns": []any{"", "", ""}, "rows": rows}}}, nil
}

type commValue struct{ code, category, value string }

// currentContacts: heute gültige Kommunikation des Partners mit Typ und Kategorie.
func (m *Module) currentContacts(ctx context.Context, bp string) ([]commValue, error) {
	d := today()
	res, err := m.db.Query(ctx, `SELECT c.comm_type_code, t.category_code, c.value FROM partner__contacts c
		JOIN partner__comm_types t ON t.code = c.comm_type_code AND t.valid_from <= ? AND t.valid_to >= ?
		WHERE c.bp_id = ? AND c.valid_from <= ? AND c.valid_to >= ?`, d, d, bp, d, d)
	if err != nil {
		return nil, err
	}
	out := make([]commValue, 0, len(res.Rows))
	for _, r := range res.Rows {
		out = append(out, commValue{code: str(r[0]), category: str(r[1]), value: str(r[2])})
	}
	return out, nil
}

// pasteAddress: Anschrift als Hauptanschrift, wenn der Partner heute keine hat;
// eine gleiche Adresse (Stammdaten Adressen) wird wiederverwendet.
func (m *Module) pasteAddress(ctx context.Context, bp string, a partnerAddress, check bool, verb string, add func(string, string, string), created *int) error {
	text := addrText(a)
	if cur, ok := m.mainAddress(ctx, bp); ok {
		if sameAddr(cur, a) {
			add("Anschrift", text, "schon vorhanden")
		} else {
			add("Anschrift", text, "weicht ab von „"+addrText(cur)+"“ – nicht geändert (Umzug: unter „Adressen“ mit neuem Gültig ab)")
		}
		return nil
	}
	d := today()
	role, err := m.db.Query(ctx, "SELECT code, description FROM partner__address_roles WHERE is_main = ? AND valid_from <= ? AND valid_to >= ?", true, d, d)
	if err != nil {
		return err
	}
	if len(role.Rows) == 0 {
		add("Anschrift", text, "nicht angelegt – keine Adressrolle „Hauptanschrift“ im Katalog Adressrollen")
		return nil
	}
	*created++
	if check {
		add("Anschrift", text, verb+" ("+str(role.Rows[0][1])+")")
		return nil
	}
	res, err := m.db.Query(ctx, `SELECT id, street, house_no, zip_code, city, country FROM partner__addresses WHERE zip_code = ? AND country = ?`, a.zip, a.country)
	if err != nil {
		return err
	}
	addrID := ""
	for _, r := range res.Rows {
		if sameAddr(partnerAddress{street: str(r[1]), houseNo: str(r[2]), zip: str(r[3]), city: str(r[4]), country: str(r[5])}, a) {
			addrID = str(r[0])
			break
		}
	}
	if addrID == "" {
		data := map[string]any{"street": a.street, "zip_code": a.zip, "city": a.city, "country": a.country}
		if a.houseNo != "" {
			data["house_no"] = a.houseNo
		}
		resp, err := m.set.Entity("PartnerAddressData").Create(ctx, map[string]any{"data": data})
		if err != nil {
			return fmt.Errorf("Anschrift %s: %w", text, err)
		}
		addrID = str(resp.Payload.(record)["id"])
	}
	if _, err := m.set.Entity("PartnerAddress").Create(ctx, map[string]any{"data": map[string]any{
		"bp_id": bp, "address_id": addrID, "address_role_code": role.Rows[0][0], "is_default": true}}); err != nil {
		return fmt.Errorf("Anschrift %s: %w", text, err)
	}
	add("Anschrift", text, verb+" ("+str(role.Rows[0][1])+")")
	return nil
}
