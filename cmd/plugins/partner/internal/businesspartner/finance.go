package businesspartner

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// Finanzrollen und Buchungskreise.
//
//   - Eine Rolle ist Finanzrolle, wenn ihr Rollentyp is_debitor oder
//     is_creditor trägt (z. B. DEBITOR, CREDITOR).
//   - Wer einem Partner eine Finanzrolle zuweist, muss im selben Aufruf
//     mindestens einen Buchungskreis-Eintrag mitgeben (Buchungskreis-Zwang).
//   - Buchungskreis-Einträge (partner__company_codes) gibt es nur für
//     Finanzrollen, die der Partner hat; der letzte Eintrag einer aktiven
//     Finanzrolle lässt sich nicht löschen.
//   - Buchungskreise müssen in iam existieren. Lesen und Schreiben ist auf
//     die Buchungskreise beschränkt, für die der Benutzer Rechte hat
//     (PartnerCompanyCode.<action>@<Buchungskreis>).

const ccObject = "PartnerCompanyCode"

// requireCompanyCode prüft Existenz (iam) und Zugriff des Benutzers.
func requireCompanyCode(ctx context.Context, action, cc string) error {
	ok, err := sdk.CheckAccess(ctx, ccObject, action, cc)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: keine Berechtigung für %s.%s im Buchungskreis %s", sdk.ErrPermissionDenied, ccObject, action, cc)
	}
	return nil
}

func (m *Module) companyCodeExists(ctx context.Context, cc string) error {
	_, err := m.services.Call(ctx, "CompanyCode", "get", map[string]any{"id": cc})
	if errors.Is(err, sdk.ErrNotFound) {
		return invalid("Buchungskreis %q gibt es nicht", cc)
	}
	return err
}

// financeRole meldet, ob der Rollentyp eine Finanzrolle ist.
func (m *Module) financeRole(ctx context.Context, code string) (bool, error) {
	res, err := m.db.Query(ctx, "SELECT is_debitor, is_creditor FROM partner__role_types WHERE code = ?", code)
	if err != nil {
		return false, err
	}
	if len(res.Rows) == 0 {
		return false, invalid("Rolle %q gibt es nicht", code)
	}
	return asBool(res.Rows[0][0]) || asBool(res.Rows[0][1]), nil
}

// --- Rollenzuordnung ----------------------------------------------------------------

func (m *Module) partnerRole() *entity {
	return &entity{
		Object: "PartnerRole", Title: "Partner-Rollen", Icon: "icon-id", Table: "partner__roles",
		Keys: []string{"bp_id", "role_code", "valid_from"}, TimeSlice: true, Order: "bp_id, role_code, valid_from",
		Filters: []string{"bp_id", "role_code"},
		Fields: withTimeSlice(
			field{Key: "bp_id", Label: "Geschäftspartner (ID)", Type: tText, Required: true, Listable: true, Immutable: true, Ref: refBP},
			field{Key: "role_code", Label: "Rolle", Type: tText, Required: true, Listable: true, Immutable: true, Ref: refRoleType},
			// Nur beim Anlegen: Buchungskreise der Finanzrolle, eine Zeile je
			// Buchungskreis "BUKRS[;Abstimmkonto[;Zahlungsbedingung]]" – oder
			// in der API eine Liste von Objekten.
			field{Key: "company_codes", Label: "Buchungskreise (bei Finanzrollen Pflicht, nur beim Anlegen: 1000;140000;NT30 je Zeile)",
				Type: metamodel.TypeTextarea, Listable: true, Virtual: true},
		),
		AfterCreate: func(ctx context.Context, rec record) error {
			fin, err := m.financeRole(ctx, str(rec["role_code"]))
			if err != nil {
				return err
			}
			entries, err := parseCompanyCodes(rec["company_codes"])
			if err != nil {
				return err
			}
			if fin && len(entries) == 0 {
				return invalid("Rolle %s ist eine Finanzrolle: mindestens ein Buchungskreis ist Pflicht", str(rec["role_code"]))
			}
			if !fin && len(entries) > 0 {
				return invalid("Rolle %s ist keine Finanzrolle: Buchungskreise sind nicht erlaubt", str(rec["role_code"]))
			}
			for _, cc := range entries {
				cc["bp_id"], cc["role_code"] = rec["bp_id"], rec["role_code"]
				if err := m.insertCompanyCode(ctx, cc, true); err != nil {
					return err
				}
			}
			return nil
		},
		Decorate: func(ctx context.Context, rec record) error {
			res, err := m.db.Query(ctx, "SELECT company_code FROM partner__company_codes WHERE bp_id = ? AND role_code = ? ORDER BY company_code",
				rec["bp_id"], rec["role_code"])
			if err != nil {
				return err
			}
			var ccs []string
			for _, r := range res.Rows {
				ccs = append(ccs, str(r[0]))
			}
			rec["company_codes"] = strings.Join(ccs, "\n")
			return nil
		},
	}
}

