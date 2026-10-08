package businesspartner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/events"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
	"github.com/coremesh-labs/coremesh/pkg/sdk/numrange"
)

// BP-Nummer und Kurzname (seit 0.10.0).
//
//   - Die BP-Nummer ist der Schlüssel des Partners. Sie kommt aus dem
//     Nummernkreis BusinessPartner; welches Intervall, legt die Partnergruppe
//     fest (Katalog PartnerGroup, Intervallschlüssel). Ob intern vergeben oder
//     eingegeben wird, entscheidet das Intervall (numrange: externe Vergabe,
//     mit Muster oder Bereich). Eine eingegebene Nummer muss frei sein.
//   - Der Kurzname (Matchcode, Spalte search_term) ist änderbar und dient der
//     Suche und Anzeige. Leer = Vorschlag aus dem Namen. Ob er eindeutig sein
//     muss, steht in der Einstellung short_name_unique (Standard: ja).
//   - Partner von 0.9.0 und früher hatten eine GUID. Migrate vergibt ihnen
//     eine Nummer aus der Standardgruppe, merkt die GUID in legacy_id und
//     meldet je Partner das SystemEvent BusinessPartner.rekey (old_id,
//     new_id). Andere Module stellen ihre Verweise über
//     BusinessPartnerService.resolve um.

const (
	rangeObject    = "BusinessPartner"
	serviceObject  = "BusinessPartnerService"
	actionResolve  = "resolve"
	actionRekey    = "rekey"
	defaultGroup   = "STD"
	shortNameLimit = 20
)

// settings: Einstellungen des Moduls (settings.modules.businesspartner).
type settings struct {
	// ShortNameUnique: Kurzname muss eindeutig sein (Standard true).
	ShortNameUnique *bool `json:"short_name_unique"`
}

func (s settings) shortNameUnique() bool { return s.ShortNameUnique == nil || *s.ShortNameUnique }

// defineNumbers meldet den Nummernkreis an (Standard: 100000–999999, intern).
func (m *Module) defineNumbers(ctx context.Context) error {
	return numrange.Define(ctx, m.services, numrange.Definition{Object: rangeObject, Owner: "partner",
		Description: "Geschäftspartner (Intervall je Partnergruppe)", Width: 6, From: 100000, To: 999999})
}

// --- Partnergruppen --------------------------------------------------------------------

func (m *Module) partnerGroupCatalog() *entity {
	return &entity{
		Object: "PartnerGroup", Section: "Kataloge", Title: "Partnergruppen", Icon: "icon-tag", Table: "partner__groups",
		Keys: []string{"code"}, Order: "code", Search: []string{"code", "description"},
		Fields: []field{
			{Key: "code", Label: "Code", Type: tText, Required: true, Listable: true, Immutable: true},
			{Key: "description", Label: "Beschreibung", Type: tText, Required: true, Listable: true},
			{Key: "range_key", Label: "Intervallschlüssel im Nummernkreis BusinessPartner (leer = Code)", Type: tText, Listable: true},
			{Key: "is_default", Label: "Standardgruppe (neue Partner ohne Gruppe, Übernahme alter Partner)", Type: tBool, Listable: true},
		},
		Validate: func(ctx context.Context, rec, _ record) error {
			rec["code"] = strings.ToUpper(strings.TrimSpace(str(rec["code"])))
			if strings.TrimSpace(str(rec["range_key"])) == "" {
				rec["range_key"] = rec["code"]
			}
			rec["range_key"] = strings.ToUpper(strings.TrimSpace(str(rec["range_key"])))
			if !asBool(rec["is_default"]) {
				return nil
			}
			return m.uniqueDefault(ctx, str(rec["code"]))
		},
	}
}

func (m *Module) uniqueDefault(ctx context.Context, code string) error {
	res, err := m.db.Query(ctx, "SELECT code FROM partner__groups WHERE is_default = ? AND code <> ?", true, code)
	if err != nil {
		return err
	}
	if len(res.Rows) > 0 {
		return invalid("Standardgruppe ist bereits %q – erst dort entfernen", str(res.Rows[0][0]))
	}
	return nil
}

// group: Gruppe (leer = Standardgruppe) → Code und Intervallschlüssel.
func (m *Module) group(ctx context.Context, code string) (string, string, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	q, args := "SELECT code, range_key FROM partner__groups WHERE code = ?", []any{code}
	if code == "" {
		q, args = "SELECT code, range_key FROM partner__groups WHERE is_default = ? ORDER BY code", []any{true}
	}
	res, err := m.db.Query(ctx, q, args...)
	if err != nil {
		return "", "", err
	}
	if len(res.Rows) == 0 {
		if code == "" {
			return "", "", invalid("Partnergruppe ist Pflicht (keine Standardgruppe im Katalog Partnergruppen)")
		}
		return "", "", invalid("Partnergruppe %q gibt es nicht", code)
	}
	key := str(res.Rows[0][1])
	if key == "" {
		key = str(res.Rows[0][0])
	}
	return str(res.Rows[0][0]), key, nil
}

