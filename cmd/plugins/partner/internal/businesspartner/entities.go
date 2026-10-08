package businesspartner

import (
	"context"
	"strings"

	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

const (
	tText   = metamodel.TypeText
	tDate   = metamodel.TypeDate
	tBool   = metamodel.TypeBoolean
	tSelect = metamodel.TypeSelect
)

// Zeitscheiben-Felder für alle zeitabhängigen Entitäten.
var timeSliceFields = []field{
	{Key: "valid_from", Label: "Gültig ab", Type: tDate, Listable: true},
	{Key: "valid_to", Label: "Gültig bis", Type: tDate, Listable: true},
}

func withTimeSlice(fs ...field) []field { return append(fs, timeSliceFields...) }

var (
	refBP          = &ref{Table: "partner__bp", Column: "id", Label: "Geschäftspartner", TimeSliced: true, Object: "BusinessPartner", LabelFields: []string{"name1", "name2"}}
	refAddress     = &ref{Table: "partner__addresses", Column: "id", Label: "Adresse", Object: "PartnerAddressData", LabelFields: []string{"street", "house_no", "zip_code", "city", "country"}}
	refAddressRole = &ref{Table: "partner__address_roles", Column: "code", Label: "Adressrolle", TimeSliced: true, Object: "PartnerAddressRole", LabelFields: []string{"description"}}
	refCommCat     = &ref{Table: "partner__comm_categories", Column: "code", Label: "Kommunikationskategorie", Object: "PartnerCommCategory", LabelFields: []string{"description"}}
	refCommType    = &ref{Table: "partner__comm_types", Column: "code", Label: "Kommunikationstyp", TimeSliced: true, Object: "PartnerCommType", LabelFields: []string{"description"}}
	refRoleType    = &ref{Table: "partner__role_types", Column: "code", Label: "Rolle", TimeSliced: true, Object: "PartnerRoleType", LabelFields: []string{"description"}}
)

// entities in der Reihenfolge der Registrierung (Navigation).
func (m *Module) entities() []*entity {
	return []*entity{
		m.businessPartner(), m.partnerRole(), m.partnerCompanyCode(),
		m.address(), m.partnerAddress(), m.partnerContact(), m.partnerBankDetail(),
		m.addressRoleCatalog(), m.commCategoryCatalog(), m.commTypeCatalog(), m.roleTypeCatalog(),
	}
}

// --- Kataloge ----------------------------------------------------------------------

func (m *Module) addressRoleCatalog() *entity {
	return &entity{
		Object: "PartnerAddressRole", Section: "Kataloge", Title: "Adressrollen", Icon: "icon-tag", Table: "partner__address_roles",
		Keys: []string{"code", "valid_from"}, TimeSlice: true, Order: "code", Search: []string{"code", "description"},
		Fields: withTimeSlice(
			field{Key: "code", Label: "Code", Type: tText, Required: true, Listable: true, Immutable: true},
			field{Key: "description", Label: "Beschreibung", Type: tText, Required: true, Listable: true},
			field{Key: "is_main", Label: "Hauptanschrift", Type: tBool, Listable: true},
		),
		// Nur eine Adressrolle darf die Hauptanschrift kennzeichnen.
		Validate: func(ctx context.Context, rec, _ record) error {
			if !asBool(rec["is_main"]) {
				return nil
			}
			return m.uniqueMain(ctx, "partner__address_roles", "code", str(rec["code"]), "", nil, "Hauptanschrift")
		},
	}
}

func (m *Module) commCategoryCatalog() *entity {
	return &entity{
		Object: "PartnerCommCategory", Section: "Kataloge", Title: "Kommunikationskategorien", Icon: "icon-tag", Table: "partner__comm_categories",
		Keys: []string{"code"}, Order: "code", Search: []string{"code", "description"},
		Fields: []field{
			{Key: "code", Label: "Code (maschinenlesbar: EMAIL, PHONE, FAX, WEB …)", Type: tText, Required: true, Listable: true, Immutable: true},
			{Key: "description", Label: "Beschreibung", Type: tText, Required: true, Listable: true},
		},
	}
}

func (m *Module) commTypeCatalog() *entity {
	return &entity{
		Object: "PartnerCommType", Section: "Kataloge", Title: "Kommunikationstypen", Icon: "icon-tag", Table: "partner__comm_types",
		Keys: []string{"code", "valid_from"}, TimeSlice: true, Order: "category_code, code", Search: []string{"code", "description"}, Filters: []string{"category_code"},
		Fields: withTimeSlice(
			field{Key: "code", Label: "Code", Type: tText, Required: true, Listable: true, Immutable: true},
			field{Key: "category_code", Label: "Kategorie", Type: tText, Required: true, Listable: true, Ref: refCommCat},
			field{Key: "description", Label: "Beschreibung", Type: tText, Required: true, Listable: true},
			field{Key: "is_main", Label: "Haupttyp der Kategorie", Type: tBool, Listable: true},
		),
		// Pro Kategorie höchstens ein Haupt-Kommunikationstyp.
		Validate: func(ctx context.Context, rec, _ record) error {
			if !asBool(rec["is_main"]) {
				return nil
			}
			return m.uniqueMain(ctx, "partner__comm_types", "code", str(rec["code"]), "category_code", rec["category_code"], "Haupttyp der Kategorie "+str(rec["category_code"]))
		},
	}
}

func (m *Module) roleTypeCatalog() *entity {
	return &entity{
		Object: "PartnerRoleType", Section: "Kataloge", Title: "Rollentypen", Icon: "icon-tag", Table: "partner__role_types",
		Keys: []string{"code", "valid_from"}, TimeSlice: true, Order: "code", Search: []string{"code", "description"},
		Fields: withTimeSlice(
			field{Key: "code", Label: "Code", Type: tText, Required: true, Listable: true, Immutable: true},
			field{Key: "description", Label: "Beschreibung", Type: tText, Required: true, Listable: true},
			field{Key: "is_debitor", Label: "Debitor (Finanzrolle)", Type: tBool, Listable: true},
			field{Key: "is_creditor", Label: "Kreditor (Finanzrolle)", Type: tBool, Listable: true},
		),
	}
}

// uniqueMain: Kein anderer Eintrag (gleiche Gruppe) darf is_main tragen.
func (m *Module) uniqueMain(ctx context.Context, table, keyCol, key, groupCol string, group any, what string) error {
	sql := "SELECT " + keyCol + " FROM " + table + " WHERE is_main = ? AND " + keyCol + " <> ?"
	args := []any{true, key}
	if groupCol != "" {
		sql += " AND " + groupCol + " = ?"
		args = append(args, group)
	}
	res, err := m.db.Query(ctx, sql, args...)
	if err != nil {
		return err
	}
	if len(res.Rows) > 0 {
		return invalid("%s ist bereits %q – erst dort entfernen", what, str(res.Rows[0][0]))
	}
	return nil
}

// --- Geschäftspartner --------------------------------------------------------------

func (m *Module) businessPartner() *entity {
	return &entity{
		Object: "BusinessPartner", Title: "Geschäftspartner", Icon: "icon-users", Table: "partner__bp",
		Keys: []string{"id", "valid_from"}, Surrogate: true, TimeSlice: true, Order: "search_term, name1, valid_from",
		Search: []string{"name1", "name2", "search_term"},
		// Filter role: nur Partner, die die Rolle heute haben (Auswahl in anderen
		// Modulen, z. B. Eigentümer eines Mietobjekts – Lookup.Filters {"role": …}).
		Filters: []string{"role"},
		FilterExpr: map[string]func(v any) (string, []any){"role": func(v any) (string, []any) {
			return "EXISTS (SELECT 1 FROM partner__roles r WHERE r.bp_id = partner__bp.id AND r.role_code = ? AND r.valid_from <= ? AND r.valid_to >= ?)",
				[]any{str(v), today(), today()}
		}},
		// Detailansicht im Stil von LeanIX: Stammdaten plus eingebettete
		// Unter-Objects. Adressen sind n:m über die Zuordnung PartnerAddress
		// (Rolle + Zeitscheibe) zu wiederverwendbaren PartnerAddressData.
		TitleField: "name1",
		Sections: []metamodel.SectionDefinition{
			{Key: "stammdaten", Title: "Stammdaten", Fields: []string{"type", "name1", "name2", "search_term", "is_blocked", "id"}},
			{Key: "gueltigkeit", Title: "Gültigkeit", Fields: []string{"valid_from", "valid_to"}},
			{Key: "rollen", Title: "Rollen", Relation: &metamodel.Relation{Object: "PartnerRole", ForeignKey: "bp_id",
				Columns: []string{"role_code", "company_codes", "valid_from", "valid_to"}}},
			{Key: "adressen", Title: "Adressen", Relation: &metamodel.Relation{Object: "PartnerAddress", ForeignKey: "bp_id",
				Columns: []string{"address_role_code", "address_id", "is_default", "valid_from", "valid_to"}}},
			{Key: "kommunikation", Title: "Kommunikation", Relation: &metamodel.Relation{Object: "PartnerContact", ForeignKey: "bp_id",
				Columns: []string{"comm_type_code", "value", "is_default", "valid_from", "valid_to"}}},
			{Key: "bank", Title: "Bankverbindungen", Collapsed: true, Relation: &metamodel.Relation{Object: "PartnerBankDetail", ForeignKey: "bp_id",
				Columns: []string{"iban", "bic", "bank_name", "is_default", "valid_from", "valid_to"}}},
			{Key: "buchungskreise", Title: "Buchungskreisdaten", Collapsed: true, Relation: &metamodel.Relation{Object: "PartnerCompanyCode", ForeignKey: "bp_id",
				Columns: []string{"company_code", "role_code", "reconciliation_account", "payment_terms", "dunning_block", "posting_block"}}},
			// Tags aus dem TagManagement (Plugin tag) – je nach Tag Sets für BusinessPartner
			// und Buchungskreis. Fehlt das Plugin, zeigt der Abschnitt einen Hinweis.
			{Key: "merkmale", Title: "Merkmale", Tags: true},
		},
		// Zeitscheibe seit 0.5.0 (Lebenszyklus timeslice): Ein Partner endet zu einem
		// Enddatum. Ab dann läuft die gesetzliche Aufbewahrungsfrist; danach kann
		// er gelöscht werden (noch nicht umgesetzt).
		Fields: withTimeSlice(
			field{Key: "id", Label: "ID", Type: tText, ReadOnly: true},
			field{Key: "type", Label: "Art", Type: tSelect, Required: true, Listable: true, Options: []metamodel.Option{
				{Value: "ORGANIZATION", Label: "Organisation"}, {Value: "PERSON", Label: "Person"}}},
			field{Key: "name1", Label: "Name 1 (Firma / Nachname)", Type: tText, Required: true, Listable: true},
			field{Key: "name2", Label: "Name 2 (Vorname / Zusatz)", Type: tText, Listable: true},
			field{Key: "search_term", Label: "Suchbegriff", Type: tText, Listable: true},
			field{Key: "is_blocked", Label: "Gesperrt", Type: tBool, Listable: true},
		),
		Validate: func(ctx context.Context, rec, old record) error {
			if rec["search_term"] == nil { // Matchcode aus Name 1
				rec["search_term"] = strings.ToUpper(str(rec["name1"]))
			}
			if old == nil {
				return nil // neu: noch ohne Rollen
			}
			// Jede Finanzrolle braucht Buchungskreisdaten mit Abstimmkonto.
			missing, err := m.missingFinanceData(ctx, str(rec["id"]))
			if err != nil {
				return err
			}
			if len(missing) > 0 {
				return invalid("Finanzrolle %s ohne Buchungskreisdaten (Buchungskreis, Abstimmkonto) – erst unter „Buchungskreisdaten“ ergänzen",
					strings.Join(missing, ", "))
			}
			return nil
		},
	}
}

func (m *Module) address() *entity {
	return &entity{
		Object: "PartnerAddressData", Title: "Adressen", Icon: "icon-map", Table: "partner__addresses",
		Keys: []string{"id"}, Surrogate: true, Order: "country, zip_code, street", Search: []string{"street", "city", "zip_code"},
		Fields: []field{
			{Key: "id", Label: "ID", Type: tText, ReadOnly: true},
			{Key: "street", Label: "Straße", Type: tText, Required: true, Listable: true},
			{Key: "house_no", Label: "Hausnummer", Type: tText, Listable: true},
			{Key: "zip_code", Label: "PLZ", Type: tText, Required: true, Listable: true},
			{Key: "city", Label: "Ort", Type: tText, Required: true, Listable: true},
			{Key: "country", Label: "Land (ISO-2)", Type: tText, Required: true, Listable: true},
		},
		Validate: func(_ context.Context, rec, _ record) error {
			c, err := normalizeCountry(str(rec["country"]))
			rec["country"] = c
			return err
		},
	}
}

func (m *Module) partnerAddress() *entity {
	return &entity{
		Object: "PartnerAddress", Title: "Partner-Adressen", Icon: "icon-map", Table: "partner__bp_addresses",
		Keys: []string{"id", "valid_from"}, Surrogate: true, TimeSlice: true, Order: "bp_id, address_role_code, valid_from",
		Filters: []string{"bp_id", "address_id", "address_role_code"},
		Fields: withTimeSlice(
			field{Key: "id", Label: "ID", Type: tText, ReadOnly: true},
			field{Key: "bp_id", Label: "Geschäftspartner (ID)", Type: tText, Required: true, Listable: true, Immutable: true, Ref: refBP},
			field{Key: "address_id", Label: "Adresse (ID)", Type: tText, Required: true, Listable: true, Ref: refAddress},
			field{Key: "address_role_code", Label: "Adressrolle", Type: tText, Required: true, Listable: true, Ref: refAddressRole},
			field{Key: "is_default", Label: "Standard", Type: tBool, Listable: true},
		),
	}
}

func (m *Module) partnerContact() *entity {
	return &entity{
		Object: "PartnerContact", Title: "Kommunikation", Icon: "icon-phone", Table: "partner__contacts",
		Keys: []string{"id", "valid_from"}, Surrogate: true, TimeSlice: true, Order: "bp_id, comm_type_code, valid_from",
		Filters: []string{"bp_id", "comm_type_code"},
		Fields: withTimeSlice(
			field{Key: "id", Label: "ID", Type: tText, ReadOnly: true},
			field{Key: "bp_id", Label: "Geschäftspartner (ID)", Type: tText, Required: true, Listable: true, Immutable: true, Ref: refBP},
			field{Key: "comm_type_code", Label: "Kommunikationstyp", Type: tText, Required: true, Listable: true, Ref: refCommType},
			field{Key: "value", Label: "Wert", Type: tText, Required: true, Listable: true},
			field{Key: "is_default", Label: "Standard", Type: tBool, Listable: true},
		),
		// Prüfung des Werts anhand der Kategorie des Kommunikationstyps.
		Validate: func(ctx context.Context, rec, _ record) error {
			res, err := m.db.Query(ctx, "SELECT category_code FROM partner__comm_types WHERE code = ?", rec["comm_type_code"])
			if err != nil {
				return err
			}
			if len(res.Rows) == 0 {
				return invalid("Kommunikationstyp %q gibt es nicht", str(rec["comm_type_code"]))
			}
			return validateContact(str(res.Rows[0][0]), str(rec["value"]))
		},
	}
}

func (m *Module) partnerBankDetail() *entity {
	return &entity{
		Object: "PartnerBankDetail", Title: "Bankverbindungen", Icon: "icon-bank", Table: "partner__bank_details",
		Keys: []string{"id", "valid_from"}, Surrogate: true, TimeSlice: true, Order: "bp_id, valid_from", Filters: []string{"bp_id"},
		Fields: withTimeSlice(
			field{Key: "id", Label: "ID", Type: tText, ReadOnly: true},
			field{Key: "bp_id", Label: "Geschäftspartner (ID)", Type: tText, Required: true, Listable: true, Immutable: true, Ref: refBP},
			field{Key: "iban", Label: "IBAN", Type: tText, Required: true, Listable: true},
			field{Key: "bic", Label: "BIC", Type: tText, Listable: true},
			field{Key: "bank_name", Label: "Bank", Type: tText, Listable: true},
			field{Key: "account_holder", Label: "Kontoinhaber", Type: tText},
			field{Key: "is_default", Label: "Standard", Type: tBool, Listable: true},
		),
		Validate: func(_ context.Context, rec, _ record) error {
			iban, err := normalizeIBAN(str(rec["iban"]))
			if err != nil {
				return err
			}
			rec["iban"] = iban
			if rec["bic"] != nil {
				bic, err := normalizeBIC(str(rec["bic"]))
				if err != nil {
					return err
				}
				rec["bic"] = bic
			}
			return nil
		},
	}
}