// parseCompanyCodes liest die Buchungskreise einer Rollenzuordnung: Text
// (eine Zeile je Buchungskreis, Felder mit ";") oder eine Liste von Objekten.
func parseCompanyCodes(v any) ([]record, error) {
	var out []record
	switch v := v.(type) {
	case nil:
	case string:
		for _, line := range strings.FieldsFunc(v, func(r rune) bool { return r == '\n' || r == '\r' }) {
			parts := strings.Split(strings.TrimSpace(line), ";")
			if parts[0] == "" {
				continue
			}
			cc := record{"company_code": strings.TrimSpace(parts[0])}
			if len(parts) > 1 {
				cc["reconciliation_account"] = strings.TrimSpace(parts[1])
			}
			if len(parts) > 2 {
				cc["payment_terms"] = strings.TrimSpace(parts[2])
			}
			out = append(out, cc)
		}
	case []any:
		for _, it := range v {
			m, ok := it.(map[string]any)
			if !ok {
				return nil, invalid("company_codes: Liste von Objekten erwartet")
			}
			out = append(out, m)
		}
	default:
		return nil, invalid("company_codes: Text oder Liste erwartet")
	}
	return out, nil
}

// insertCompanyCode legt einen Buchungskreis-Eintrag an (mit allen Prüfungen
// der Entität PartnerCompanyCode).
func (m *Module) insertCompanyCode(ctx context.Context, data record, skipRoleCheck bool) error {
	e := m.set.Entity(ccObject)
	rec, err := e.Input(map[string]any{"data": data})
	if err != nil {
		return err
	}
	if skipRoleCheck {
		rec["_role_assigned"] = true // Rolle entsteht im selben Aufruf; Insert schreibt nur Spalten
	}
	if err := e.Insert(ctx, rec); err != nil {
		if errors.Is(err, sdk.ErrAlreadyExists) {
			return fmt.Errorf("%w: Buchungskreis %s für %s existiert bereits", sdk.ErrAlreadyExists, str(rec["company_code"]), str(rec["role_code"]))
		}
		return err
	}
	return nil
}

// --- Buchungskreis-Ausprägung (Debitor / Kreditor) ---------------------------------------

func (m *Module) partnerCompanyCode() *entity {
	e := &entity{
		Object: ccObject, Title: "Buchungskreisdaten", Icon: "icon-building", Table: "partner__company_codes",
		Keys: []string{"bp_id", "company_code", "role_code"}, Order: "bp_id, company_code, role_code",
		Filters: []string{"bp_id", "company_code", "role_code"},
		Fields: []field{
			{Key: "bp_id", Label: "Geschäftspartner (ID)", Type: tText, Required: true, Listable: true, Immutable: true, Ref: refBP},
			{Key: "company_code", Label: "Buchungskreis", Type: tText, Required: true, Listable: true, Immutable: true,
				Lookup: &metamodel.Lookup{Object: "CompanyCode", ValueField: "code", LabelFields: []string{"description"}}},
			{Key: "role_code", Label: "Finanzrolle", Type: tText, Required: true, Listable: true, Immutable: true, Ref: refRoleType},
			{Key: "reconciliation_account", Label: "Abstimmkonto", Type: tText, Listable: true},
			{Key: "payment_terms", Label: "Zahlungsbedingung", Type: tText, Listable: true},
			{Key: "dunning_block", Label: "Mahnsperre", Type: tBool, Listable: true},
			{Key: "posting_block", Label: "Buchungssperre", Type: tBool, Listable: true},
		},
		Validate: func(ctx context.Context, rec, old record) error {
			if old != nil {
				return nil // Schlüssel unverändert, Rolle bereits geprüft
			}
			if err := m.companyCodeExists(ctx, str(rec["company_code"])); err != nil {
				return err
			}
			fin, err := m.financeRole(ctx, str(rec["role_code"]))
			if err != nil {
				return err
			}
			if !fin {
				return invalid("Rolle %s ist keine Finanzrolle (is_debitor/is_creditor)", str(rec["role_code"]))
			}
			if rec["_role_assigned"] == true {
				return nil // im selben Aufruf zugewiesen (PartnerRole.create)
			}
			res, err := m.db.Query(ctx, "SELECT COUNT(*) FROM partner__roles WHERE bp_id = ? AND role_code = ?", rec["bp_id"], rec["role_code"])
			if err != nil {
				return err
			}
			if n, _ := strconv.Atoi(str(res.Rows[0][0])); n == 0 {
				return invalid("Partner hat die Rolle %s nicht – erst die Rolle zuweisen", str(rec["role_code"]))
			}
			return nil
		},
		CheckRecord: func(ctx context.Context, action string, rec record) error {
			return requireCompanyCode(ctx, action, str(rec["company_code"]))
		},
		// Liste nur mit den Buchungskreisen, die der Benutzer sehen darf.
		ListScope: func(ctx context.Context) (string, []any, bool, error) {
			g, err := sdk.GrantedCompanyCodes(ctx, ccObject, "list")
			if err != nil || g.All {
				return "", nil, false, err
			}
			if g.None() {
				return "", nil, true, nil
			}
			marks := make([]string, len(g.CompanyCodes))
			args := make([]any, len(g.CompanyCodes))
			for i, cc := range g.CompanyCodes {
				marks[i], args[i] = "?", cc
			}
			return "company_code IN (" + strings.Join(marks, ", ") + ")", args, false, nil
		},
	}
	return e
}