// --- Anlegen: Nummer und Kurzname --------------------------------------------------------

// preparePartner (vor der Transaktion): Gruppe, Kurzname-Vorschlag, BP-Nummer.
func (m *Module) preparePartner(ctx context.Context, rec record) error {
	code, key, err := m.group(ctx, str(rec["group_code"]))
	if err != nil {
		return err
	}
	rec["group_code"] = code
	if strings.TrimSpace(str(rec["search_term"])) == "" {
		if rec["search_term"], err = m.suggestShortName(ctx, str(rec["name1"]), ""); err != nil {
			return err
		}
	}
	id := strings.ToUpper(strings.TrimSpace(str(rec["id"])))
	if id != "" {
		if other, err := m.partnerByID(ctx, id); err != nil {
			return err
		} else if other != "" {
			return invalid("BP-Nummer %s ist bereits vergeben (%s) – andere Nummer wählen", id, other)
		}
	}
	res, err := numrange.Assign(ctx, m.services, numrange.AssignRequest{
		Request: numrange.Request{Object: rangeObject, Key: key, Reference: str(rec["name1"])}, Value: id})
	if err != nil {
		if errors.Is(err, sdk.ErrUnimplemented) || errors.Is(err, sdk.ErrUnavailable) {
			return fmt.Errorf("%w: Nummernkreise nicht verfügbar: %v", sdk.ErrUnavailable, err)
		}
		return err
	}
	if other, err := m.partnerByID(ctx, res.Number); err != nil {
		return err
	} else if other != "" {
		return invalid("BP-Nummer %s ist bereits vergeben (%s) – Stand des Intervalls %s prüfen", res.Number, other, res.Interval)
	}
	rec["id"] = res.Number
	return nil
}

// partnerByID: Name des Partners mit der Nummer ("" = frei).
func (m *Module) partnerByID(ctx context.Context, id string) (string, error) {
	res, err := m.db.Query(ctx, "SELECT name1 FROM partner__bp WHERE id = ? ORDER BY valid_from DESC", id)
	if err != nil || len(res.Rows) == 0 {
		return "", err
	}
	return str(res.Rows[0][0]), nil
}

// checkShortName: Kurzname Pflicht, bei short_name_unique eindeutig.
func (m *Module) checkShortName(ctx context.Context, rec, old record) error {
	name := strings.ToUpper(strings.TrimSpace(str(rec["search_term"])))
	if name == "" {
		return invalid("Kurzname ist Pflicht")
	}
	rec["search_term"] = name
	if !m.settings.shortNameUnique() || (old != nil && str(old["search_term"]) == name) {
		return nil
	}
	res, err := m.db.Query(ctx, "SELECT id, name1 FROM partner__bp WHERE search_term = ? AND id <> ?", name, str(rec["id"]))
	if err != nil {
		return err
	}
	if len(res.Rows) > 0 {
		return invalid("Kurzname %s ist bereits vergeben (%s %s) – anderen Kurznamen wählen", name, str(res.Rows[0][0]), str(res.Rows[0][1]))
	}
	return nil
}

// suggestShortName: Name in Großbuchstaben, Umlaute ausgeschrieben, nur A–Z
// und 0–9; ist er vergeben (und eindeutig verlangt), mit Zähler (MUELLER2 …).
func (m *Module) suggestShortName(ctx context.Context, name, self string) (string, error) {
	base := shortName(name)
	if base == "" {
		return "", nil
	}
	if !m.settings.shortNameUnique() {
		return base, nil
	}
	for i := 1; i < 1000; i++ {
		cand := base
		if i > 1 {
			suffix := fmt.Sprint(i)
			if len(cand)+len(suffix) > shortNameLimit {
				cand = cand[:shortNameLimit-len(suffix)]
			}
			cand += suffix
		}
		res, err := m.db.Query(ctx, "SELECT 1 FROM partner__bp WHERE search_term = ? AND id <> ?", cand, self)
		if err != nil {
			return "", err
		}
		if len(res.Rows) == 0 {
			return cand, nil
		}
	}
	return base, nil
}

func shortName(name string) string {
	r := strings.NewReplacer("ä", "AE", "ö", "OE", "ü", "UE", "Ä", "AE", "Ö", "OE", "Ü", "UE", "ß", "SS")
	var b strings.Builder
	for _, c := range r.Replace(name) {
		if c < unicode.MaxASCII && (unicode.IsLetter(c) || unicode.IsDigit(c)) {
			b.WriteRune(unicode.ToUpper(c))
		}
		if b.Len() >= shortNameLimit {
			break
		}
	}
	return b.String()
}

// partnerFormState: Neuanlage – BP-Nummer nur bei externer Vergabe der Gruppe
// (Pflicht, mit Hinweis auf das Erlaubte), Kurzname-Vorschlag aus dem Namen.
func (m *Module) partnerFormState(ctx context.Context, req metamodel.FormStateRequest) (metamodel.FormState, error) {
	st := metamodel.FormState{Fields: map[string]metamodel.FieldState{}}
	if req.Mode != "create" {
		return st, nil
	}
	if req.Values["search_term"] == "" && req.Values["name1"] != "" {
		if v, err := m.suggestShortName(ctx, req.Values["name1"], ""); err == nil && v != "" {
			st.Fields["search_term"] = metamodel.FieldState{Value: &v}
		}
	}
	code, key, err := m.group(ctx, req.Values["group_code"])
	if err != nil {
		st.Message = err.Error()
		return st, nil
	}
	if req.Values["group_code"] == "" {
		st.Fields["group_code"] = metamodel.FieldState{Value: &code}
	}
	info, err := numrange.IntervalInfo(ctx, m.services, numrange.Request{Object: rangeObject, Key: key})
	if err != nil {
		return st, nil
	}
	yes, no := true, false
	if info.External {
		allowed := fmt.Sprintf("%d–%d", info.From, info.To)
		if info.ExternalPattern != "" {
			allowed = info.ExternalPattern
		}
		st.Fields["id"] = metamodel.FieldState{Visible: &yes, Required: &yes}
		st.Message = fmt.Sprintf("Gruppe %s: BP-Nummer eingeben (erlaubt: %s); ist sie vergeben, eine andere wählen.", code, allowed)
	} else {
		st.Fields["id"] = metamodel.FieldState{Visible: &no}
	}
	return st, nil
}

// --- Migration von GUIDs ---------------------------------------------------------------

// partnerTables: Tabellen des Moduls mit Verweis auf den Partner.
var partnerTables = []string{"partner__roles", "partner__bp_addresses", "partner__contacts", "partner__bank_details", "partner__company_codes"}

// Migrate vergibt Partnern ohne Gruppe (bis 0.9.0, GUID) eine BP-Nummer der
// Standardgruppe und stellt die eigenen Verweise um (module.Migrator).
func (m *Module) Migrate(ctx context.Context) error {
	res, err := m.db.Query(ctx, "SELECT DISTINCT id FROM partner__bp WHERE group_code IS NULL ORDER BY id")
	if err != nil || len(res.Rows) == 0 {
		return err
	}
	code, key, err := m.group(ctx, "")
	if err != nil {
		return err
	}
	for _, r := range res.Rows {
		old := str(r[0])
		name, _ := m.partnerByID(ctx, old)
		nr, err := numrange.Assign(ctx, m.services, numrange.AssignRequest{
			Request: numrange.Request{Object: rangeObject, Key: key, Reference: "Übernahme " + old + " " + name}})
		if err != nil {
			return fmt.Errorf("BP-Nummer für %s: %w", old, err)
		}
		err = m.db.InTx(ctx, nil, func(ctx context.Context) error {
			if _, err := m.db.Exec(ctx, "UPDATE partner__bp SET id = ?, legacy_id = ?, group_code = ? WHERE id = ?", nr.Number, old, code, old); err != nil {
				return err
			}
			for _, t := range partnerTables {
				if _, err := m.db.Exec(ctx, "UPDATE "+t+" SET bp_id = ? WHERE bp_id = ?", nr.Number, old); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		m.log.InfoContext(ctx, "Partner umgeschlüsselt", "old_id", old, "new_id", nr.Number)
		if err := events.Push(ctx, m.services, events.Event{Object: "BusinessPartner", Action: actionRekey, EntityID: nr.Number,
			Data: map[string]any{"old_id": old, "new_id": nr.Number}}); err != nil {
			m.log.WarnContext(ctx, "SystemEvent BusinessPartner.rekey nicht gesendet", "err", err.Error())
		}
	}
	return nil
}

// resolve: BusinessPartnerService.resolve {ids: [...]} → {ids: {alt: neu}} für
// Verweise auf GUIDs von 0.9.0 und früher; unbekannte und aktuelle fehlen.
func (m *Module) resolve(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	var in struct {
		IDs []string `json:"ids"`
	}
	if err := sdk.Decode(req.Payload, &in); err != nil {
		return sdk.Response{}, err
	}
	out := map[string]string{}
	for _, id := range in.IDs {
		if id == "" || out[id] != "" {
			continue
		}
		res, err := m.db.Query(ctx, "SELECT id FROM partner__bp WHERE legacy_id = ?", id)
		if err != nil {
			return sdk.Response{}, err
		}
		if len(res.Rows) > 0 {
			out[id] = str(res.Rows[0][0])
		}
	}
	return sdk.Response{Payload: map[string]any{"ids": out}}, nil
}
